package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html/charset"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

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

func fetchURL(ctx context.Context, targetURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	reader, err := charset.NewReader(resp.Body, contentType)
	if err != nil {
		reader = resp.Body
	}

	body, err := io.ReadAll(io.LimitReader(reader, 10_000))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
