package compose

import (
	"reflect"
	"strings"

	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

type FieldMapping struct {
	fromNodeKey string
	from        string
	to          string

	customExtractor func(input any) (any, error)
}

const pathSeparator = "\x1F"

var strType = reflect.TypeOf("")

// 构造一个“转换函数”出来，这个转换函数负责把 map[string]any 转成目标类型 I
func buildFieldMappingConverter[I any]() func(input any) (any, error) {
	return func(input any) (any, error) {
		in, ok := input.(map[string]any)
		if !ok {
			panic(newUnexpectedInputTypeErr(reflect.TypeOf(map[string]any{}), reflect.TypeOf(input)))
		}
		return convertTo(in, generic.TypeOf[I]()), nil
	}
}

// 给“流式输入”场景准备一个转换器，把流里每个 map[string]any chunk 转成目标类型 I 的 chunk
func buildStreamFieldMappingConverter[I any]() func(input streamReader) streamReader {
	return func(input streamReader) streamReader {
		// 把输入的 streamReader 解包为 "元素类型为 map[string]any" 的流
		s, ok := unpackStreamReader[map[string]any](input)
		if !ok {
			panic("mappingStreamAssign incoming streamReader chunk type not map[string]any")
		}

		return packStreamReader(schema.StreamReaderWithConvert(s, func(v map[string]any) (I, error) {
			t := convertTo(v, generic.TypeOf[I]())
			return t.(I), nil
		}))
	}
}

// 把map 组装成 typ 类型的实例
func convertTo(mappings map[string]any, typ reflect.Type) any {
	// 创建一个空 map
	tValue := newInstanceByType(typ)
	if !tValue.CanAddr() {
		tValue = newInstanceByType(reflect.PointerTo(typ)).Elem()
	}

	// 把每条路径（mapping) 对应的值写入 tValue 中
	for mapping, taken := range mappings {
		tValue = assignOne(tValue, taken, mapping)
	}

	return tValue.Interface()
}

func newInstanceByType(typ reflect.Type) reflect.Value {
	switch typ.Kind() {
	case reflect.Map:
		return reflect.MakeMap(typ)
	case reflect.Slice, reflect.Array:
		// 下面这种写法思路类似于，其实也可以直接 return reflect.MakeSlice...
		// var s []int
		// s = make([]int, 0, 0)
		slice := reflect.New(typ).Elem()
		slice.Set(reflect.MakeSlice(typ, 0, 0))
		return slice
	case reflect.Ptr:
		typ = typ.Elem()
		origin := reflect.New(typ)
		// 指针需要递归解析
		nested := newInstanceByType(typ)
		// 指向解析完的零值
		origin.Elem().Set(nested)

		return origin
	default:
		return reflect.New(typ).Elem()
	}
}

// 把一个取出来的值 taken ，按照目标路径 to (可能有段，类似于 jsonPath)，写进目标对象 destValue 里
func assignOne(destValue reflect.Value, taken any, to string) reflect.Value {
	// 没有复杂路径，直接设置
	if len(to) == 0 {
		destValue.Set(reflect.ValueOf(taken))
		return destValue
	}

	var (
		// 拆开路径，一层一层看
		toPaths           = splitFieldPath(to)
		originalDestValue = destValue
		parentMap         reflect.Value
		parentKey         string
	)

	for {
		// 拿出本次要处理的 path
		path := toPaths[0]
		// 待处理的 path
		toPaths = toPaths[1:]
		// 到达最终的路径处，也就是循环的边界条件
		if len(toPaths) == 0 {
			// 转换成反射类型
			// 如果 taken 是 nil ， reflect.ValueOf(nil) 会得到一个 invalid reflect.Value
			toSet := reflect.ValueOf(taken)

			// 判断 destValue 的类型是不是 any
			if destValue.Type() == reflect.TypeOf((*any)(nil)).Elem() {
				// 先断言类型是 map
				existingMap, ok := destValue.Interface().(map[string]any)
				if ok {
					// 把existingMap 包装成反射类型，方便进行后面的操作
					destValue = reflect.ValueOf(existingMap)
				} else {
					// 如果具体不是 map[string]any 类型，由于 destCalue 本身就是 any 类型，那么直接初始化一个 map[string]any 放入进去
					mapValue := reflect.MakeMap(reflect.TypeOf(map[string]any{}))
					destValue.Set(mapValue)
					destValue = mapValue
				}
			}

			// 如果目标类型是 map
			if destValue.Kind() == reflect.Map {
				key := reflect.ValueOf(path)
				keyType := destValue.Type().Key()
				// key 的类型不是目标类型，就用反射转换
				if keyType != strType {
					key = key.Convert(keyType)
				}

				// 如果 toSet 是无效值，那么就 failback 到 0 值
				if !toSet.IsValid() {
					toSet = reflect.Zero(destValue.Type().Elem())
				}

				// 把值写进去
				destValue.SetMapIndex(key, toSet)

				// 如果上层 map 存在，那么就需要把 destValue 写回到上层 map
				if parentMap.IsValid() {
					parentMap.SetMapIndex(reflect.ValueOf(parentKey), destValue)
				}

				return originalDestValue
			}

			// 如果 destValue 是指针类型，那么需要循环解引用
			ptrValue := destValue
			for destValue.Kind() == reflect.Ptr {
				destValue = destValue.Elem()
			}

			// 如果 toSet 无效，通常表示原始 taken 是 nil，直接跳过不写
			if !toSet.IsValid() {

			} else {
				// 按照 path 写入 destValue 中
				field := destValue.FieldByName(path)
				field.Set(toSet)
			}

			// 因为值已经写完了，有父 map，那么就重新写回去
			if parentMap.IsValid() {
				parentMap.SetMapIndex(reflect.ValueOf(parentKey), ptrValue)
			}

			return originalDestValue
		}

		// 如果目的值是 any，想办法把它变成 map[string]any 类型
		if destValue.Type() == reflect.TypeOf((*any)(nil)).Elem() {
			existingMap, ok := destValue.Interface().(map[string]any)
			if ok {
				destValue = reflect.ValueOf(existingMap)

			} else {
				mapValue := reflect.MakeMap(reflect.TypeOf(map[string]any{}))
				destValue.Set(mapValue)
				destValue = mapValue
			}
		}

		// 如果是 map 类型，那就继续走下一层
		if destValue.Kind() == reflect.Map {
			keyValue := reflect.ValueOf(path)
			valueValue := destValue.MapIndex(keyValue)
			// 如果 key 对应的值不存在，那么就创建一个零值放进去
			if !valueValue.IsValid() {
				valueValue = newInstanceByType(destValue.Type().Elem())
				destValue.SetMapIndex(keyValue, valueValue)
			}

			// 避免循环过程中的修改消失，所以重写一下
			if parentMap.IsValid() {
				parentMap.SetMapIndex(reflect.ValueOf(parentKey), destValue)
			}

			// 当前 map 是下一轮循环的父 map
			parentMap = destValue
			parentKey = path
			destValue = valueValue

			continue
		}

		// 如果不是 map，那么就按照 struct 处理
		ptrValue := destValue
		// 如果有指针，先去掉指针
		for destValue.Kind() == reflect.Ptr {
			destValue = destValue.Elem()
		}

		// 获取 struct 中的字段
		field := destValue.FieldByName(path)
		instantiateIfNeeded(field)

		// 把 map 重新写回去
		if parentMap.IsValid() {
			parentMap.SetMapIndex(reflect.ValueOf(parentKey), ptrValue)
			// 清理引用信息，避免下一层错误使用
			// 不需要像 map 那样还需要记住这些，直接改 field 即可
			parentMap = reflect.Value{}
			parentKey = ""
		}

		// 进入下一层
		destValue = field
	}
}

type FieldPath []string

func (fp *FieldPath) join() string {
	return strings.Join(*fp, pathSeparator)
}

// 按照\x1F 切开路径
func splitFieldPath(path string) FieldPath {
	p := strings.Split(path, pathSeparator)
	// 说明原本就是空字符串
	if len(p) == 1 && p[0] == "" {
		return FieldPath{}
	}

	return p
}

// 如果目标字段是“需要先初始化才能继续写入”的类型，就把它初始化出来，这里也就是指针和 map
func instantiateIfNeeded(field reflect.Value) {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
	} else if field.Kind() == reflect.Map {
		if field.IsNil() {
			field.Set(reflect.MakeMap(field.Type()))
		}
	}
}
