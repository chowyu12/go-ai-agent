package agent

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/tools"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type BuiltinHandler func(ctx context.Context, input string) (string, error)

type ToolRegistry struct {
	mu       sync.RWMutex
	builtins map[string]BuiltinHandler
}

func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{
		builtins: make(map[string]BuiltinHandler),
	}
	r.registerDefaults()
	return r
}

func (r *ToolRegistry) RegisterBuiltin(name string, handler BuiltinHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtins[name] = handler
}

func (r *ToolRegistry) BuildLangChainTools(toolDefs []model.Tool) []tools.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []tools.Tool
	for _, td := range toolDefs {
		if !td.Enabled {
			continue
		}
		result = append(result, r.buildTool(td))
	}
	return result
}

func (r *ToolRegistry) BuildTrackedTools(toolDefs []model.Tool, tracker *StepTracker) []tools.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []tools.Tool
	for _, td := range toolDefs {
		if !td.Enabled {
			continue
		}
		baseTool := r.buildTool(td)
		result = append(result, &trackedTool{
			inner:   baseTool,
			tracker: tracker,
		})
	}
	return result
}

func (r *ToolRegistry) buildTool(td model.Tool) tools.Tool {
	switch td.HandlerType {
	case model.HandlerHTTP:
		return &dynamicTool{
			name:        td.Name,
			description: td.Description,
			handler:     r.httpHandler(td.HandlerConfig),
		}
	case model.HandlerBuiltin:
		if h, ok := r.builtins[td.Name]; ok {
			return &dynamicTool{
				name:        td.Name,
				description: td.Description,
				handler:     h,
			}
		}
		return &dynamicTool{
			name:        td.Name,
			description: td.Description,
			handler: func(_ context.Context, input string) (string, error) {
				return fmt.Sprintf("builtin tool %q not registered", td.Name), nil
			},
		}
	default:
		return &dynamicTool{
			name:        td.Name,
			description: td.Description,
			handler: func(_ context.Context, input string) (string, error) {
				return fmt.Sprintf("unsupported handler type: %s", td.HandlerType), nil
			},
		}
	}
}

func (r *ToolRegistry) httpHandler(cfgRaw json.RawMessage) BuiltinHandler {
	var cfg model.HTTPHandlerConfig
	json.Unmarshal(cfgRaw, &cfg)

	return func(ctx context.Context, input string) (string, error) {
		method := strings.ToUpper(cfg.Method)
		if method == "" {
			method = http.MethodPost
		}
		body := cfg.Body
		if body == "" {
			body = input
		}
		req, err := http.NewRequestWithContext(ctx, method, cfg.URL, strings.NewReader(body))
		if err != nil {
			return "", err
		}
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(respBody), nil
	}
}

func (r *ToolRegistry) registerDefaults() {
	r.RegisterBuiltin("current_time", func(_ context.Context, _ string) (string, error) {
		return time.Now().Format(time.RFC3339), nil
	})

	r.RegisterBuiltin("calculator", func(_ context.Context, input string) (string, error) {
		return fmt.Sprintf("Calculator received: %s (implement evaluation logic as needed)", input), nil
	})

	r.RegisterBuiltin("uuid_generator", func(_ context.Context, _ string) (string, error) {
		return uuid.New().String(), nil
	})

	r.RegisterBuiltin("base64_encode", func(_ context.Context, input string) (string, error) {
		text := extractJSONField(input, "text")
		return base64.StdEncoding.EncodeToString([]byte(text)), nil
	})

	r.RegisterBuiltin("base64_decode", func(_ context.Context, input string) (string, error) {
		text := extractJSONField(input, "text")
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return "", fmt.Errorf("base64 decode failed: %w", err)
		}
		return string(decoded), nil
	})

	r.RegisterBuiltin("json_formatter", func(_ context.Context, input string) (string, error) {
		raw := extractJSONField(input, "json_string")
		if raw == "" {
			raw = input
		}
		var obj any
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			return "", fmt.Errorf("invalid JSON: %w", err)
		}
		formatted, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return "", err
		}
		return string(formatted), nil
	})

	r.RegisterBuiltin("hash_text", func(_ context.Context, input string) (string, error) {
		text := extractJSONField(input, "text")
		algo := extractJSONField(input, "algorithm")
		if algo == "" {
			algo = "sha256"
		}
		data := []byte(text)
		switch algo {
		case "md5":
			h := md5.Sum(data)
			return fmt.Sprintf("%x", h), nil
		case "sha1":
			h := sha1.Sum(data)
			return fmt.Sprintf("%x", h), nil
		case "sha256":
			h := sha256.Sum256(data)
			return fmt.Sprintf("%x", h), nil
		default:
			return "", fmt.Errorf("unsupported algorithm: %s (use md5, sha1, or sha256)", algo)
		}
	})

	r.RegisterBuiltin("random_number", func(_ context.Context, input string) (string, error) {
		minVal := extractJSONInt(input, "min", 1)
		maxVal := extractJSONInt(input, "max", 100)
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		n := rand.IntN(maxVal-minVal+1) + minVal
		return fmt.Sprintf("%d", n), nil
	})
}

func extractJSONField(input, field string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return input
	}
	if v, ok := m[field]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		data, _ := json.Marshal(v)
		return string(data)
	}
	return input
}

func extractJSONInt(input, field string, defaultVal int) int {
	var m map[string]any
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return defaultVal
	}
	if v, ok := m[field]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case json.Number:
			i, _ := n.Int64()
			return int(i)
		}
	}
	return defaultVal
}

type dynamicTool struct {
	name        string
	description string
	handler     BuiltinHandler
}

func (t *dynamicTool) Name() string       { return t.name }
func (t *dynamicTool) Description() string { return t.description }
func (t *dynamicTool) Call(ctx context.Context, input string) (string, error) {
	return t.handler(ctx, input)
}

type trackedTool struct {
	inner   tools.Tool
	tracker *StepTracker
}

func (t *trackedTool) Name() string       { return t.inner.Name() }
func (t *trackedTool) Description() string { return t.inner.Description() }

func (t *trackedTool) Call(ctx context.Context, input string) (string, error) {
	start := time.Now()
	output, err := t.inner.Call(ctx, input)
	duration := time.Since(start)

	status := model.StepSuccess
	errMsg := ""
	if err != nil {
		status = model.StepError
		errMsg = err.Error()
	}

	t.tracker.RecordStep(ctx, model.StepToolCall, t.inner.Name(), input, output, status, errMsg, duration, 0, &model.StepMetadata{
		ToolName: t.inner.Name(),
	})

	return output, err
}
