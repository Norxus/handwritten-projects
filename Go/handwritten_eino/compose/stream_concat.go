package compose

import (
	"errors"
	"io"

	"github.com/cloudwego/eino/internal"
	"github.com/cloudwego/eino/schema"
)

var emptyStreamConcatErr = errors.New("stream reader is empty, concat fail")

// 把一个流式返回的 StreamReader[T] 读完，并合并成一个最终的单值 T (map or slice)
func concatStreamReader[T any](sr *schema.StreamReader[T]) (T, error) {
	// 关闭这个流
	defer sr.Close()

	var items []T

	for {
		chunk, err := sr.Recv()
		if err != nil {
			// 全部流都读完了，退出
			if err == io.EOF {
				break
			}

			// 只是某个流的数据读完了，继续读下一个流
			if _, ok := schema.GetSourceName(err); ok {
				continue
			}

			var t T
			return t, newStreamReadError(err)
		}

		// 读取的数据合并
		items = append(items, chunk)
	}

	if len(items) == 0 {
		var t T
		return t, emptyStreamConcatErr
	}

	if len(items) == 1 {
		return items[0], nil
	}

	// 多个数据，就开始合并
	res, err := internal.ConcatItems(items)
	if err != nil {
		var t T
		return t, err
	}
	return res, nil
}