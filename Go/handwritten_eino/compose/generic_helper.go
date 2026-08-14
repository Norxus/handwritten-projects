package compose

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/schema"
)

type genericHelper struct {
	inputStreamFilter, outputStreamFilter                   streamMapFilter
	inputConverter, outputConverter                         handlerPair
	inputFieldMappingConverter, outputFieldMappingConverter handlerPair
	inputStreamConvertPair, outputStreamConvertPair         streamConvertPair
	inputZeroValue, outputZeroValue                         func() any
	inputEmptyStream, outputEmptyStream                     func() streamReader
}

type streamMapFilter func(key string, isr streamReader) (streamReader, bool)

type handlerPair struct {
	invoke    valueHandler
	transform streamHandler
}

type valueHandler func(value any) (any, error)
type streamHandler func(streamReader) streamReader

func newGenericHelper[I, O any]() *genericHelper {
	return &genericHelper{
		inputStreamFilter:  defaultStreamMapFilter[I],
		outputStreamFilter: defaultStreamMapFilter[O],
		inputConverter: handlerPair{
			invoke:    defaultValueChecker[I],
			transform: defaultStreamConverter[I],
		},
		outputConverter: handlerPair{
			invoke:    defaultValueChecker[O],
			transform: defaultStreamConverter[O],
		},
		inputFieldMappingConverter: handlerPair{
			invoke:    buildFieldMappingConverter[I](),
			transform: buildStreamFieldMappingConverter[I](),
		},
		outputFieldMappingConverter: handlerPair{
			invoke:    buildFieldMappingConverter[O](),
			transform: buildStreamFieldMappingConverter[O](),
		},
		inputStreamConvertPair:  defaultStreamConvertPair[I](),
		outputStreamConvertPair: defaultStreamConvertPair[O](),
		inputZeroValue:          zeroValueFromGeneric[I],
		outputZeroValue:         zeroValueFromGeneric[O],
		inputEmptyStream:        emptyStreamFromGeneric[I],
		outputEmptyStream:       emptyStreamFromGeneric[O],
	}
}

// 从一个“元素类型是 map[string]any 的流”里，按指定 key 取值，并把它过滤/转换成“元素类型是 T 的流”
func defaultStreamMapFilter[T any](key string, isr streamReader) (streamReader, bool) {
	// 把类型转换为 map[string]any 的流
	sr, ok := unpackStreamReader[map[string]any](isr)
	if !ok {
		return nil, false
	}

	// 处理 map[string]any 的元素
	cvt := func(m map[string]any) (T, error) {
		var t T
		// 提取指定 key 的值
		v, ok_ := m[key]
		if !ok_ {
			return t, schema.ErrNoValue
		}
		// 将值转换为 T 类型
		vv, ok_ := v.(T)
		if !ok_ {
			return t, fmt.Errorf(
				"[defaultStreamMapFilter]fail, key[%s]'s value type[%s] isn't expected type[%s]",
				key, reflect.TypeOf(v).String(), reflect.TypeOf(t).String())
		}
		return vv, nil
	}

	ret := schema.StreamReaderWithConvert[map[string]any, T](sr, cvt)

	// 重新把流包装回去
	return packStreamReader(ret), true
}

// 把 streamReader 流转换为“元素类型必须是 T ”的新流
func defaultStreamConverter[T any](reader streamReader) streamReader {
	//通用的 streamReader 流 ->  any 流 -> convert T 流（顺便可以做类型校验） -> 向上转型为通用的 streamReader 流
	return packStreamReader(schema.StreamReaderWithConvert(reader.toAnyStreamReader(), func(v any) (T, error) {
		vv, ok := v.(T)
		if !ok {
			var t T
			return t, fmt.Errorf("runtime type check fail, expected type: %T, actual type: %T", t, v)
		}
		return vv, nil
	}))
}

// 尝试将 v 进行 T 的断言
func defaultValueChecker[T any](v any) (any, error) {
	nValue, ok := v.(T)
	if !ok {
		var t T
		return nil, fmt.Errorf("runtime type check fail, expected type: %T, actual type: %T", t, v)
	}
	return nValue, nil
}

// 直接返回一个类型空值
func zeroValueFromGeneric[T any]() any {
	var t T
	return t
}

func emptyStreamFromGeneric[T any]() streamReader {
	var t T
	// 创建一个 stream，返回 reader 和 writer
	sr, sw := schema.Pipe[T](1)
	// 发送一个 T 的零值，然后关闭这个 stream
	sw.Send(t, nil)
	sw.Close()
	// 返回 reader，接收方只能从这个 reader 中接收到 1 个零值
	return packStreamReader(sr)
}

type streamConvertPair struct {
	concatStream  func(sr streamReader) (any, error)
	restoreStream func(any) (streamReader, error)
}

// 为泛型类型 T 构造一对“流 <-> 单值”的默认转换器
func defaultStreamConvertPair[T any]() streamConvertPair {
	var t T
	return streamConvertPair{
		concatStream: func(sr streamReader) (any, error) {
			tsr, ok := unpackStreamReader[T](sr)
			if !ok {
				return nil, fmt.Errorf("cannot convert sr to streamReader[%T]", t)
			}
			// 把流的所有元素进行合并，实际上就是合并 slice 或 map
			value, err := concatStreamReader(tsr)
			if err != nil {
				if errors.Is(err, emptyStreamConcatErr) {
					return nil, nil
				}
				return nil, err
			}
			return value, nil
		},
		restoreStream: func(a any) (streamReader, error) {
			if a == nil {
				return packStreamReader(schema.StreamReaderFromArray([]T{})), nil
			}
			value, ok := a.(T)
			if !ok {
				return nil, fmt.Errorf("cannot convert value[%T] to streamReader[%T]", a, t)
			}
			// 把单值转换为只包含该单值的流
			return packStreamReader(schema.StreamReaderFromArray([]T{value})), nil
		},
	}
}
