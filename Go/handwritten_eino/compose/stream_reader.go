package compose

import (
	"github.com/cloudwego/eino/internal/generic"
	"reflect"

	"github.com/cloudwego/eino/schema"
)

type streamReader interface {
	Copy(n int) []streamReader
	getType() reflect.Type
	getChunkType() reflect.Type
	merge([]streamReader) streamReader
	withKey(string) streamReader
	close()
	toAnyStreamReader() *schema.StreamReader[any]
	mergeWithNames([]streamReader, []string) streamReader
}

type streamReaderPacker[T any] struct {
	sr *schema.StreamReader[T]
}

func (srp streamReaderPacker[T]) close() {
	srp.sr.Close()
}

func (srp streamReaderPacker[T]) Copy(n int) []streamReader {
	ret := make([]streamReader, n)
	srs := srp.sr.Copy(n)

	for i := 0; i < n; i++ {
		ret[i] = streamReaderPacker[T]{srs[i]}
	}
	return ret
}

// 内部使用的 streamReader 类型
func (srp streamReaderPacker[T]) getType() reflect.Type {
	return reflect.TypeOf(srp.sr)
}

// 通道元素类型
func (srp streamReaderPacker[T]) getChunkType() reflect.Type {
	return generic.TypeOf[T]()
}

// 把内部的 streamReader 拿出来，并把自己内部的 streamReader 放在第一个
func (srp streamReaderPacker[T]) toStreamReaders(srs []streamReader) []*schema.StreamReader[T] {
	ret := make([]*schema.StreamReader[T], len(srs)+1)
	ret[0] = srp.sr
	for i := 1; i < len(ret); i++ {
		sr, ok := unpackStreamReader[T](srs[i-1])
		if !ok {
			return nil
		}
		ret[i] = sr
	}
	return ret
}

func (srp streamReaderPacker[T]) merge(isrs []streamReader) streamReader {
	srs := srp.toStreamReaders(isrs)
	sr := schema.MergeStreamReaders(srs)
	return packStreamReader(sr)
}

// 把一组带名字的 StreamReader[T] 合并成一个 StreamReader[T]
func (srp streamReaderPacker[T]) mergeWithNames(isrs []streamReader, names []string) streamReader {
	srs := srp.toStreamReaders(isrs)
	sr := schema.InternalMergeNamedStreamReaders(srs, names)
	return packStreamReader(sr)
}

// 在外面包一层 convert，把每个元素转换为 map[string]any
func (srp streamReaderPacker[T]) withKey(key string) streamReader {
	cvt := func(v T) (map[string]any, error) {
		return map[string]any{key: v}, nil
	}

	ret := schema.StreamReaderWithConvert(srp.sr, cvt)
	return packStreamReader(ret)
}

// 在外面包一层 convert，把每个元素转换为 any
func (srp streamReaderPacker[T]) toAnyStreamReader() *schema.StreamReader[any] {
	return schema.StreamReaderWithConvert(srp.sr, func(t T) (any, error) {
		return t, nil
	})
}

func packStreamReader[T any](sr *schema.StreamReader[T]) streamReader {
	return streamReaderPacker[T]{sr}
}

// 尝试把 compose 层的通用 streamReader ，还原成指定类型的 *schema.StreamReader[T]
func unpackStreamReader[T any](isr streamReader) (*schema.StreamReader[T], bool) {
	c, ok := isr.(streamReaderPacker[T])
	if ok {
		return c.sr, true
	}

	// 获取泛型 T 的类型
	typ := generic.TypeOf[T]()
	// 如果 T 是接口类型
	if typ.Kind() == reflect.Interface {
		// 先把每个元素当 any 读出来（向上转型）, 再转换为 T 类型（向下转型为一个接口类型）
		return schema.StreamReaderWithConvert(isr.toAnyStreamReader(), func(t any) (T, error) {
			return t.(T), nil
		}), true
	}
	return nil, false
}
