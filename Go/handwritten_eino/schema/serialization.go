package schema

import (
	"encoding/gob"
	"reflect"

	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/internal/serialization"
)

func init() {
	RegisterName[Message]("_eino_message")
	RegisterName[[]*Message]("_eino_message_slice")
	RegisterName[Document]("_eino_document")
	RegisterName[RoleType]("_eino_role_type")
	RegisterName[ToolCall]("_eino_tool_call")
	RegisterName[FunctionCall]("_eino_function_call")
	RegisterName[ResponseMeta]("_eino_response_meta")
	RegisterName[TokenUsage]("_eino_token_usage")
	RegisterName[LogProbs]("_eino_log_probs")
	RegisterName[ChatMessagePart]("_eino_chat_message_part")
	RegisterName[ChatMessagePartType]("_eino_chat_message_type")
	RegisterName[ChatMessageImageURL]("_eino_chat_message_image_url")
	RegisterName[ChatMessageAudioURL]("_eino_chat_message_audio_url")
	RegisterName[ChatMessageVideoURL]("_eino_chat_message_video_url")
	RegisterName[ChatMessageFileURL]("_eino_chat_message_file_url")
	RegisterName[MessageInputPart]("_eino_message_input_part")
	RegisterName[MessageInputImage]("_eino_message_input_image")
	RegisterName[MessageInputAudio]("_eino_message_input_audio")
	RegisterName[MessageInputVideo]("_eino_message_input_video")
	RegisterName[MessageInputFile]("_eino_message_input_file")
	RegisterName[MessageOutputPart]("_eino_message_output_part")
	RegisterName[MessageOutputImage]("_eino_message_output_image")
	RegisterName[MessageOutputAudio]("_eino_message_output_audio")
	RegisterName[MessageOutputVideo]("_eino_message_output_video")
	RegisterName[MessagePartCommon]("_eino_message_part_common")
	RegisterName[ImageURLDetail]("_eino_image_url_detail")
	RegisterName[PromptTokenDetails]("_eino_prompt_token_details")
}

// 给任意类型 T 注册一个指定名字
func RegisterName[T any](name string) {
	// 把某个具体类型 T 注册到 Go 的 encoding/gob 序列化系统里
	gob.RegisterName(name, generic.NewInstance[T]())

	err := serialization.GenericRegister[T](name)
	if err != nil {
		panic(err)
	}
}

// 获取类型名（全限定名）
func getTypeName(rt reflect.Type) string {
	name := rt.String()

	star := ""

	// 当前类型本身不是命名类型
	if rt.Name() == "" {
		// 如果它是指针，就拆一层 * 看看它指向的元素是不是命名类型
		if pt := rt; pt.Kind() == reflect.Pointer {
			// 用 star 记录这是一个指针
			star = "*"
			rt = pt.Elem()
		}
	}

	// 如果是命名类型，有包前缀就加上包前缀
	if rt.Name() != "" {
		if rt.PkgPath() == "" {
			name = star + rt.Name()
		} else {
			name = star + rt.PkgPath() + "." + rt.Name()
		}
	}
	// 如果最终仍然不是命名类型，就直接返回最开始的 rt.String()
	return name
}

func Register[T any]() {
	value := generic.NewInstance[T]()

	gob.Register(value)

	// 获取全限定名
	name := getTypeName(reflect.TypeOf(value))

	// 把 T 按这个名字注册到项目自己的序列化注册表里
	err := serialization.GenericRegister[T](name)
	if err != nil {
		panic(err)
	}
}


