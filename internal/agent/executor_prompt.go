package agent

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/parser"
)

func buildMessages(ag *model.Agent, skills []model.Skill, history []openai.ChatCompletionMessage, userMsg string, agentTools []model.Tool, subAgentTools []Tool, toolSkillMap map[string]string, files []*model.File) []openai.ChatCompletionMessage {
	systemPrompt := buildSystemPrompt(ag, skills, agentTools, subAgentTools, toolSkillMap)

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

func buildSystemPrompt(ag *model.Agent, skills []model.Skill, agentTools []model.Tool, subAgentTools []Tool, toolSkillMap map[string]string) string {
	l := log.WithField("agent", ag.Name)

	var sb strings.Builder
	if ag.SystemPrompt != "" {
		sb.WriteString(ag.SystemPrompt)
		l.WithField("len", len(ag.SystemPrompt)).Debug("[Prompt]  base prompt loaded")
	}

	var enabledTools []model.Tool
	for _, t := range agentTools {
		if t.Enabled {
			enabledTools = append(enabledTools, t)
		}
	}

	hasSkillInstructions := false
	for _, sk := range skills {
		if sk.Instruction != "" {
			hasSkillInstructions = true
			break
		}
	}
	hasTools := len(enabledTools) > 0 || len(subAgentTools) > 0

	if !hasSkillInstructions && !hasTools {
		result := sb.String()
		l.WithField("total_len", len(result)).Debug("[Prompt]  system prompt built (minimal)")
		return result
	}

	skillToolNames := make(map[string][]string)
	for _, t := range enabledTools {
		if sn, ok := toolSkillMap[t.Name]; ok {
			skillToolNames[sn] = append(skillToolNames[sn], t.Name)
		}
	}

	if hasSkillInstructions {
		sb.WriteString("\n\n## 技能\n")
		for _, sk := range skills {
			if sk.Instruction == "" {
				l.WithField("skill", sk.Name).Debug("[Prompt]  skill has no instruction, skipped")
				continue
			}
			sb.WriteString("\n### " + sk.Name + "\n")
			sb.WriteString(sk.Instruction + "\n")
			if names := skillToolNames[sk.Name]; len(names) > 0 {
				sb.WriteString("关联工具: " + strings.Join(names, ", ") + "\n")
			}
			l.WithFields(log.Fields{"skill": sk.Name, "len": len(sk.Instruction)}).Debug("[Prompt]  skill injected")
		}
	}

	if hasTools {
		sb.WriteString("\n\n## 可用工具\n\n")
		for _, t := range enabledTools {
			desc := cmp.Or(t.Description, t.Name)
			line := fmt.Sprintf("- **%s**: %s", t.Name, desc)
			if sn, ok := toolSkillMap[t.Name]; ok {
				line += fmt.Sprintf(" (技能: %s)", sn)
			}
			sb.WriteString(line + "\n")
		}
		for _, t := range subAgentTools {
			desc := cmp.Or(t.Description(), "委托子Agent执行")
			sb.WriteString(fmt.Sprintf("- **%s**: %s (子Agent)\n", t.Name(), desc))
		}

		strategies := []string{
			"**意图识别**: 分析用户问题，判断是否与已有技能或工具的能力匹配",
			"**工具优先**: 当问题可通过工具获得更准确或实时的结果时，必须调用工具，禁止仅凭内置知识推测",
		}
		if hasSkillInstructions {
			strategies = append(strategies, "**技能路由**: 若问题匹配某项技能，优先使用该技能及其关联工具")
		}
		strategies = append(strategies,
			"**组合调用**: 复杂问题可串联或并行调用多个工具",
			"**结果驱动**: 基于工具返回的真实数据生成回答，不编造或臆测信息",
		)

		sb.WriteString("\n## 工具使用策略\n\n")
		for i, s := range strategies {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}

		l.WithFields(log.Fields{
			"agent_tools": len(enabledTools),
			"sub_agents":  len(subAgentTools),
		}).Debug("[Prompt]  tool catalog injected")
	}

	result := sb.String()
	l.WithFields(log.Fields{
		"total_len": len(result),
		"skills":    len(skills),
		"tools":     len(enabledTools) + len(subAgentTools),
	}).Debug("[Prompt]  system prompt built")
	return result
}

func buildLLMToolDefs(modelTools []model.Tool, subAgentTools []Tool, mcpTools []Tool, skillTools []Tool) []openai.Tool {
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

	for _, tools := range [][]Tool{mcpTools, skillTools} {
		for _, t := range tools {
			mt, ok := t.(*trackedTool)
			if !ok {
				continue
			}
			dt, ok := mt.baseTool.(*dynamicTool)
			if !ok {
				continue
			}
			params := dt.params
			if params == nil {
				params = map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}
			}
			result = append(result, openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        dt.toolName,
					Description: dt.toolDesc,
					Parameters:  params,
				},
			})
		}
	}

	return result
}
