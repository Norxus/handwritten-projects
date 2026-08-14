package adk

import "github.com/cloudwego/eino/internal/core"

type ResumeInfo struct {
	EnableStreaming bool
	*InterruptInfo
	WasInterrupted bool
	InterruptState any
	IsResumeTarget bool
	ResumeData     any
}

type InterruptInfo struct {
	Data              any
	InterruptContexts []*InterruptCtx
}

type InterruptCtx = core.InterruptCtx
