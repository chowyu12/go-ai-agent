package agent

import (
	"strings"

	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

func extractContent(resp openai.ChatCompletionResponse) string {
	if len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}

func applyModelCaps(req *openai.ChatCompletionRequest, ag *model.Agent, l *log.Entry) {
	caps := model.GetModelCaps(ag.ModelName)
	if caps.NoTemperature || caps.NoTopP {
		l.WithFields(log.Fields{
			"model":          ag.ModelName,
			"no_temperature": caps.NoTemperature,
			"no_top_p":       caps.NoTopP,
		}).Debug("[LLM] model caps applied")
	}
	if ag.Temperature > 0 && !caps.NoTemperature {
		req.Temperature = float32(ag.Temperature)
	}
	if ag.MaxTokens > 0 {
		req.MaxCompletionTokens = ag.MaxTokens
	}
}

func logResourceSummary(l *log.Entry, agentTools []model.Tool, skills []model.Skill) {
	toolNames := make([]string, 0, len(agentTools))
	for _, t := range agentTools {
		toolNames = append(toolNames, t.Name)
	}
	skillNames := make([]string, 0, len(skills))
	for _, s := range skills {
		skillNames = append(skillNames, s.Name)
	}
	l.WithFields(log.Fields{
		"tools":  toolNames,
		"skills": skillNames,
	}).Info("[Prepare]    resources loaded")

	for _, sk := range skills {
		fields := log.Fields{"skill": sk.Name, "has_instruction": sk.Instruction != ""}
		if sk.Instruction != "" {
			fields["instruction_len"] = len(sk.Instruction)
		}
		l.WithFields(fields).Debug("[Prepare]    skill detail")
	}
}

func logMessages(l *log.Entry, messages []openai.ChatCompletionMessage) {
	for i, msg := range messages {
		content := msg.Content
		if content == "" && len(msg.MultiContent) > 0 {
			var parts []string
			for _, p := range msg.MultiContent {
				if p.Type == openai.ChatMessagePartTypeText {
					parts = append(parts, p.Text)
				}
			}
			content = strings.Join(parts, "")
		}
		l.WithFields(log.Fields{
			"idx":  i,
			"role": msg.Role,
			"len":  len(content),
			"text": truncateLog(content, 300),
		}).Debug("[LLM]    message")
	}
}

func truncateLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
