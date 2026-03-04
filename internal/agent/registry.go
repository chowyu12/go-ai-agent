package agent

import (
	"context"
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/google/uuid"
	"github.com/tmc/langchaingo/tools"
	"golang.org/x/net/html/charset"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type BuiltinHandler func(ctx context.Context, args string) (string, error)

type ToolRegistry struct {
	builtins map[string]BuiltinHandler
}

func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{builtins: make(map[string]BuiltinHandler)}
	registerDefaults(r)
	return r
}

func (r *ToolRegistry) RegisterBuiltin(name string, handler BuiltinHandler) {
	r.builtins[name] = handler
}

func (r *ToolRegistry) BuildTrackedTools(toolDefs []model.Tool, tracker *StepTracker, toolSkillMap map[string]string) []tools.Tool {
	var result []tools.Tool
	for _, td := range toolDefs {
		if !td.Enabled {
			continue
		}
		baseTool := r.buildTool(td)
		if baseTool == nil {
			log.WithField("tool", td.Name).Warn("no handler found for tool, skipping")
			continue
		}
		result = append(result, &trackedTool{
			baseTool:  baseTool,
			name:      td.Name,
			skillName: toolSkillMap[td.Name],
			tracker:   tracker,
		})
	}
	return result
}

func (r *ToolRegistry) buildTool(td model.Tool) tools.Tool {
	switch td.HandlerType {
	case model.HandlerBuiltin:
		handler, ok := r.builtins[td.Name]
		if !ok {
			return nil
		}
		return &dynamicTool{toolName: td.Name, toolDesc: td.Description, handler: handler}
	case model.HandlerHTTP:
		var cfg model.HTTPHandlerConfig
		if json.Unmarshal(td.HandlerConfig, &cfg) != nil {
			return nil
		}
		timeout := td.TimeoutSeconds()
		return &dynamicTool{
			toolName: td.Name,
			toolDesc: td.Description,
			handler: func(ctx context.Context, input string) (string, error) {
				return httpToolHandler(ctx, cfg, timeout, input)
			},
		}
	case model.HandlerCommand:
		var cfg model.CommandHandlerConfig
		if json.Unmarshal(td.HandlerConfig, &cfg) != nil {
			return nil
		}
		timeout := td.TimeoutSeconds()
		return &dynamicTool{
			toolName: td.Name,
			toolDesc: td.Description,
			handler: func(ctx context.Context, input string) (string, error) {
				return commandToolHandler(ctx, cfg, timeout, input)
			},
		}
	default:
		return nil
	}
}

type trackedTool struct {
	baseTool  tools.Tool
	name      string
	skillName string
	tracker   *StepTracker
}

func (t *trackedTool) Name() string        { return t.baseTool.Name() }
func (t *trackedTool) Description() string { return t.baseTool.Description() }
func (t *trackedTool) Call(ctx context.Context, input string) (string, error) {
	l := log.WithField("tool", t.name)
	if t.skillName != "" {
		l = l.WithField("skill", t.skillName)
	}
	l.WithField("input", truncateLog(input, 200)).Debug("[Tool]    invoke args")

	start := time.Now()
	output, err := t.baseTool.Call(ctx, input)
	duration := time.Since(start)

	status := model.StepSuccess
	errMsg := ""
	if err != nil {
		status = model.StepError
		errMsg = err.Error()
	}

	t.tracker.RecordStep(ctx, model.StepToolCall, t.name, input, output, status, errMsg, duration, 0, &model.StepMetadata{
		ToolName:  t.name,
		SkillName: t.skillName,
	})
	return output, err
}

var _ tools.Tool = (*trackedTool)(nil)

type dynamicTool struct {
	toolName string
	toolDesc string
	handler  func(ctx context.Context, input string) (string, error)
}

func (t *dynamicTool) Name() string                                         { return t.toolName }
func (t *dynamicTool) Description() string                                  { return t.toolDesc }
func (t *dynamicTool) Call(ctx context.Context, input string) (string, error) { return t.handler(ctx, input) }

var _ tools.Tool = (*dynamicTool)(nil)

func httpToolHandler(ctx context.Context, cfg model.HTTPHandlerConfig, timeoutSec int, input string) (string, error) {
	urlStr := cfg.URL
	method := cfg.Method
	if method == "" {
		method = http.MethodGet
	}

	var params map[string]any
	if input != "" {
		json.Unmarshal([]byte(input), &params)
	}
	for key, val := range params {
		urlStr = strings.ReplaceAll(urlStr, "{"+key+"}", fmt.Sprint(val))
	}

	var body io.Reader
	if cfg.Body != "" {
		bodyStr := cfg.Body
		for key, val := range params {
			bodyStr = strings.ReplaceAll(bodyStr, "{"+key+"}", fmt.Sprint(val))
		}
		body = strings.NewReader(bodyStr)
	}

	timeout := time.Duration(timeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	reader, err := charset.NewReader(resp.Body, contentType)
	if err != nil {
		reader = resp.Body
	}

	respBody, err := io.ReadAll(io.LimitReader(reader, 10_000))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	return string(respBody), nil
}

var dangerousPatterns = []string{
	"rm -rf /",
	"rm -rf /*",
	"rm -rf ~",
	"mkfs",
	"dd if=",
	":(){:|:&};:",
	"> /dev/sda",
	"chmod -R 777 /",
	"chown -R",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"init 0",
	"init 6",
	"kill -9 1",
	"killall",
	"pkill",
	"ssh-keygen",
	"ssh ",
	"scp ",
	"sftp ",
	"telnet ",
	"nc -l",
	"ncat -l",
	"curl.*|.*sh",
	"wget.*|.*sh",
	"useradd",
	"userdel",
	"usermod",
	"passwd",
	"visudo",
	"iptables -F",
	"iptables -X",
	"nft flush",
	"crontab -r",
	"systemctl disable",
	"service.*stop",
	"eval ",
	"exec ",
	"nohup ",
	"> /etc/",
	"tee /etc/",
	"mount ",
	"umount ",
	"fdisk ",
	"parted ",
	"wipefs",
}

func checkDangerousCommand(cmdStr string) error {
	lower := strings.ToLower(strings.TrimSpace(cmdStr))
	for _, p := range dangerousPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return fmt.Errorf("dangerous command blocked: contains '%s'", p)
		}
	}
	for _, seg := range strings.Split(lower, "|") {
		seg = strings.TrimSpace(seg)
		if strings.HasPrefix(seg, "rm ") && (strings.Contains(seg, " -r") || strings.Contains(seg, " -f")) {
			return fmt.Errorf("dangerous command blocked: recursive/force rm is not allowed")
		}
	}
	return nil
}

func commandToolHandler(ctx context.Context, cfg model.CommandHandlerConfig, timeoutSec int, input string) (string, error) {
	cmdStr := cfg.Command

	var params map[string]any
	if input != "" {
		json.Unmarshal([]byte(input), &params)
	}
	for key, val := range params {
		cmdStr = strings.ReplaceAll(cmdStr, "{"+key+"}", fmt.Sprint(val))
	}

	if err := checkDangerousCommand(cmdStr); err != nil {
		log.WithFields(log.Fields{"command": cmdStr, "reason": err}).Warn("[Tool] !! command blocked by safety check")
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	shell := cfg.Shell
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.CommandContext(ctx, shell, "-c", cmdStr)
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.WithFields(log.Fields{"command": cmdStr, "shell": shell, "timeout": timeoutSec}).Info("[Tool] >> exec command")
	err := cmd.Run()

	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\n[stderr]\n" + stderr.String()
	}

	const maxOutput = 10_000
	if len(result) > maxOutput {
		result = result[:maxOutput] + "\n... (output truncated)"
	}

	if err != nil {
		log.WithFields(log.Fields{"command": cmdStr, "error": err}).Warn("[Tool] << command failed")
		return result, fmt.Errorf("command failed: %w", err)
	}

	log.WithField("command", cmdStr).Info("[Tool] << command ok")
	return result, nil
}

func extractJSONField(jsonStr, field string) string {
	var m map[string]any
	if json.Unmarshal([]byte(jsonStr), &m) != nil {
		return jsonStr
	}
	v, ok := m[field]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}

func registerDefaults(r *ToolRegistry) {
	r.RegisterBuiltin("current_time", func(_ context.Context, _ string) (string, error) {
		return time.Now().Format(time.RFC3339), nil
	})

	r.RegisterBuiltin("uuid_generator", func(_ context.Context, _ string) (string, error) {
		return uuid.New().String(), nil
	})

	r.RegisterBuiltin("calculator", func(_ context.Context, args string) (string, error) {
		expr := extractJSONField(args, "expression")
		if expr == "" {
			expr = args
		}
		return fmt.Sprintf("计算表达式: %s (计算器功能简化版)", expr), nil
	})

	r.RegisterBuiltin("base64_encode", func(_ context.Context, args string) (string, error) {
		text := extractJSONField(args, "text")
		if text == "" {
			text = args
		}
		return base64.StdEncoding.EncodeToString([]byte(text)), nil
	})

	r.RegisterBuiltin("base64_decode", func(_ context.Context, args string) (string, error) {
		text := extractJSONField(args, "text")
		if text == "" {
			text = args
		}
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return "", fmt.Errorf("base64 decode: %w", err)
		}
		return string(decoded), nil
	})

	r.RegisterBuiltin("json_formatter", func(_ context.Context, args string) (string, error) {
		jsonStr := extractJSONField(args, "json_string")
		if jsonStr == "" {
			jsonStr = args
		}
		var v any
		if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
			return "", fmt.Errorf("invalid JSON: %w", err)
		}
		formatted, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", err
		}
		return string(formatted), nil
	})

	r.RegisterBuiltin("hash_text", func(_ context.Context, args string) (string, error) {
		text := extractJSONField(args, "text")
		algo := extractJSONField(args, "algorithm")
		if algo == "" {
			algo = "sha256"
		}
		switch algo {
		case "md5":
			return fmt.Sprintf("%x", md5.Sum([]byte(text))), nil
		case "sha1":
			return fmt.Sprintf("%x", sha1.Sum([]byte(text))), nil
		case "sha256":
			return fmt.Sprintf("%x", sha256.Sum256([]byte(text))), nil
		default:
			return "", fmt.Errorf("unsupported algorithm: %s", algo)
		}
	})

	r.RegisterBuiltin("random_number", func(_ context.Context, args string) (string, error) {
		minVal := 1
		maxVal := 100
		var m map[string]any
		if json.Unmarshal([]byte(args), &m) == nil {
			if v, ok := m["min"].(float64); ok {
				minVal = int(v)
			}
			if v, ok := m["max"].(float64); ok {
				maxVal = int(v)
			}
		}
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		return fmt.Sprintf("%d", minVal+rand.IntN(maxVal-minVal+1)), nil
	})
}
