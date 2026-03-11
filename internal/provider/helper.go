package provider

import (
	"context"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// Complete 发起一次非流式 LLM 调用，返回文本内容。
// 供 brain/、memory/ 等模块共用，避免各处重复构建请求。
func Complete(ctx context.Context, llm LLMProvider, model, system, user string) (string, error) {
	resp, err := llm.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}

// ExtractJSON 从可能包含 markdown 围栏的 LLM 输出中提取 JSON 对象或数组。
func ExtractJSON(s string) string {
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	}
	s = strings.TrimSpace(s)

	objIdx := strings.Index(s, "{")
	arrIdx := strings.Index(s, "[")

	var open, close byte
	var start int
	switch {
	case objIdx >= 0 && (arrIdx < 0 || objIdx < arrIdx):
		open, close, start = '{', '}', objIdx
	case arrIdx >= 0:
		open, close, start = '[', ']', arrIdx
	default:
		return s
	}

	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		if esc {
			esc = false
			continue
		}
		c := s[i]
		switch {
		case c == '\\' && inStr:
			esc = true
		case c == '"':
			inStr = !inStr
		case !inStr && c == open:
			depth++
		case !inStr && c == close:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}
