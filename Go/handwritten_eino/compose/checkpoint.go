package compose

import "github.com/cloudwego/eino/internal/core"

type CheckPointStore = core.CheckPointStore

type Serializer interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}
