package provider

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

type LLMProvider interface {
	GetModel() llms.Model
	GenerateContent(ctx context.Context, messages []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error)
	StreamContent(ctx context.Context, messages []llms.MessageContent, handler func(ctx context.Context, chunk []byte) error, opts ...llms.CallOption) (*llms.ContentResponse, error)
}
