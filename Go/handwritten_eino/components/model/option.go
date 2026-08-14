package model

import "github.com/cloudwego/eino/schema"

type Options struct {
	Temperature      *float32
	MaxTokens        *int
	Model            *string
	TopP             *float32
	Stop             []string
	Tools            []*schema.ToolInfo
	ToolChoice       *schema.ToolChoice
	AllowedToolNames []string
}

type Option struct {
	apply func(opt *Options)

	implSpecificOptFn any
}
