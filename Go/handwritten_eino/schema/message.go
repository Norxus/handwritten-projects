package schema

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/internal"
	"github.com/cloudwego/eino/internal/generic"
)

func init() {
	internal.RegisterStreamChunkConcatFunc(ConcatMessage)
	internal.RegisterStreamChunkConcatFunc(ConcatMessageArray)
	internal.RegisterStreamChunkConcatFunc(ConcatToolResults)
}

type RoleType string

const (
	Assistant RoleType = "assistant"
	User      RoleType = "user"
	System    RoleType = "system"
	Tool      RoleType = "tool"
)

type ChatMessagePartType string

const (
	ChatMessagePartTypeText      ChatMessagePartType = "text"
	ChatMessagePartTypeImageURL  ChatMessagePartType = "image_url"
	ChatMessagePartTypeAudioURL  ChatMessagePartType = "audio_url"
	ChatMessagePartTypeVideoURL  ChatMessagePartType = "video_url"
	ChatMessagePartTypeFileURL   ChatMessagePartType = "file_url"
	ChatMessagePartTypeReasoning ChatMessagePartType = "reasoning"
)

type Message struct {
	Role                     RoleType            `json:"role"`
	Content                  string              `json:"content"`
	MultiContent             []ChatMessagePart   `json:"multi_content,omitempty"`
	UserInputMultiContent    []MessageInputPart  `json:"user_input_multi_content,omitempty"`
	AssistantGenMultiContent []MessageOutputPart `json:"assistant_output_multi_conent,omitempty"`
	Name                     string              `json:"name,omitempty"`
	ToolCalls                []ToolCall          `json:"tool_calls,omitempty"`
	ToolCallID               string              `json:"tool_call_id,omitempty"`
	ToolName                 string              `json:"tool_name,omitempty"`
	ResponseMeta             *ResponseMeta       `json:"response_meta,omitempty"`
	ReasoningContent         string              `json:"reasoning_content,omitempty"`
	Extra                    map[string]any      `json:"extra,omitempty"`
}

type ResponseMeta struct {
	FinishReason string      `json:"finish_reason,omitemtpy"`
	Usage        *TokenUsage `json:"usage,omitempty"`
	LogProbs     *LogProbs   `json:"logprobs,omitempty"`
}

type TokenUsage struct {
	PromptTokens            int                    `json:"prompt_tokens"`
	PromptTokenDetails      PromptTokenDetails     `json:"prompt_token_details"`
	CompletionTokens        int                    `json:"completion_tokens"`
	TotalTokens             int                    `json:"total_tokens"`
	CompletionTokensDetails CompletionTokenDetails `json:"completion_token_details"`
}

type LogProbs struct {
	Content []LogProb `json:"content"`
}

type LogProb struct {
	Token       string       `json:"token"`
	LogProb     float64      `json:"logprob"`
	Bytes       []int64      `json:"bytes,omitempty"`
	TopLogProbs []TopLogProb `json:"top_logprobs"`
}

type TopLogProb struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
	Bytes   []int64 `json:"bytes,omitempty"`
}

type PromptTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type CompletionTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

type ChatMessagePart struct {
	Type     ChatMessagePartType  `json:"type,omitempty"`
	Text     string               `json:"text,omitempty"`
	ImageURL *ChatMessageImageURL `json:"image_url,omitempty"`
	AudioURL *ChatMessageAudioURL `json:"audio_url,omitempty"`
	VideoURL *ChatMessageVideoURL `json:"video_url,omitempty"`
	FileURL  *ChatMessageFileURL  `json:"file_url,omitempty"`
}

type ChatMessageImageURL struct {
	URL      string         `json:"url,omitempty"`
	URI      string         `json:"uri,omitempty"`
	Detail   ImageURLDetail `json:"detail,omitempty"`
	MIMEType string         `json:"mime_type,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}

type ChatMessageAudioURL struct {
	URL      string         `json:"url,omitempty"`
	URI      string         `json:"uri,omitempty"`
	MIMEType string         `json:"mime_type,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}

type ChatMessageVideoURL struct {
	URL      string         `json:"url,omitempty"`
	URI      string         `json:"uri,omitempty"`
	MIMEType string         `json:"mime_type,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}

type ChatMessageFileURL struct {
	URL      string         `json:"url,omitempty"`
	URI      string         `json:"uri,omitempty"`
	MIMEType string         `json:"mime_type,omitempty"`
	Name     string         `json:"name,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}

type MessageInputPart struct {
	Type  ChatMessagePartType `json:"type"`
	Text  string              `json:"text,omitempty"`
	Image *MessageInputImage  `json:"image,omitempty"`
	Audio *MessageInputAudio  `json:"audio,omitempty"`
	Video *MessageInputVideo  `json:"video,omitempty"`
	File  *MessageInputFile   `json:"file,omitempty"`
	Extra map[string]any      `json:"extra,omitempty"`
}

type MessageInputImage struct {
	MessagePartCommon
	Detail ImageURLDetail `json:"detail,omitempty"`
}

type MessagePartCommon struct {
	URL        *string        `json:"url,omitempty"`
	Base64Data *string        `json:"base64data,omitempty"`
	MIMEType   string         `json:"mime_type,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

type ImageURLDetail string

const (
	ImageURLDetailHigh ImageURLDetail = "high"
	ImageURLDetailLow  ImageURLDetail = "low"
	ImageURLDetailAuto ImageURLDetail = "auto"
)

type MessageInputAudio struct {
	MessagePartCommon
}

type MessageInputVideo struct {
	MessagePartCommon
}

type MessageInputFile struct {
	MessagePartCommon

	Name string `json:"name,omitempty"`
}

type MessageOutputPart struct {
	Type          ChatMessagePartType     `json:"type"`
	Text          string                  `json:"text,omitempty"`
	Image         *MessageOutputImage     `json:"image,omitempty"`
	Audio         *MessageOutputAudio     `json:"audio,omitempty"`
	Video         *MessageOutputVideo     `json:"video,omitempty"`
	Reasoning     *MessageOutputReasoning `json:"reasoning,omitempty"`
	Extra         map[string]any          `json:"extra,omitempty"`
	StreamingMeta *MessageStreamingMeta   `json:"-"`
}

type MessageOutputImage struct {
	MessagePartCommon
}

type MessageOutputAudio struct {
	MessagePartCommon
}

type MessageOutputVideo struct {
	MessagePartCommon
}

type MessageOutputReasoning struct {
	Text      string `json:"text,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type MessageStreamingMeta struct {
	Index int `json:"index,omitempty"`
}

type ToolCall struct {
	Index    *int           `json:"index, omitempty"`
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function FunctionCall   `json:"function"`
	Extra    map[string]any `json:"extra,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ToolArgument struct {
	Text string `json:"text,omitempty"`
}

type ToolResult struct {
	Parts []ToolOutputPart `json:"parts,omitempty"`
}

type ToolOutputPart struct {
	Type  ToolPartType     `json:"type"`
	Text  string           `json:"text,omitempty"`
	Image *ToolOutputImage `json:"image,omitemtpy"`
	Audio *ToolOutputAudio `json:"audio,omitempty"`
	Video *ToolOutputVideo `json:"video,omitempty"`
	File  *ToolOutputFile  `json:"file,omitempty"`
	Extra map[string]any   `json:"extra,omitempty"`
}

type ToolPartType string

const (
	ToolPartTypeText  ToolPartType = "text"
	ToolPartTypeImage ToolPartType = "image"
	ToolPartTypeAudio ToolPartType = "audio"
	ToolPartTypeVideo ToolPartType = "video"
	ToolPartTypeFile  ToolPartType = "file"
)

type ToolOutputImage struct {
	MessagePartCommon
}

type ToolOutputAudio struct {
	MessagePartCommon
}

type ToolOutputVideo struct {
	MessagePartCommon
}

type ToolOutputFile struct {
	MessagePartCommon
}
type FormatType uint8

const (
	FString    FormatType = 0
	GoTemplate FormatType = 1
	Jinja2     FormatType = 2
)

type MessagesTemplate interface {
	Format(ctx context.Context, vs map[string]any, formatType FormatType) ([]*Message, error)
}

type toolMessageOptions struct {
	toolName string
}

type ToolMessageOption func(*toolMessageOptions)

func WithToolName(name string) ToolMessageOption {
	return func(o *toolMessageOptions) {
		o.toolName = name
	}
}

// 返回工具调用的结果消息
func ToolMessage(content string, toolCallID string, opts ...ToolMessageOption) *Message {
	o := &toolMessageOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return &Message{
		Role:       Tool,
		Content:    content,
		ToolCallID: toolCallID,
		ToolName:   o.toolName,
	}
}

// 过滤二维数组 [][]*Message，实际上就是批量底层调用了 ConcatMessage
func ConcatMessageArray(mas [][]*Message) ([]*Message, error) {
	arrayLen := len(mas[0])

	ret := make([]*Message, arrayLen)
	slicesToConcat := make([][]*Message, arrayLen)

	for _, ma := range mas {
		// 每一个子数组的长度都应该相等
		if len(ma) != arrayLen {
			return nil, fmt.Errorf("unexpected array length. "+
				"Got %d, expected %d", len(ma), arrayLen)
		}

		// 过滤 nil
		for i := 0; i < arrayLen; i++ {
			m := ma[i]
			if m != nil {
				slicesToConcat[i] = append(slicesToConcat[i], m)
			}
		}
	}

	for i, slice := range slicesToConcat {
		if len(slice) == 0 {
			ret[i] = nil
		} else if len(slice) == 1 {
			ret[i] = slice[0]
		} else {
			cm, err := ConcatMessage(slice)
			if err != nil {
				return nil, err
			}

			ret[i] = cm
		}
	}

	return ret, nil
}

func ConcatToolResults(chunks []*ToolResult) (*ToolResult, error) {
	if len(chunks) == 0 {
		return &ToolResult{}, nil
	}

	nonTextPartType := make(map[ToolPartType]int)

	var allParts []ToolOutputPart
	for chunkIdx, chunk := range chunks {
		if chunk == nil || len(chunk.Parts) == 0 {
			continue
		}

		for _, part := range chunk.Parts {
			// 当前这个非文本类型之前如果出现在其他 chunk 里，直接报错
			if part.Type != ToolPartTypeText {
				// 文本输出可能是流式一点点吐出来的，所以可以拆成多个 chunk，再合并
				// 但像图片、音频、文件这种非文本内容，通常是一个完整对象，不适合在多个 chunk 中重复声明同一种类型
				if prevChunkIdx, exists := nonTextPartType[part.Type]; exists {
					return nil, fmt.Errorf("conflecting %s parts found in chunk %d and chunk %d:"+
						"non-text modality parts cannot appear in multiple chunks", part.Type, prevChunkIdx, chunkIdx)
				}
				nonTextPartType[part.Type] = chunkIdx
			}
		}

		mergedChunkParts, err := concatToolOutputParts(chunk.Parts)
		if err != nil {
			return nil, fmt.Errorf("failed to merge text parts in chunk %d: %w", chunkIdx, err)
		}

		allParts = append(allParts, mergedChunkParts...)
	}

	if len(allParts) == 0 {
		return &ToolResult{}, nil
	}

	return &ToolResult{Parts: allParts}, nil
}

// 分组后合并
func concatToolOutputParts(parts []ToolOutputPart) ([]ToolOutputPart, error) {
	if len(parts) == 0 {
		return nil, nil
	}

	groups := groupToolOutputParts(parts)

	merged := make([]ToolOutputPart, 0, len(groups))
	for _, group := range groups {
		if len(group) == 1 {
			merged = append(merged, group...)
			continue
		}

		switch group[0].Type {
		case ToolPartTypeText:
			mergedPart, err := mergeToolTextParts(group)
			if err != nil {
				return nil, err
			}
			merged = append(merged, mergedPart)
		default:
			merged = append(merged, group...)
		}
	}

	return merged, nil
}

// 把连续的 Text 元素进行分组
func groupToolOutputParts(parts []ToolOutputPart) [][]ToolOutputPart {
	groups := make([][]ToolOutputPart, 0)
	i := 0
	for i < len(parts) {
		if parts[i].Type == ToolPartTypeText {
			end := i + 1
			// 继续向后找，直到遇到第一个非文本元素
			for end < len(parts) && parts[end].Type == ToolPartTypeText {
				end++
			}
			groups = append(groups, parts[i:end])
		} else {
			// 只把当前这一个元素 parts[i:i+1] 作为一组加入 groups
			groups = append(groups, parts[i:i+1])
			i++
		}
	}
	return groups
}

// 把 text 合并，extra map 合并
func mergeToolTextParts(group []ToolOutputPart) (ToolOutputPart, error) {
	var sb strings.Builder
	extraList := make([]map[string]any, 0, len(group))
	for _, part := range group {
		sb.WriteString(part.Text)
		if len(part.Extra) > 0 {
			extraList = append(extraList, part.Extra)
		}
	}

	var mergedExtra map[string]any
	if len(extraList) > 0 {
		var err error
		mergedExtra, err = concatExtra(extraList)
		if err != nil {
			return ToolOutputPart{}, fmt.Errorf("failed to concat tool output text part extra: %w", err)
		}
	}

	return ToolOutputPart{
		Type:  ToolPartTypeText,
		Text:  sb.String(),
		Extra: mergedExtra,
	}, nil
}

func ConcatMessage(msgs []*Message) (*Message, error) {
	var (
		contents                      []string
		contentLen                    int
		reasoningContents             []string
		reasoningContentLen           int
		toolCalls                     []ToolCall
		multiContentParts             []ChatMessagePart
		assistantGenMultiContentParts []MessageOutputPart
		userInputMultiContentParts    []MessageInputPart
		ret                           = Message{}
		extraList                     = make([]map[string]any, 0, len(msgs))
	)

	for idx, msg := range msgs {
		if msg == nil {
			return nil, fmt.Errorf("unexpected nil chunk in message stream, %d", idx)
		}

		if msg.Role != "" {
			if ret.Role == "" {
				ret.Role = msg.Role
				// 预期拼接的所有 msg 应该是 role 一致
			} else if ret.Role != msg.Role {
				return nil, fmt.Errorf("cannot concat message with "+
					"different roles: '%s'", ret.Role, msg.Role)
			}
		}

		// 预期所有的 msg 的 name 应该都是一致的
		if msg.Name != "" {
			if ret.Name == "" {
				ret.Name = msg.Name
			} else if ret.Name != msg.Name {
				return nil, fmt.Errorf("cannot concat message with"+
					" different names: '%s' '%s'", ret.Name, msg.Name)
			}
		}

		if msg.ToolCallID != "" {
			if ret.ToolCallID == "" {
				ret.ToolCallID = msg.ToolCallID
			} else if ret.ToolCallID != msg.ToolCallID {
				return nil, fmt.Errorf("cannot concat messages with"+
					" different toolCallIDs: '%s' '%s'", ret.ToolCallID, msg.ToolCallID)
			}
		}
		if msg.ToolName != "" {
			if ret.ToolName == "" {
				ret.ToolName = msg.ToolName
			} else if ret.ToolName != msg.ToolName {
				return nil, fmt.Errorf("cannot concat messages with"+
					" different toolNames: '%s' '%s'", ret.ToolCallID, msg.ToolCallID)
			}
		}

		if msg.Content != "" {
			contents = append(contents, msg.Content)
			contentLen += len(msg.Content)
		}
		if msg.ReasoningContent != "" {
			reasoningContents = append(reasoningContents, msg.ReasoningContent)
			reasoningContentLen += len(msg.ReasoningContent)
		}

		if len(msg.ToolCalls) > 0 {
			toolCalls = append(toolCalls, msg.ToolCalls...)
		}

		if len(msg.Extra) > 0 {
			extraList = append(extraList, msg.Extra)
		}

		if len(msg.MultiContent) > 0 {
			multiContentParts = append(multiContentParts, msg.MultiContent...)
		}
		if len(msg.AssistantGenMultiContent) > 0 {
			assistantGenMultiContentParts = append(assistantGenMultiContentParts, msg.AssistantGenMultiContent...)
		}
		if len(msg.UserInputMultiContent) > 0 {
			userInputMultiContentParts = append(userInputMultiContentParts, msg.UserInputMultiContent...)
		}

		if msg.ResponseMeta != nil && ret.ResponseMeta != nil {
			if msg.ResponseMeta.FinishReason != "" {
				ret.ResponseMeta.FinishReason = msg.ResponseMeta.FinishReason
			}

			// 对几个 token 统计字段做“取最大值”合并，而不是累加
			// 很多模型供应商在流式过程中上报的 usage 是“当前累计值”，后面的 chunk 会比前面的更完整
			if msg.ResponseMeta.Usage != nil {
				if ret.ResponseMeta.Usage == nil {
					ret.ResponseMeta.Usage = &TokenUsage{}
				}

				if msg.ResponseMeta.Usage.PromptTokens > ret.ResponseMeta.Usage.PromptTokens {
					ret.ResponseMeta.Usage.PromptTokens = msg.ResponseMeta.Usage.PromptTokens
				}

				if msg.ResponseMeta.Usage.CompletionTokens > ret.ResponseMeta.Usage.CompletionTokens {
					ret.ResponseMeta.Usage.CompletionTokens = msg.ResponseMeta.Usage.CompletionTokens
				}

				if msg.ResponseMeta.Usage.TotalTokens > ret.ResponseMeta.Usage.TotalTokens {
					ret.ResponseMeta.Usage.TotalTokens = msg.ResponseMeta.Usage.TotalTokens
				}

				if msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens > ret.ResponseMeta.Usage.PromptTokenDetails.CachedTokens {
					ret.ResponseMeta.Usage.PromptTokenDetails.CachedTokens = msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens
				}

				if msg.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens > ret.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens {
					ret.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens = msg.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens
				}
			}
		}

		// 把每个 msg 里带的 logprobs 内容 拼接起来
		if msg.ResponseMeta.LogProbs != nil {
			if ret.ResponseMeta.LogProbs == nil {
				ret.ResponseMeta.LogProbs = &LogProbs{}
			}

			ret.ResponseMeta.LogProbs.Content = append(ret.ResponseMeta.LogProbs.Content, msg.ResponseMeta.LogProbs.Content...)
		}
	}

	// 开始拼接成一段话
	if len(contents) > 0 {
		var sb strings.Builder
		sb.Grow(contentLen)
		for _, content := range contents {
			_, err := sb.WriteString(content)
			if err != nil {
				return nil, err
			}
		}

		ret.Content = sb.String()
	}

	if len(reasoningContents) > 0 {
		var sb strings.Builder
		sb.Grow(reasoningContentLen)
		for _, rc := range reasoningContents {
			_, err := sb.WriteString(rc)
			if err != nil {
				return nil, err
			}
		}

		ret.ReasoningContent = sb.String()
	}

	// toolCall 可能有分片的，需要进行合并
	if len(toolCalls) > 0 {
		merged, err := concatToolCalls(toolCalls)
		if err != nil {
			return nil, err
		}
		ret.ToolCalls = merged
	}

	// 把 extraList 进行合并
	if len(extraList) > 0 {
		extra, err := concatExtra(extraList)
		if err != nil {
			return nil, fmt.Errorf("failed to concat message's extra: %w", err)
		}

		if len(extra) > 0 {
			ret.Extra = extra
		}
	}

	if len(multiContentParts) > 0 {
		ret.MultiContent = multiContentParts
	}

	// 多模态分片必须按照规则拼接，而不是简单像 content 一样拼接
	if len(assistantGenMultiContentParts) > 0 {
		merged, err := concatAssistantMultiContent(assistantGenMultiContentParts)
		if err != nil {
			return nil, fmt.Errorf("failed to concat message's assistant multicontent: %w", err)
		}
		ret.AssistantGenMultiContent = merged
	}

	return &ret, nil
}

func concatAssistantMultiContent(parts []MessageOutputPart) ([]MessageOutputPart, error) {
	if len(parts) == 0 {
		return parts, nil
	}

	groups := groupOutputParts(parts)

	merged := make([]MessageOutputPart, 0, len(groups))
	for _, group := range groups {
		mergedPart, err := mergeOutputPartGroup(group)
		if err != nil {
			return nil, err
		}
		merged = append(merged, mergedPart)
	}

	return merged, nil
}

func mergeOutputPartGroup(group []MessageOutputPart) (MessageOutputPart, error) {
	if len(group) == 0 {
		return MessageOutputPart{}, nil
	}

	if len(group) == 1 {
		return group[0], nil
	}

	first := group[0]
	switch first.Type {
	case ChatMessagePartTypeText:
		return mergeTextParts(group)
	case ChatMessagePartTypeReasoning:
		return mergeReasoningParts(group)
	case ChatMessagePartTypeAudioURL:
		if isBase64MessageOutputAudioPart(first) {
			return mergeAudioParts(group)
		}
	}

	return first, nil
}

// 内容，extra 合并，StreamingMeta 使用第一个
func mergeTextParts(group []MessageOutputPart) (MessageOutputPart, error) {
	var sb strings.Builder
	extraList := make([]map[string]any, 0, len(group))
	// text 追加写，extra 拼接合并
	for _, part := range group {
		sb.WriteString(part.Text)
		if len(part.Extra) > 0 {
			extraList = append(extraList, part.Extra)
		}
	}

	var mergedExtra map[string]any
	if len(extraList) > 0 {
		var err error
		mergedExtra, err = concatExtra(extraList)
		if err != nil {
			return MessageOutputPart{}, fmt.Errorf("failed to concat text part extra: %w", err)
		}
	}

	return MessageOutputPart{
		Type:  ChatMessagePartTypeText,
		Text:  sb.String(),
		Extra: mergedExtra,
		// 直接使用第一个的 StreamingMeta 作为最终的合并 Meta
		StreamingMeta: group[0].StreamingMeta,
	}, nil
}

// 流程基本和 text 类型的一样
func mergeReasoningParts(group []MessageOutputPart) (MessageOutputPart, error) {
	var textBuilder strings.Builder
	var signature string

	extraList := make([]map[string]any, 0, len(group))
	for _, part := range group {
		if part.Reasoning != nil {
			textBuilder.WriteString(part.Reasoning.Text)
			// Signature 不拼接，而是 取遍历过程中最后一个非空的 Signature，覆盖之前保存的 signature
			if part.Reasoning.Signature != "" {
				signature = part.Reasoning.Signature
			}
		}

		if len(part.Extra) > 0 {
			extraList = append(extraList, part.Extra)
		}
	}

	var mergedExtra map[string]any
	if len(extraList) > 0 {
		var err error
		mergedExtra, err = concatExtra(extraList)
		if err != nil {
			return MessageOutputPart{}, fmt.Errorf("failed to concat reasoning part extra: %w", err)
		}
	}

	return MessageOutputPart{
		Type: ChatMessagePartTypeReasoning,
		Reasoning: &MessageOutputReasoning{
			Text:      textBuilder.String(),
			Signature: signature,
		},
		Extra:         mergedExtra,
		StreamingMeta: group[0].StreamingMeta,
	}, nil
}

// 把不同部分的 audio 内部的 base64 编码依次拼接在一起
func mergeAudioParts(group []MessageOutputPart) (MessageOutputPart, error) {
	var b64Builder strings.Builder
	var mimeType string

	audioExtraList := make([]map[string]any, 0, len(group))
	partExtraList := make([]map[string]any, 0, len(group))

	// 收集阶段
	for _, part := range group {
		audioPart := part.Audio
		if audioPart.Base64Data != nil {
			b64Builder.WriteString(*audioPart.Base64Data)
		}
		if mimeType == "" {
			mimeType = audioPart.MIMEType
		}
		if len(audioPart.Extra) > 0 {
			audioExtraList = append(audioExtraList, audioPart.Extra)
		}
		if len(part.Extra) > 0 {
			partExtraList = append(partExtraList, part.Extra)
		}
	}

	// 拼接阶段
	var mergedAudioExtra map[string]any
	var err error
	if len(audioExtraList) > 0 {
		mergedAudioExtra, err = concatExtra(audioExtraList)
		if err != nil {
			return MessageOutputPart{}, fmt.Errorf("failed to concat audio extra: %w", err)
		}
	}

	var mergedPartExtra map[string]any
	if len(partExtraList) > 0 {
		mergedPartExtra, err = concatExtra(partExtraList)
		if err != nil {
			return MessageOutputPart{}, fmt.Errorf("failed to concat audio part extra: %w", err)
		}
	}

	mergedB64 := b64Builder.String()
	return MessageOutputPart{
		Type: ChatMessagePartTypeAudioURL,
		Audio: &MessageOutputAudio{
			MessagePartCommon: MessagePartCommon{
				Base64Data: &mergedB64,
				MIMEType:   mimeType,
				Extra:      mergedAudioExtra,
			},
		},
		Extra:         mergedPartExtra,
		StreamingMeta: group[0].StreamingMeta,
	}, nil

}

// 对 MessageOutputPart 进行分组
// parts 的顺序已经是“最终要表达的输出顺序”，也就是一个有序的序列，这个函数只做“ 相邻可合并段的压缩 ”，不做“全局重组”
func groupOutputParts(parts []MessageOutputPart) [][]MessageOutputPart {
	if len(parts) == 0 {
		return nil
	}

	groups := make([][]MessageOutputPart, 0)
	currentGroup := []MessageOutputPart{parts[0]}

	for i := 1; i < len(parts); i++ {
		// 拿“当前组第一个元素”比较，也就是说一组里的所有元素，都要和组头满足同一套合并条件
		if canMergeOutputParts(currentGroup[0], parts[i]) {
			currentGroup = append(currentGroup, parts[i])
		} else {
			groups = append(groups, currentGroup)
			currentGroup = []MessageOutputPart{parts[i]}
		}
	}

	groups = append(groups, currentGroup)

	return groups
}

// 判断两个 Message 是否可以合并
func canMergeOutputParts(current, next MessageOutputPart) bool {
	if current.Type != next.Type {
		return false
	}

	if !isMergeableOutputPartType(current) {
		return false
	}

	// 如果两个 part 都带了 StreamingMeta，只有当它们的 Index 相同，才认为它们属于同一条流，可以合并
	if current.StreamingMeta != nil && next.StreamingMeta != nil {
		return current.StreamingMeta.Index == next.StreamingMeta.Index
	}

	// 否则，只有当两边都没有 StreamingMeta ，才允许合并
	return current.StreamingMeta == nil && next.StreamingMeta == nil
}

// 判断 message 是否可以进行合并
// 流式输出时，一个完整内容可能被拆成多个 chunk，不是所有 chunk 都能直接拼起来
func isMergeableOutputPartType(part MessageOutputPart) bool {
	switch part.Type {
	// text 和 reasoning 可以合并
	case ChatMessagePartTypeText, ChatMessagePartTypeReasoning:
		return true
	// AudioURL 类型：只有当它其实是“Base64 内联音频”时才可合并
	case ChatMessagePartTypeAudioURL:
		return isBase64MessageOutputAudioPart(part)
	default:
		return false
	}
}

// 判断一个 MessageOutputPart 是否表示“ 以内联 Base64 形式携带的音频片段
func isBase64MessageOutputAudioPart(part MessageOutputPart) bool {
	return part.Type == ChatMessagePartTypeAudioURL &&
		part.Audio != nil &&
		part.Audio.Base64Data != nil &&
		part.Audio.URL == nil
}

func concatToolCalls(chunks []ToolCall) ([]ToolCall, error) {
	var merged []ToolCall
	m := make(map[int][]int)
	for i := range chunks {
		index := chunks[i].Index
		if index == nil {
			// 把不需要合并的 ToolCall 直接放进结果集 merged
			merged = append(merged, chunks[i])
		} else {
			// 把需要合并的 ToolCall 按 Index 分组，先记到 m 里
			// 比如一个 JSON 参数对象比较大，上游会分多段吐出来
			// 同一个 index 的 chunk 会被当成同一个调用的不同片段
			m[*index] = append(m[*index], i)
		}
	}

	var args strings.Builder
	// 把需要合并的 chunk 进行合并
	for k, v := range m {
		index := k
		toolCall := ToolCall{Index: &index}
		if len(v) > 0 {
			toolCall = chunks[v[0]]
		}

		args.Reset()

		toolID, toolType, toolName := "", "", ""

		for _, n := range v {
			chunk := chunks[n]
			// 如果之前已经记录过，但这次的 ID 不一样，直接报错
			if chunk.ID != "" {
				if toolID == "" {
					toolID = chunk.ID
				} else if toolID != chunk.ID {
					return nil, fmt.Errorf("cannot concat ToolCalls with different tool id: '%s' '%s'", toolID, chunk.ID)
				}
			}

			if chunk.Function.Name != "" {
				if toolName == "" {
					toolName = chunk.Function.Name
				} else if toolName != chunk.Function.Name {
					return nil, fmt.Errorf("cannot concat ToolCalls with different tool name: '%s' '%s'", toolName, chunk.Function.Name)
				}
			}

			// 只要不为空，就追加到 args（stringBuilder 追加写）
			if chunk.Function.Arguments != "" {
				_, err := args.WriteString(chunk.Function.Arguments)
				if err != nil {
					return nil, err
				}
			}
		}

		toolCall.ID = toolID
		toolCall.Type = toolType
		toolCall.Function.Name = toolName
		toolCall.Function.Arguments = args.String()

		merged = append(merged, toolCall)
	}

	if len(merged) > 1 {
		// 按照 index 从小到大排
		sort.SliceStable(merged, func(i, j int) bool {
			iVal, jVal := merged[i].Index, merged[j].Index
			if iVal == nil && jVal == nil {
				return false
			} else if iVal == nil && jVal != nil {
				return true
			} else if iVal != nil && jVal == nil {
				return false
			}

			return *iVal < *jVal
		})
	}

	return merged, nil
}

// 把 extraList 合并成一个 map[string]any
func concatExtra(extraList []map[string]any) (map[string]any, error) {
	if len(extraList) == 1 {
		return generic.CopyMap(extraList[0]), nil
	}
	return internal.ConcatItems(extraList)
}

type ToolResult struct {
	Parts []ToolOutputPart `json:"parts,omitempty"`
}

func (tr *ToolResult) ToMessageInputParts() ([]MessageInputPart, error) {
	if tr == nil || len(tr.Parts) == 0 {
		return nil, nil
	}

	result := make([]MessageInputPart, len(tr.Parts))
	for i, part := range tr.Parts {
		var err error
		result[i], err = convToolOutputPartToMessageInputPart(part)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// 把工具的执行结果转换成模型消息输入里的一个 MessageInputPart
func convToolOutputPartToMessageInputPart(toolPart ToolOutputPart) (MessageInputPart, error) {
	switch toolPart.Type {
	case ToolPartTypeText:
		return MessageInputPart{
			Type:  ChatMessagePartTypeText,
			Text:  toolPart.Text,
			Extra: toolPart.Extra,
		}, nil
	case ToolPartTypeImage:
		if toolPart.Image == nil {
			return MessageInputPart{}, fmt.Errorf("image content is nil for tool part type %v", toolPart.Type)
		}
		return MessageInputPart{
			Type: ChatMessagePartTypeImageURL,
			Image: &MessageInputImage{
				MessagePartCommon: toolPart.Image.MessagePartCommon,
			},
			Extra: toolPart.Extra,
		}, nil
	case ToolPartTypeAudio:
		if toolPart.Audio == nil {
			return MessageInputPart{}, fmt.Errorf("audio content is nil for tool part type %v", toolPart.Type)
		}
		return MessageInputPart{
			Type:  ChatMessagePartTypeAudioURL,
			Audio: &MessageInputAudio{MessagePartCommon: toolPart.Audio.MessagePartCommon},
			Extra: toolPart.Extra,
		}, nil
	case ToolPartTypeVideo:
		if toolPart.Video == nil {
			return MessageInputPart{}, fmt.Errorf("video content is nil  for tool part type %v", toolPart.Type)
		}
		return MessageInputPart{
			Type:  ChatMessagePartTypeVideoURL,
			Video: &MessageInputVideo{MessagePartCommon: toolPart.Video.MessagePartCommon},
			Extra: toolPart.Extra,
		}, nil
	case ToolPartTypeFile:
		if toolPart.File == nil {
			return MessageInputPart{}, fmt.Errorf("file content is nil for tool part type %v", toolPart.Type)
		}
		return MessageInputPart{
			Type:  ChatMessagePartTypeFileURL,
			File:  &MessageInputFile{MessagePartCommon: toolPart.File.MessagePartCommon},
			Extra: toolPart.Extra,
		}, nil
	default:
		return MessageInputPart{}, fmt.Errorf("unknown tool part type: %v", toolPart.Type)
	}
}
