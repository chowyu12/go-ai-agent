package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/parser"
)

func buildMessages(ag *model.Agent, skills []model.Skill, history []openai.ChatCompletionMessage, userMsg string, toolNames []string, files []*model.File) []openai.ChatCompletionMessage {
	systemPrompt := buildSystemPrompt(ag, skills, toolNames)

	var messages []openai.ChatCompletionMessage
	if systemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}

	messages = append(messages, history...)

	var textFiles []*model.File
	var imageFiles []*model.File
	for _, f := range files {
		if f.IsImage() && f.StoragePath != "" {
			imageFiles = append(imageFiles, f)
		} else if f.TextContent != "" {
			textFiles = append(textFiles, f)
		} else if f.StoragePath != "" {
			data, err := os.ReadFile(f.StoragePath)
			if err == nil {
				text, err := parser.ExtractText(f.ContentType, bytes.NewReader(data))
				if err == nil && text != "" {
					f.TextContent = text
					textFiles = append(textFiles, f)
					continue
				}
			}
			log.WithField("file", f.Filename).Warn("[Execute] document text extraction failed, skipping")
		}
	}

	userText := userMsg
	if len(textFiles) > 0 {
		var sb strings.Builder
		sb.WriteString("以下是用户提供的参考文件内容:\n\n")
		for _, f := range textFiles {
			sb.WriteString(fmt.Sprintf("--- [文件: %s] ---\n%s\n---\n\n", f.Filename, f.TextContent))
		}
		sb.WriteString("用户消息: ")
		sb.WriteString(userMsg)
		userText = sb.String()
	}

	if len(imageFiles) > 0 {
		multiContent := []openai.ChatMessagePart{
			{Type: openai.ChatMessagePartTypeText, Text: userText},
		}
		for _, img := range imageFiles {
			part, err := imagePartForFile(img)
			if err != nil {
				log.WithError(err).WithField("file", img.Filename).Warn("[Execute] prepare image failed, skipping")
				continue
			}
			multiContent = append(multiContent, part)
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:         openai.ChatMessageRoleUser,
			MultiContent: multiContent,
		})
	} else {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userText,
		})
	}

	return messages
}

func buildSystemPrompt(ag *model.Agent, skills []model.Skill, toolNames []string) string {
	l := log.WithField("agent", ag.Name)

	var sb strings.Builder
	if ag.SystemPrompt != "" {
		sb.WriteString(ag.SystemPrompt)
		l.WithField("len", len(ag.SystemPrompt)).Debug("[Prompt]  base prompt loaded")
	}

	for _, sk := range skills {
		if sk.Instruction == "" {
			l.WithField("skill", sk.Name).Debug("[Prompt]  skill has no instruction, skipped")
			continue
		}
		sb.WriteString("\n\n## Skill: " + sk.Name + "\n" + sk.Instruction)
		l.WithFields(log.Fields{"skill": sk.Name, "len": len(sk.Instruction)}).Debug("[Prompt]  skill instruction injected")
	}

	if len(toolNames) > 0 {
		sb.WriteString("\n\n## 工具使用策略\n")
		sb.WriteString("你拥有以下工具: " + strings.Join(toolNames, ", ") + "\n")
		sb.WriteString("请在回答问题时优先使用可用的工具来获取信息或执行操作，而不是仅依赖你的内置知识。\n")
		sb.WriteString("思考步骤：1. 分析用户问题 2. 判断哪些工具可以帮助回答 3. 调用工具 4. 综合工具结果给出最终回答。\n")
		sb.WriteString("如果问题可以通过工具获得更准确的答案，必须优先调用工具。\n")
		l.WithField("tools", toolNames).Debug("[Prompt]  tool strategy injected")
	}

	result := sb.String()
	l.WithFields(log.Fields{
		"total_len": len(result),
		"skills":    len(skills),
		"tools":     len(toolNames),
	}).Debug("[Prompt]  system prompt built")
	return result
}

func buildLLMToolDefs(modelTools []model.Tool, subAgentTools []Tool) []openai.Tool {
	var result []openai.Tool

	for _, mt := range modelTools {
		if !mt.Enabled {
			continue
		}
		fd := &openai.FunctionDefinition{
			Name:        mt.Name,
			Description: mt.Description,
		}
		if len(mt.FunctionDef) > 0 {
			var def map[string]any
			if json.Unmarshal(mt.FunctionDef, &def) == nil {
				if desc, ok := def["description"].(string); ok && desc != "" {
					fd.Description = desc
				}
				if params, ok := def["parameters"]; ok {
					fd.Parameters = params
				}
			}
		}
		if fd.Parameters == nil {
			fd.Parameters = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
		result = append(result, openai.Tool{Type: openai.ToolTypeFunction, Function: fd})
	}

	for _, t := range subAgentTools {
		result = append(result, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"input": map[string]any{
							"type":        "string",
							"description": "Task description to delegate to the sub-agent",
						},
					},
					"required": []string{"input"},
				},
			},
		})
	}

	return result
}
