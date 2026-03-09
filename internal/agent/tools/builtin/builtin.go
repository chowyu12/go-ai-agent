package builtin

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/chowyu12/go-ai-agent/internal/agent/tools/result"
	"github.com/google/uuid"
)

func Handlers() map[string]func(context.Context, string) (string, error) {
	return map[string]func(context.Context, string) (string, error){
		"current_time":   currentTime,
		"uuid_generator": uuidGenerator,
		"calculator":     calculator,
		"base64_encode":  base64Encode,
		"base64_decode":  base64Decode,
		"json_formatter": jsonFormatter,
		"hash_text":      hashText,
		"random_number":  randomNumber,
	}
}

func currentTime(_ context.Context, _ string) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

func uuidGenerator(_ context.Context, _ string) (string, error) {
	return uuid.New().String(), nil
}

func calculator(_ context.Context, args string) (string, error) {
	expr := result.ExtractJSONField(args, "expression")
	if expr == "" {
		expr = args
	}
	return fmt.Sprintf("计算表达式: %s (计算器功能简化版)", expr), nil
}

func base64Encode(_ context.Context, args string) (string, error) {
	text := result.ExtractJSONField(args, "text")
	if text == "" {
		text = args
	}
	return base64.StdEncoding.EncodeToString([]byte(text)), nil
}

func base64Decode(_ context.Context, args string) (string, error) {
	text := result.ExtractJSONField(args, "text")
	if text == "" {
		text = args
	}
	decoded, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	return string(decoded), nil
}

func jsonFormatter(_ context.Context, args string) (string, error) {
	jsonStr := result.ExtractJSONField(args, "json_string")
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

func hashText(_ context.Context, args string) (string, error) {
	text := result.ExtractJSONField(args, "text")
	algo := result.ExtractJSONField(args, "algorithm")
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

func randomNumber(_ context.Context, args string) (string, error) {
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
