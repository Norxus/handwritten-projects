package internal

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/cloudwego/eino/internal/generic"
)

var (
	concatFuncs = map[reflect.Type]any{
		generic.TypeOf[string]():        concatStrings,
		generic.TypeOf[int8]():          useLast[int8],
		generic.TypeOf[int16]():         useLast[int16],
		generic.TypeOf[int32]():         useLast[int32],
		generic.TypeOf[int64]():         useLast[int64],
		generic.TypeOf[int]():           useLast[int],
		generic.TypeOf[uint8]():         useLast[uint8],
		generic.TypeOf[uint16]():        useLast[uint16],
		generic.TypeOf[uint32]():        useLast[uint32],
		generic.TypeOf[uint64]():        useLast[uint64],
		generic.TypeOf[uint]():          useLast[uint],
		generic.TypeOf[bool]():          useLast[bool],
		generic.TypeOf[float32]():       useLast[float32],
		generic.TypeOf[float64]():       useLast[float64],
		generic.TypeOf[time.Time]():     useLast[time.Time],
		generic.TypeOf[time.Duration](): useLast[time.Duration],
	}
)

func useLast[T any](s []T) (T, error) {
	return s[len(s)-1], nil
}

// 使用 stringBuilder 来连接
func concatStrings(ss []string) (string, error) {
	var n int
	for _, s := range ss {
		n += len(s)
	}

	var b strings.Builder
	b.Grow(n)
	for _, s := range ss {
		_, err := b.WriteString(s)
		if err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

func RegisterStreamChunkConcatFunc[T any](fn func([]T) (T, error)) {
	concatFuncs[generic.TypeOf[T]()] = fn
}

// 根据类型获取对应的拼接函数，把这个“强类型函数”统一包装成一个反射风格的函数签名
func GetConcatFunc(typ reflect.Type) func(reflect.Value) (reflect.Value, error) {
	if fn, ok := concatFuncs[typ]; ok {
		return func(a reflect.Value) (reflect.Value, error) {
			rvs := reflect.ValueOf(fn).Call([]reflect.Value{a})
			var err error
			// 两个返回值，一个结果，一个 error
			if !rvs[1].IsNil() {
				err = rvs[1].Interface().(error)
			}
			return rvs[0], err
		}
	}
	return nil
}

// 根据类型判断，进行 map 或 slice 和合并
func ConcatItems[T any](items []T) (T, error) {
	typ := generic.TypeOf[T]()
	v := reflect.ValueOf(items)

	var cv reflect.Value
	var err error

	if typ.Kind() == reflect.Map {
		cv, err = concatMaps(v)
	} else {
		cv, err = concatSliceValue(v)
	}

	if err != nil {
		var t T
		return t, err
	}

	return cv.Interface().(T), nil
}

// 用于处理各种不同的 map 类型
func concatMaps(ms reflect.Value) (reflect.Value, error) {
	// 取容器元素类型
	typ := ms.Type().Elem()

	// 创建合并的中间 map，结构是 map[K][]any，把同一个 key 的多个候选值先归并到一起
	rms := reflect.MakeMap(reflect.MapOf(typ.Key(), generic.TypeOf[[]any]()))
	// 最终处理后的 map，把这些候选值处理成最终合法的 map value
	ret := reflect.MakeMap(typ)

	// 确定待合并的 map 的数量
	n := ms.Len()
	for i := 0; i < n; i++ {
		m := ms.Index(i)

		for _, key := range m.MapKeys() {
			vals := rms.MapIndex(key)
			// MapIndex 在 key 不存在时，不会返回 nil ，而是返回一个 无效的 reflect.Value
			if !vals.IsValid() {
				var s []any
				vals = reflect.ValueOf(s)
			}

			val := m.MapIndex(key)
			// 把值放入合并的 map 中
			vals = reflect.Append(vals, val)
			rms.SetMapIndex(key, vals)
		}
	}

	// 遍历 rms ，把每个 key 的收集结果真正合成为最终的 ret[key]
	for _, key := range rms.MapKeys() {
		vals := rms.MapIndex(key)

		anyVals := vals.Interface().([]any)
		if len(anyVals) == 1 {
			ele := anyVals[0]
			if ele == nil {
				ret.SetMapIndex(key, reflect.Zero(typ.Elem()))
				continue
			}

			ret.SetMapIndex(key, reflect.ValueOf(ele))
			continue
		}

		// 如果长度 > 1，把 []any 转成一个真正的切片反射值
		v, err := toSliceValue(anyVals)
		if err != nil {
			return reflect.Value{}, err
		}

		var cv reflect.Value

		// 长度大于 1 还需要区分内部元素是 map 还是 slice
		if v.Type().Elem().Kind() == reflect.Map {
			// 把 []map 合并
			cv, err = concatMaps(v)
		} else {
			// 把二维 slice [][] 合并
			cv, err = concatSliceValue(v)
		}

		if err != nil {
			return reflect.Value{}, err
		}

		ret.SetMapIndex(key, cv)
	}

	return ret, nil
}

// 把 slice 合并成一个最终的值，如果 slice 里有多个非零值会报错
func concatSliceValue(val reflect.Value) (reflect.Value, error) {
	elmType := val.Type().Elem()

	// 如果只有一个元素，直接返回这个元素
	if val.Len() == 1 {
		return val.Index(0), nil
	}

	f := GetConcatFunc(elmType)
	if f != nil {
		return f(val)
	}

	var filtered reflect.Value
	for i := 0; i < val.Len(); i++ {
		oneVal := val.Index(i)
		// 找出数组中的非零值
		if !oneVal.IsZero() {
			// 如果之前已经见过一个非零值了，这时无法决定该保留谁，所以直接报错
			if filtered.IsValid() {
				return reflect.Value{}, fmt.Errorf("cannot concat multiple non-zero value of type %s", elmType)
			}

			// 记录这个非零值
			filtered = oneVal
		}
	}

	// 循环结束，还是非零值，那就构造一个该类型的零值
	if !filtered.IsValid() {
		filtered = reflect.New(elmType).Elem()
	}

	return filtered, nil
}

// 把 []any 换成类型明确的 []T
func toSliceValue(vs []any) (reflect.Value, error) {
	typ := reflect.TypeOf(vs[0])

	ret := reflect.MakeSlice(reflect.SliceOf(typ), len(vs), len(vs))
	ret.Index(0).Set(reflect.ValueOf(vs[0]))

	for i := 1; i < len(vs); i++ {
		v := vs[i]
		vt := reflect.TypeOf(v)
		// 如果当前类型不等于期望类型，报错
		if typ != vt {
			return reflect.Value{}, fmt.Errorf("unexpected slice element type. Got %v, expected %v", typ, vt)
		}

		ret.Index(i).Set(reflect.ValueOf(v))
	}

	return ret, nil
}
