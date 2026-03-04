package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

var DefaultBaseURLs = map[model.ProviderType]string{
	model.ProviderOpenAI:     "https://api.openai.com/v1",
	model.ProviderQwen:       "https://dashscope.aliyuncs.com/compatible-mode/v1",
	model.ProviderKimi:       "https://api.moonshot.cn/v1",
	model.ProviderOpenRouter: "https://openrouter.ai/api/v1",
	model.ProviderNewAPI:     "",
}

type adapter struct {
	llm llms.Model
}

func (a *adapter) GetModel() llms.Model {
	return a.llm
}

func (a *adapter) GenerateContent(ctx context.Context, messages []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error) {
	return a.llm.GenerateContent(ctx, messages, opts...)
}

func (a *adapter) StreamContent(ctx context.Context, messages []llms.MessageContent, handler func(ctx context.Context, chunk []byte) error, opts ...llms.CallOption) (*llms.ContentResponse, error) {
	streamFn := func(_ context.Context, chunk []byte) error {
		return handler(ctx, chunk)
	}
	opts = append(opts, llms.WithStreamingFunc(streamFn))
	return a.llm.GenerateContent(ctx, messages, opts...)
}

func ResolveBaseURL(p *model.Provider) (string, error) {
	baseURL := p.BaseURL
	if baseURL == "" {
		var ok bool
		baseURL, ok = DefaultBaseURLs[p.Type]
		if !ok || baseURL == "" {
			return "", fmt.Errorf("unsupported provider type: %s", p.Type)
		}
	}
	return strings.TrimRight(baseURL, "/"), nil
}

type openAIModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func FetchRemoteModels(ctx context.Context, p *model.Provider) ([]string, error) {
	baseURL, err := ResolveBaseURL(p)
	if err != nil {
		return nil, err
	}

	modelsURL := baseURL + "/models"

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("models API returned %d: %s", resp.StatusCode, string(body))
	}

	var result openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	sort.Strings(models)
	return models, nil
}

func NewFromProvider(p *model.Provider, modelName string) (LLMProvider, error) {
	baseURL, err := ResolveBaseURL(p)
	if err != nil {
		return nil, err
	}

	opts := []openai.Option{
		openai.WithToken(p.APIKey),
		openai.WithBaseURL(baseURL),
	}
	if modelName != "" {
		opts = append(opts, openai.WithModel(modelName))
	}

	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create llm for provider %s: %w", p.Name, err)
	}
	return &adapter{llm: llm}, nil
}

var _ LLMProvider = (*adapter)(nil)
