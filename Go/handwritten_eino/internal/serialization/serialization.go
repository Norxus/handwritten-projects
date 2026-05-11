package serialization

import (
	"encoding/json"
	"fmt"
	"github.com/bytedance/sonic"
	"reflect"
)

// 反射类型 与 自定义类型的双向映射关系
var (
	m  = map[string]reflect.Type{}
	rm = map[reflect.Type]string{}
)

func init() {
	_ = GenericRegister[int]("_eino_int")
	_ = GenericRegister[int8]("_eino_int8")
	_ = GenericRegister[int16]("_eino_int16")
	_ = GenericRegister[int32]("_eino_int32")
	_ = GenericRegister[int64]("_eino_int64")
	_ = GenericRegister[uint]("_eino_uint")
	_ = GenericRegister[uint8]("_eino_uint8")
	_ = GenericRegister[uint16]("_eino_uint16")
	_ = GenericRegister[uint32]("_eino_uint32")
	_ = GenericRegister[uint64]("_eino_uint64")
	_ = GenericRegister[float32]("_eino_float32")
	_ = GenericRegister[float64]("_eino_float64")
	_ = GenericRegister[complex64]("_eino_complex64")
	_ = GenericRegister[complex128]("_eino_complex128")
	_ = GenericRegister[uintptr]("_eino_uintptr")
	_ = GenericRegister[bool]("_eino_bool")
	_ = GenericRegister[string]("_eino_string")
	_ = GenericRegister[any]("_eino_any")
}

// 记录自定义类型 与 golang 类型的双向映射关系
// 同时省略了指针的作用，只记录了指针类型最底层的元素类型
func GenericRegister[T any](key string) error {
	t := reflect.TypeOf((*T)(nil)).Elem()
	// 这种 for 循环是处理指针类型的常用方式
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if nt, ok := m[key]; ok {
		return fmt.Errorf("key[%s] already registered to %s", key, nt.String())
	}

	if nk, ok := rm[t]; ok {
		return fmt.Errorf("type[%s] already registered to %s", t.String(), nk)
	}

	m[key] = t
	rm[t] = key

	return nil
}

type internalSerializer struct{}

func (i *internalSerializer) Marshal(v any) ([]byte, error) {
	is, err := internalMarshal(v, nil)
	if err != nil {
		return nil, err
	}
	return sonic.Marshal(is)
}

type internalStruct struct {
	Type        *valueType                 `json:",omitempty"`
	JSONValue   json.RawMessage            `json:",omitempty"`
	MapValues   map[string]*internalStruct `json:",omitempty"`
	SliceValues []*internalStruct          `json:",omitempty"`
}

type valueType struct {
	PointerNum     uint32     `json:",omitempty"`
	SimpleType     string     `json:",omitempty"`
	StructType     string     `json:",omitempty"`
	MapKeyType     *valueType `json:",omitempty"`
	MapValueType   *valueType `json:",omitempty"`
	SliceValueType *valueType `json:",omitempty"`
}

/*
把任意 Go 值 v，转成一个统一的“内部中间表示” internalStruct
*/
func internalMarshal(v any, fieldType reflect.Type) (*internalStruct, error) {
	if v == nil ||
		// 值为空，如果已经知道值的具体类型，就直接返回 nil
		// 如果 internalMarshal(0, nil)，因为这是顶层值，如果省掉了，就完全不知道它原本是 int(0) 、 false 、 "" 还是别的
		// 如果 internalMarshal(0, interface{}), 也不会返回 nil，因为不知道它的实际具体类型，反序列化时，代码只能知道
		// 这个字段类型是 interface{} 对应内容是空，恢复出来就是 nil，而不是 0
		(reflect.ValueOf(v).IsZero() && fieldType != nil && fieldType.Kind() != reflect.Interface) {
		return nil, nil
	}

	ret := &internalStruct{}
	// 值
	rv := reflect.ValueOf(v)
	// 值类型
	rt := rv.Type()
	typeUnspecific := fieldType == nil || fieldType.Kind() == reflect.Interface

	var pointerNum uint32
	for rt.Kind() == reflect.Ptr {
		pointerNum++
		if !rv.IsNil() {
			rv = rv.Elem()
			rt = rt.Elem()
			continue
		}

		// 获取 rt 真正的底层类型
		for rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}

		if typeUnspecific {
			key, ok := rm[rt]
			if !ok {
				return nil, fmt.Errorf("unknown type %v", rt)
			}
			ret.Type = &valueType{
				PointerNum: pointerNum,
				SimpleType: key,
			}
		}

		// t.Kind() == reflect.Ptr && rv.IsNil() == true 时会走到这里
		// eg. var p *int = nil
		ret.JSONValue = json.RawMessage("null")
		return ret, nil
	}

	switch rt.Kind() {
	case reflect.Struct:
		// ret.Type ：只有当外部拿不到准确静态类型时，才需要额外带上类型标签
		if typeUnspecific {
			key, ok := rm[rt]
			if !ok {
				return nil, fmt.Errorf("unknown type: %v", rt)
			}

			// 如果一个类型实现了 json.Marshaler 和 json.Unmarshaler 接口
			// 就把它当成一种“简单值类型”处理，因为这种类型定义了自己的序列化/反序列化逻辑
			// 不能像普通结构体那样递归处理，把它当作一个基本原子类型处理即可
			if checkMarshaler(rt) {
				ret.Type = &valueType{
					PointerNum: pointerNum,
					SimpleType: key,
				}
			} else {
				ret.Type = &valueType{
					PointerNum: pointerNum,
					StructType: key,
				}
			}
		}

		// 如果一个类型实现了 json.Marshaler 和 json.Unmarshaler 接口
		// 直接获取它的值
		if checkMarshaler(rt) {
			jsonBytes, err := json.Marshal(rv.Interface())
			if err != nil {
				return nil, err
			}
			ret.JSONValue = jsonBytes
			return ret, nil
		}

		// key 是字段名，比如 "Name" 、 "Age"
		// value 是该字段对应的内部序列化结构 *internalStruct
		ret.MapValues = make(map[string]*internalStruct)
		for i := 0; i < rt.NumField(); i++ {
			// 取第 i 个字段元信息
			field := rt.Field(i)
			// 只处理导出的字段，非空表示未导出字段
			if field.PkgPath == "" {
				// 字段名
				k := field.Name
				// 字段值
				v := rv.Field(i)

				// 对字段值继续递归做序列化
				internalValue, err := internalMarshal(v.Interface(), field.Type)
				if err != nil {
					return nil, err
				}
				// 把字段结果存到 ret.MapValues
				ret.MapValues[k] = internalValue
			}
		}

		return ret, nil
	case reflect.Map:
		if typeUnspecific {
			var err error
			ret.Type = &valueType{
				PointerNum: pointerNum,
			}
			ret.Type.MapValueType, err = extractType(rt.Key())
			if err != nil {
				return nil, err
			}

			ret.Type.MapValueType, err = extractType(rt.Elem())
			if err != nil {
				return nil, err
			}
		}

		ret.MapValues = make(map[string]*internalStruct)
		// 创建一个 map 迭代器
		iter := rv.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()

			// 对 map 的值进行递归做序列化
			internalValue, err := internalMarshal(v.Interface(), rt.Elem())
			if err != nil {
				return nil, err
			}

			// 这里 key 直接变成 string
			keyStr, err := sonic.MarshalString(k.Interface())
			if err != nil {
				return nil, fmt.Errorf("marshaling map key[%v] fail: %v", k.Interface(), err)
			}

			ret.MapValues[keyStr] = internalValue
		}

		return ret, nil

	case reflect.Slice, reflect.Array:
		if typeUnspecific {
			var err error
			ret.Type = &valueType{
				PointerNum: pointerNum,
			}
			ret.Type.SliceValueType, err = extractType(rt.Elem())
			if err != nil {
				return nil, err
			}
		}

		length := rv.Len()
		ret.SliceValues = make([]*internalStruct, length)

		for i := 0; i < length; i++ {
			internalValue, err := internalMarshal(rv.Index(i).Interface(), rt.Elem())
			if err != nil {
				return nil, err
			}
			ret.SliceValues[i] = internalValue
		}

		return ret, nil
	default:
		if typeUnspecific {
			key, ok := rm[rv.Type()]
			if !ok {
				return nil, fmt.Errorf("unknown type: %v", rv.Type())
			}
			ret.Type = &valueType{
				PointerNum: pointerNum,
				SimpleType: key,
			}
		}

		jsonBytes, err := sonic.Marshal(rv.Interface())
		if err != nil {
			return nil, err
		}
		ret.JSONValue = jsonBytes
		return ret, nil
	}
}

// 把一个内部序列化格式 *internalStruct
// 按照给定的目标类型 typ ，恢复成真正的 Go 值
// 最终返回 any
func internalUnmarshal(v *internalStruct, typ reflect.Type) (any, error) {
	if v == nil {
		return nil, nil
	}

	// 说明需要使用传入的 typ 类型来反序列化
	if v.Type == nil {
		if checkMarshaler(typ) {
			pv := reflect.New(typ)
			err := json.Unmarshal(v.JSONValue, pv.Interface())
			if err != nil {
				return nil, err
			}
			// 取指针指向的实际值，然后返回它
			return pv.Elem().Interface(), nil
		}
		// 按照指定的 typ 类型来反序列化
		return internalSpecificTypeUnmarshal(v, typ)
	}

	if len(v.Type.SimpleType) != 0 {
		t, ok := m[v.Type.SimpleType]
		if !ok {
			return nil, fmt.Errorf("unknown type key: %v", v.Type.SimpleType)
		}
		pResult := reflect.New(resolvePointerNum(v.Type.PointerNum, t))
		err := sonic.Unmarshal(v.JSONValue, pResult.Interface())
		if err != nil {
			return nil, fmt.Errorf("unmarshal type[%s] fail: %v, data: %s", t.String(), err, string(v.JSONValue))
		}
		// reflect.New(...) 多包了一层指针，所以这里用 Elem() 去掉这一层
		return pResult.Elem().Interface(), nil
	}

	if len(v.Type.StructType) > 0 {
		rt, ok := m[v.Type.StructType]
		if !ok {
			return nil, fmt.Errorf("unknown type key: %v", v.Type.StructType)
		}
		result, dResult := createValueFromType(resolvePointerNum(v.Type.PointerNum, rt))

		// 利用解引用的值设置好字段
		err := setStructFields(dResult, v.MapValues)
		if err != nil {
			return nil, err
		}
		// 返回指向填好值的值的指针类型
		return result.Interface(), nil
	}

	if v.Type.MapKeyType != nil {
		rkt, err := restoreType(v.Type.MapKeyType)
		if err != nil {
			return nil, err
		}
		rvt, err := restoreType(v.Type.MapValueType)
		if err != nil {
			return nil, err
		}

		// MapOf 用于将 rkt 和 rvt 组合成一个 map 类型
		result, dResult := createValueFromType(reflect.MapOf(rkt, rvt))
		err = setMapKVs(dResult, v.MapValues)
		if err != nil {
			return nil, err
		}
		return result.Interface(), nil
	}

	// 到这里说明是 slice 类型
	rvt, err := restoreType(v.Type.SliceValueType)
	if err != nil {
		return nil, err
	}

	// SliceOf 用于将 rvt 组合成一个 slice 类型
	result, dResult := createValueFromType(reflect.SliceOf(rvt))
	err = setSliceElems(dResult, v.SliceValues)
	if err != nil {
		return nil, err
	}
	return result.Interface(), nil
}

// 根据自定义的 valueType 生成相对应的 reflect.Type
func restoreType(vt *valueType) (reflect.Type, error) {
	if vt.SimpleType != "" {
		rt, ok := m[vt.SimpleType]
		if !ok {
			return nil, fmt.Errorf("unknown type key: %s", vt.SimpleType)
		}
		return resolvePointerNum(vt.PointerNum, rt), nil
	}
	if vt.StructType != "" {
		rt, ok := m[vt.StructType]
		if !ok {
			return nil, fmt.Errorf("unknown type key: %s", vt.StructType)
		}
		return resolvePointerNum(vt.PointerNum, rt), nil
	}
	// 因为 map_key 是组合类型，所以需要递归处理
	if vt.MapKeyType != nil {
		rkt, err := restoreType(vt.MapKeyType)
		if err != nil {
			return nil, err
		}
		rvt, err := restoreType(vt.MapValueType)
		if err != nil {
			return nil, err
		}
		return resolvePointerNum(vt.PointerNum, reflect.MapOf(rkt, rvt)), nil
	}
	if vt.SliceValueType != nil {
		rt, err := restoreType(vt.SliceValueType)
		if err != nil {
			return nil, err
		}
		return resolvePointerNum(vt.PointerNum, reflect.SliceOf(rt)), nil
	}
	return nil, fmt.Errorf("empty value")
}

// 根据指针数量了，给类型添加相应的指针数量
func resolvePointerNum(pointerNum uint32, t reflect.Type) reflect.Type {
	for i := uint32(0); i < pointerNum; i++ {
		t = reflect.PointerTo(t)
	}
	return t
}

// 我已经知道目标类型 typ 了，现在根据 typ 是 struct / map / slice / 普通类型，
// 把 is 里的数据填回去
func internalSpecificTypeUnmarshal(is *internalStruct, typ reflect.Type) (any, error) {
	_, dtyp := derefPointerNum(typ)
	result, dResult := createValueFromType(typ)

	if dtyp.Kind() == reflect.Struct {
		err := setStructFields(dResult, is.MapValues)
		if err != nil {
			return nil, err
		}
		return dResult.Interface(), nil
	} else if dtyp.Kind() == reflect.Map {
		err := setMapKVs(dResult, is.MapValues)
		if err != nil {
			return nil, err
		}
		return dResult.Interface(), nil
	} else if dtyp.Kind() == reflect.Array || dtyp.Kind() == reflect.Slice {
		err := setSliceElems(dResult, is.SliceValues)
		if err != nil {
			return nil, err
		}
		return result.Interface(), nil
	}
	// simple type
	v := reflect.New(typ)
	err := sonic.Unmarshal(is.JSONValue, v.Interface())
	if err != nil {
		return nil, fmt.Errorf("unmarshal type[%s] fail: %v", typ.String(), err)
	}
	return v.Elem().Interface(), nil
}

func setSliceElems(dResult reflect.Value, values []*internalStruct) error {
	t := dResult.Type()

	// Handle arrays differently from slices
	// Arrays have fixed size and cannot use reflect.Append
	// 如果是 array ，就按下标一个个 Set，如果下标超出数组长度，就报错
	if dResult.Kind() == reflect.Array {
		for i, internalValue := range values {
			if i >= dResult.Len() {
				return fmt.Errorf("array index out of bounds: trying to set index %d in array of length %d", i, dResult.Len())
			}
			value, err := internalUnmarshal(internalValue, t.Elem())
			if err != nil {
				return fmt.Errorf("unmarshal array[%s] element %d fail: %v", t.Elem(), i, err)
			}
			if value == nil {
				dResult.Index(i).Set(reflect.Zero(t.Elem()))
			} else {
				dResult.Index(i).Set(reflect.ValueOf(value))
			}
		}
		return nil
	}

	// For slices, use Append as before
	for _, internalValue := range values {
		value, err := internalUnmarshal(internalValue, t.Elem())
		if err != nil {
			return fmt.Errorf("unmarshal slice[%s] fail: %v", t.Elem(), err)
		}
		if value == nil {
			// empty value
			dResult.Set(reflect.Append(dResult, reflect.New(t.Elem()).Elem()))
		} else {
			dResult.Set(reflect.Append(dResult, reflect.ValueOf(value)))
		}
	}
	return nil
}

// 把 map[string]*internalStruct 这种中间表示，恢复成真正的 Go map[K]V
func setMapKVs(dResult reflect.Value, values map[string]*internalStruct) error {
	// 拿到 map 的类型信息，如果目标是 map[int]User，t.Key() 是 int，t.Elem() 是 User
	t := dResult.Type()
	for marshaledMapKey, internalValue := range values {
		// new 一个 key 类型的零值
		prkv := reflect.New(t.Key())
		// 反序列化 marshaledMapKey 到 prkv 中, eg. "123" -> int(123)
		err := sonic.UnmarshalString(marshaledMapKey, prkv.Interface())
		if err != nil {
			return fmt.Errorf("unmarshal map key[%v] to type[%v] fail: %v", marshaledMapKey, t.Key(), err)
		}

		// t.Elem() 是 map 的 value 类型
		value, err := internalUnmarshal(internalValue, t.Elem())
		if err != nil {
			return fmt.Errorf("unmarshal map value fail: %v", err)
		}

		if value == nil {
			dResult.SetMapIndex(prkv.Elem(), reflect.New(t.Elem()).Elem())
		} else {
			dResult.SetMapIndex(prkv.Elem(), reflect.ValueOf(value))
		}
	}
	return nil
}

// 从 values 里按字段名拿数据，反序列化成对应 Go 值，然后设置到 dResult 的同名字段中
func setStructFields(dResult reflect.Value, values map[string]*internalStruct) error {
	// 目标结构体类型
	t := dResult.Type()

	for k, internalValue := range values {
		// 从目标结构体类型中获取字段 k 对应的字段元信息
		sf, ok := t.FieldByName(k)
		if !ok {
			continue
		}
		// 反序列化 internalValue 到 sf.Type 类型
		value, err := internalUnmarshal(internalValue, sf.Type)
		if err != nil {
			return fmt.Errorf("unmarshal map field[%v] fail: %v", k, err)
		}
		// 把反序列化出来后的值设置到 dResult 的同名字段中
		err = setStructField(t, dResult, k, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func setStructField(t reflect.Type, s reflect.Value, fieldName string, val any) error {
	field := s.FieldByName(fieldName)
	// 检测这个 field 是否可以被修改
	if !field.CanSet() {
		return fmt.Errorf("unmarshal map fail, can not set field %v", fieldName)
	}
	// 如果反序列化出来的是 nil，获取字段的类型，并创建一个零值
	if val == nil {
		rft, ok := t.FieldByName(fieldName)
		if !ok {
			return fmt.Errorf("unmarshal map fail, can not find field %v", fieldName)
		}
		field.Set(reflect.New(rft.Type).Elem())
	} else {
		field.Set(reflect.ValueOf(val))
	}
	return nil
}

// 把一个类型上的指针层数剥掉，并返回剥了几层
func derefPointerNum(typ reflect.Type) (uint32, reflect.Type) {
	var ptrCount uint32 = 0
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		ptrCount++
	}
	return ptrCount, typ
}

// 返回根据 t 类型创建的值，以及把 value 所有指针都解引用之后的值 (返回的两个值是有联系的）
func createValueFromType(t reflect.Type) (value reflect.Value, derefValue reflect.Value) {
	// 创建一个 t 类型的值
	value = reflect.New(t).Elem()

	// 获取实际值
	derefValue = value
	// 考虑多层指针，递归创建指针类型的值
	for derefValue.Kind() == reflect.Ptr {
		// 指针没有指向东西
		if derefValue.IsNil() {
			// 创建一个指针类型的值，并指向它
			// 类似于 var p *User = nil; p = &User{}
			derefValue.Set(reflect.New(derefValue.Type().Elem()))
		}
		// 变为指针指向的实际值
		derefValue = derefValue.Elem()
	}

	// 如果是 map 类型，且为 nil，就创建一个空的 map 值并指向它
	if derefValue.Kind() == reflect.Map && derefValue.IsNil() {
		derefValue.Set(reflect.MakeMap(derefValue.Type()))
	}

	// 把 var s []int = nil 变成 s := make([]int, 0) 这样的空 slice
	if derefValue.Kind() == reflect.Slice {
		if derefValue.Len() == 0 && derefValue.Cap() == 0 {
			derefValue.Set(reflect.MakeSlice(derefValue.Type(), 0, 0))
		}
	}

	return value, derefValue
}

/*
map[string][]*int

会被解析成：

	&valueType{
		MapKeyType: &valueType{
			SimpleType: "_eino_string",
		},
		MapValueType: &valueType{
			SliceValueType: &valueType{
				PointerNum: 1,
				SimpleType: "_eino_int",
			},
		},
	}
*/
func extractType(t reflect.Type) (*valueType, error) {
	ret := &valueType{}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var err error
	if t.Kind() == reflect.Map {
		// 递归解析 map 的键类型
		ret.MapKeyType, err = extractType(t.Key())
		if err != nil {
			return nil, err
		}
		// 递归解析 map 的值类型
		ret.MapValueType, err = extractType(t.Elem())
		if err != nil {
			return nil, err
		}
	} else if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		// 递归解析 slice 或 array 的元素类型
		ret.SliceValueType, err = extractType(t.Elem())
		if err != nil {
			return nil, err
		}
	} else {
		// 进入这里 说明 t 是一个简单值类型
		key, ok := rm[t]
		if !ok {
			return ret, fmt.Errorf("unknown type %s", t.String())
		}
		ret.SimpleType = key
	}
	return ret, nil
}

// 拿到标准库的 序列化/反序列化 接口类型
var marshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
var unmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()

// 检查类型 t 是否实现了 Marshaler 接口和 Unmarshaler 接口 (自定义序列化逻辑)
func checkMarshaler(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// 判断值类型 T 和指针类型 *T 是否实现 Marshaler 接口和 Unmarshaler 接口
	if t.Implements(marshalerType) || reflect.PointerTo(t).Implements(marshalerType) &&
		(t.Implements(unmarshalerType) || reflect.PointerTo(t).Implements(unmarshalerType)) {
		return true
	}

	return false
}
