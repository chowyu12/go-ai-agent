package agent

// 新增 builtin 工具步骤：
//   1. 在本文件添加 handler 函数 (签名: func(ctx, args string) (string, error))
//   2. 在 registerDefaults 中注册
//   3. 在 internal/seed/seed.go defaultTools() 添加种子数据

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/google/uuid"
)

func registerDefaults(r *ToolRegistry) {
	r.RegisterBuiltin("current_time", builtinCurrentTime)
	r.RegisterBuiltin("uuid_generator", builtinUUIDGenerator)
	r.RegisterBuiltin("calculator", builtinCalculator)
	r.RegisterBuiltin("base64_encode", builtinBase64Encode)
	r.RegisterBuiltin("base64_decode", builtinBase64Decode)
	r.RegisterBuiltin("json_formatter", builtinJSONFormatter)
	r.RegisterBuiltin("hash_text", builtinHashText)
	r.RegisterBuiltin("random_number", builtinRandomNumber)
	r.RegisterBuiltin("url_reader", builtinURLReader)
	r.RegisterBuiltin("webpage_screenshot", builtinWebpageScreenshot)
}

func builtinCurrentTime(_ context.Context, _ string) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

func builtinUUIDGenerator(_ context.Context, _ string) (string, error) {
	return uuid.New().String(), nil
}

func builtinCalculator(_ context.Context, args string) (string, error) {
	expr := extractJSONField(args, "expression")
	if expr == "" {
		expr = args
	}
	return fmt.Sprintf("计算表达式: %s (计算器功能简化版)", expr), nil
}

func builtinBase64Encode(_ context.Context, args string) (string, error) {
	text := extractJSONField(args, "text")
	if text == "" {
		text = args
	}
	return base64.StdEncoding.EncodeToString([]byte(text)), nil
}

func builtinBase64Decode(_ context.Context, args string) (string, error) {
	text := extractJSONField(args, "text")
	if text == "" {
		text = args
	}
	decoded, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	return string(decoded), nil
}

func builtinJSONFormatter(_ context.Context, args string) (string, error) {
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
}

func builtinHashText(_ context.Context, args string) (string, error) {
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
}

func builtinRandomNumber(_ context.Context, args string) (string, error) {
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
}

func builtinURLReader(ctx context.Context, args string) (string, error) {
	targetURL := extractJSONField(args, "url")
	if targetURL == "" {
		return "", fmt.Errorf("url is required")
	}

	content, httpErr := fetchURL(ctx, targetURL)
	if httpErr == nil {
		if !looksLikeHTML(content) {
			return content, nil
		}
		text, err := webpageToText(targetURL, 30*time.Second)
		if err == nil {
			return text, nil
		}
		log.WithFields(log.Fields{"url": targetURL, "error": err}).Warn("[url_reader] chromedp render failed, using raw HTTP content")
		return content, nil
	}

	log.WithFields(log.Fields{"url": targetURL, "http_error": httpErr}).Info("[url_reader] HTTP failed, trying chromedp")
	text, err := webpageToText(targetURL, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("http: %v; chromedp: %w", httpErr, err)
	}
	return text, nil
}

func looksLikeHTML(content string) bool {
	head := strings.ToLower(content[:min(len(content), 500)])
	return strings.Contains(head, "<!doctype html") || strings.Contains(head, "<html")
}

func builtinWebpageScreenshot(_ context.Context, args string) (string, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	url, _ := m["url"].(string)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	opt := &ScreenshotOption{}
	if w, ok := m["width"].(float64); ok && w > 0 {
		opt.Width = int(w)
	}
	if h, ok := m["height"].(float64); ok && h > 0 {
		opt.Height = int(h)
	}
	if fp, ok := m["full_page"].(bool); ok {
		opt.FullPage = fp
	}

	path, err := webpageToImage(url, opt)
	if err != nil {
		return "", err
	}
	return newFileResult(path, "image/png", fmt.Sprintf("Screenshot of %s", url)), nil
}

type toolFileResult struct {
	Type        string `json:"__type"`
	Path        string `json:"path"`
	MimeType    string `json:"mime"`
	Description string `json:"description"`
}

func newFileResult(filePath, mimeType, description string) string {
	data, _ := json.Marshal(toolFileResult{
		Type:        "file",
		Path:        filePath,
		MimeType:    mimeType,
		Description: description,
	})
	return string(data)
}

func parseFileResult(output string) *toolFileResult {
	var r toolFileResult
	if json.Unmarshal([]byte(output), &r) == nil && r.Type == "file" && r.Path != "" {
		return &r
	}
	if fr := detectFilePath(output); fr != nil {
		return fr
	}
	return nil
}

func detectFilePath(output string) *toolFileResult {
	p := strings.TrimSpace(output)
	if p == "" || strings.Contains(p, "\n") || len(p) > 500 {
		return nil
	}
	if !filepath.IsAbs(p) {
		return nil
	}
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		return nil
	}
	return &toolFileResult{
		Type:        "file",
		Path:        p,
		MimeType:    mimeFromExt(filepath.Ext(p)),
		Description: fmt.Sprintf("File: %s", filepath.Base(p)),
	}
}

func mimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".txt", ".log":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".html", ".htm":
		return "text/html"
	case ".xml":
		return "application/xml"
	case ".md":
		return "text/markdown"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
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
