package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/provider"
	"github.com/chowyu12/go-ai-agent/internal/store"
)

type ExecuteResult struct {
	ConversationID string
	Content        string
	TokensUsed     int
	Steps          []model.ExecutionStep
}

type Executor struct {
	store    store.Store
	registry *ToolRegistry
	memory   *MemoryManager
}

func NewExecutor(s store.Store, registry *ToolRegistry) *Executor {
	return &Executor{
		store:    s,
		registry: registry,
		memory:   NewMemoryManager(s),
	}
}

func (e *Executor) Execute(ctx context.Context, req model.ChatRequest) (*ExecuteResult, error) {
	l := log.WithFields(log.Fields{"agent_uuid": req.AgentID, "user_id": req.UserID})
	reqBody, _ := json.Marshal(req)
	l.WithField("request_body", string(reqBody)).Info("[Execute] start")

	ag, err := e.store.GetAgentByUUID(ctx, req.AgentID)
	if err != nil {
		l.WithError(err).Error("[Execute] agent not found")
		return nil, fmt.Errorf("agent not found: %w", err)
	}
	l = l.WithFields(log.Fields{"agent_name": ag.Name, "agent_id": ag.ID})

	prov, err := e.store.GetProvider(ctx, ag.ProviderID)
	if err != nil {
		l.WithFields(log.Fields{"provider_id": ag.ProviderID}).WithError(err).Error("[Execute] provider not found")
		return nil, fmt.Errorf("provider not found: %w", err)
	}
	l.WithFields(log.Fields{"provider": prov.Name, "model": ag.ModelName}).Info("[Execute] resolved provider")

	llmProv, err := provider.NewFromProvider(prov, ag.ModelName)
	if err != nil {
		l.WithError(err).Error("[Execute] create llm provider failed")
		return nil, fmt.Errorf("create llm provider: %w", err)
	}

	agentTools, err := e.collectTools(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Execute] collect tools failed")
		return nil, err
	}
	l.WithField("tool_count", len(agentTools)).Info("[Execute] collected tools")

	skills, err := e.store.GetAgentSkills(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Execute] get skills failed")
		return nil, fmt.Errorf("get agent skills: %w", err)
	}
	l.WithField("skill_count", len(skills)).Info("[Execute] loaded skills")

	conv, err := e.memory.GetOrCreateConversation(ctx, req.ConversationID, ag.ID, req.UserID)
	if err != nil {
		l.WithError(err).Error("[Execute] get/create conversation failed")
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	l.WithFields(log.Fields{"conv_id": conv.ID, "conv_uuid": conv.UUID}).Info("[Execute] conversation ready")

	tracker := NewStepTracker(e.store, conv.ID)

	if len(agentTools) > 0 {
		l.Info("[Execute] using tool-augmented execution")
		return e.executeWithTools(ctx, ag, prov, llmProv, agentTools, conv, skills, req.Message, tracker)
	}
	l.Info("[Execute] using simple execution")
	return e.executeSimple(ctx, ag, prov, llmProv, conv, skills, req.Message, tracker)
}

func (e *Executor) collectTools(ctx context.Context, agentID int64) ([]model.Tool, error) {
	agentTools, err := e.store.GetAgentTools(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent tools: %w", err)
	}
	skills, err := e.store.GetAgentSkills(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent skills: %w", err)
	}
	for _, sk := range skills {
		skillTools, err := e.store.GetSkillTools(ctx, sk.ID)
		if err != nil {
			return nil, fmt.Errorf("get skill tools: %w", err)
		}
		agentTools = append(agentTools, skillTools...)
	}
	return agentTools, nil
}

func (e *Executor) executeSimple(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker) (*ExecuteResult, error) {
	l := log.WithFields(log.Fields{"agent": ag.Name, "conv_id": conv.ID, "mode": "simple"})

	history, err := e.memory.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		l.WithError(err).Error("[SimpleExec] load history failed")
		return nil, err
	}
	l.WithField("history_count", len(history)).Info("[SimpleExec] history loaded")

	messages := e.buildMessages(ag, skills, history, userMsg)
	l.WithField("total_messages", len(messages)).Info("[SimpleExec] messages built")
	logMessages(l, "[SimpleExec]", messages)

	opts := []llms.CallOption{
		llms.WithTemperature(ag.Temperature),
		llms.WithMaxTokens(ag.MaxTokens),
	}

	if err := e.memory.SaveMessage(ctx, conv.ID, "user", userMsg, 0); err != nil {
		l.WithError(err).Error("[SimpleExec] save user message failed")
		return nil, err
	}

	l.WithFields(log.Fields{"provider": prov.Name, "model": ag.ModelName, "temperature": ag.Temperature, "max_tokens": ag.MaxTokens}).Info("[SimpleExec] calling LLM")
	start := time.Now()
	resp, err := llmProv.GenerateContent(ctx, messages, opts...)
	duration := time.Since(start)

	if err != nil {
		l.WithFields(log.Fields{"duration": duration}).WithError(err).Error("[SimpleExec] LLM call failed")
		tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, "", model.StepError, err.Error(), duration, 0, &model.StepMetadata{
			Provider:    prov.Name,
			Model:       ag.ModelName,
			Temperature: ag.Temperature,
		})
		return nil, fmt.Errorf("generate content: %w", err)
	}

	content := extractContent(resp)
	l.WithFields(log.Fields{"duration": duration, "response_len": len(content), "response_preview": truncateLog(content, 300)}).Info("[SimpleExec] LLM call success")

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        content,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[SimpleExec] save assistant message failed")
		return nil, err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, content, model.StepSuccess, "", duration, 0, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithField("msg_id", assistantMsg.ID).Info("[SimpleExec] completed")
	return &ExecuteResult{
		ConversationID: conv.UUID,
		Content:        content,
		Steps:          tracker.Steps(),
	}, nil
}

func (e *Executor) executeWithTools(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, agentTools []model.Tool, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker) (*ExecuteResult, error) {
	l := log.WithFields(log.Fields{"agent": ag.Name, "conv_id": conv.ID, "mode": "tools"})

	history, err := e.memory.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		l.WithError(err).Error("[ToolExec] load history failed")
		return nil, err
	}
	l.WithField("history_count", len(history)).Info("[ToolExec] history loaded")

	systemPrompt := buildSystemPrompt(ag, skills)

	var historyText strings.Builder
	for _, msg := range history {
		for _, part := range msg.Parts {
			if tc, ok := part.(llms.TextContent); ok {
				switch msg.Role {
				case llms.ChatMessageTypeHuman:
					historyText.WriteString("User: " + tc.Text + "\n")
				case llms.ChatMessageTypeAI:
					historyText.WriteString("Assistant: " + tc.Text + "\n")
				}
			}
		}
	}

	if err := e.memory.SaveMessage(ctx, conv.ID, "user", userMsg, 0); err != nil {
		l.WithError(err).Error("[ToolExec] save user message failed")
		return nil, err
	}

	toolNames := make([]string, 0, len(agentTools))
	for _, t := range agentTools {
		toolNames = append(toolNames, t.Name)
	}
	l.WithFields(log.Fields{"tools": toolNames, "provider": prov.Name, "model": ag.ModelName}).Info("[ToolExec] building agent")

	lcTools := e.registry.BuildTrackedTools(agentTools, tracker)
	agentObj := agents.NewConversationalAgent(llmProv.GetModel(), lcTools)
	executor := agents.NewExecutor(agentObj, agents.WithMaxIterations(5))

	input := userMsg
	if historyText.Len() > 0 {
		input = fmt.Sprintf("System: %s\n\nConversation history:\n%s\nUser: %s", systemPrompt, historyText.String(), userMsg)
	} else if systemPrompt != "" {
		input = fmt.Sprintf("System: %s\n\nUser: %s", systemPrompt, userMsg)
	}

	l.WithField("input", input).Debug("[ToolExec] full input")
	l.WithFields(log.Fields{"input_len": len(input), "input_preview": truncateLog(input, 300)}).Info("[ToolExec] running agent chain")
	start := time.Now()
	result, err := chains.Run(ctx, executor, input)
	duration := time.Since(start)

	if err != nil {
		l.WithFields(log.Fields{"duration": duration}).WithError(err).Error("[ToolExec] agent chain failed")
		tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, input, "", model.StepError, err.Error(), duration, 0, &model.StepMetadata{
			Provider:    prov.Name,
			Model:       ag.ModelName,
			Temperature: ag.Temperature,
		})
		return nil, fmt.Errorf("run agent: %w", err)
	}

	l.WithFields(log.Fields{"duration": duration, "response_len": len(result), "response_preview": truncateLog(result, 300)}).Info("[ToolExec] agent chain success")

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        result,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[ToolExec] save assistant message failed")
		return nil, err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, result, model.StepSuccess, "", duration, 0, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithFields(log.Fields{"msg_id": assistantMsg.ID, "steps": len(tracker.Steps())}).Info("[ToolExec] completed")
	return &ExecuteResult{
		ConversationID: conv.UUID,
		Content:        result,
		Steps:          tracker.Steps(),
	}, nil
}

func (e *Executor) ExecuteStream(ctx context.Context, req model.ChatRequest, chunkHandler func(chunk model.StreamChunk) error) error {
	l := log.WithFields(log.Fields{"agent_uuid": req.AgentID, "user_id": req.UserID, "mode": "stream"})
	reqBody, _ := json.Marshal(req)
	l.WithField("request_body", string(reqBody)).Info("[Stream] start")

	ag, err := e.store.GetAgentByUUID(ctx, req.AgentID)
	if err != nil {
		l.WithError(err).Error("[Stream] agent not found")
		return fmt.Errorf("agent not found: %w", err)
	}
	l = l.WithFields(log.Fields{"agent": ag.Name, "agent_id": ag.ID})

	prov, err := e.store.GetProvider(ctx, ag.ProviderID)
	if err != nil {
		l.WithFields(log.Fields{"provider_id": ag.ProviderID}).WithError(err).Error("[Stream] provider not found")
		return fmt.Errorf("provider not found: %w", err)
	}
	l.WithFields(log.Fields{"provider": prov.Name, "model": ag.ModelName}).Info("[Stream] resolved provider")

	llmProv, err := provider.NewFromProvider(prov, ag.ModelName)
	if err != nil {
		l.WithError(err).Error("[Stream] create llm provider failed")
		return fmt.Errorf("create llm provider: %w", err)
	}

	skills, err := e.store.GetAgentSkills(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Stream] get skills failed")
		return fmt.Errorf("get agent skills: %w", err)
	}

	conv, err := e.memory.GetOrCreateConversation(ctx, req.ConversationID, ag.ID, req.UserID)
	if err != nil {
		l.WithError(err).Error("[Stream] get/create conversation failed")
		return fmt.Errorf("get conversation: %w", err)
	}
	l = l.WithField("conv_id", conv.ID)
	l.WithField("conv_uuid", conv.UUID).Info("[Stream] conversation ready")

	tracker := NewStepTracker(e.store, conv.ID)

	history, err := e.memory.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		l.WithError(err).Error("[Stream] load history failed")
		return err
	}
	l.WithField("history_count", len(history)).Info("[Stream] history loaded")

	messages := e.buildMessages(ag, skills, history, req.Message)
	l.WithFields(log.Fields{"total_messages": len(messages), "skill_count": len(skills)}).Info("[Stream] messages built")
	logMessages(l, "[Stream]", messages)

	opts := []llms.CallOption{
		llms.WithTemperature(ag.Temperature),
		llms.WithMaxTokens(ag.MaxTokens),
	}

	var fullContent strings.Builder
	var chunkCount int

	streamHandler := func(_ context.Context, chunk []byte) error {
		chunkCount++
		text := string(chunk)
		fullContent.WriteString(text)
		return chunkHandler(model.StreamChunk{
			ConversationID: conv.UUID,
			Delta:          text,
		})
	}

	if err := e.memory.SaveMessage(ctx, conv.ID, "user", req.Message, 0); err != nil {
		l.WithError(err).Error("[Stream] save user message failed")
		return err
	}

	l.WithFields(log.Fields{"provider": prov.Name, "model": ag.ModelName, "temperature": ag.Temperature, "max_tokens": ag.MaxTokens}).Info("[Stream] calling LLM (streaming)")
	start := time.Now()
	_, err = llmProv.StreamContent(ctx, messages, streamHandler, opts...)
	duration := time.Since(start)

	if err != nil {
		l.WithFields(log.Fields{"duration": duration, "chunks_received": chunkCount}).WithError(err).Error("[Stream] LLM streaming failed")
		tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, req.Message, "", model.StepError, err.Error(), duration, 0, &model.StepMetadata{
			Provider:    prov.Name,
			Model:       ag.ModelName,
			Temperature: ag.Temperature,
		})
		return fmt.Errorf("stream content: %w", err)
	}

	content := fullContent.String()
	l.WithFields(log.Fields{"duration": duration, "chunks": chunkCount, "response_len": len(content), "response_preview": truncateLog(content, 300)}).Info("[Stream] LLM streaming success")

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        content,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[Stream] save assistant message failed")
		return err
	}

	tracker.SetMessageID(assistantMsg.ID)
	step := tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, req.Message, content, model.StepSuccess, "", duration, 0, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithField("msg_id", assistantMsg.ID).Info("[Stream] completed")
	return chunkHandler(model.StreamChunk{
		ConversationID: conv.UUID,
		Done:           true,
		Step:           step,
	})
}

func (e *Executor) buildMessages(ag *model.Agent, skills []model.Skill, history []llms.MessageContent, userMsg string) []llms.MessageContent {
	systemPrompt := buildSystemPrompt(ag, skills)

	var messages []llms.MessageContent
	if systemPrompt != "" {
		messages = append(messages, llms.MessageContent{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextContent{Text: systemPrompt}},
		})
	}

	messages = append(messages, history...)

	messages = append(messages, llms.MessageContent{
		Role:  llms.ChatMessageTypeHuman,
		Parts: []llms.ContentPart{llms.TextContent{Text: userMsg}},
	})

	return messages
}

func buildSystemPrompt(ag *model.Agent, skills []model.Skill) string {
	var sb strings.Builder
	if ag.SystemPrompt != "" {
		sb.WriteString(ag.SystemPrompt)
	}
	for _, sk := range skills {
		if sk.Instruction != "" {
			sb.WriteString("\n\n## Skill: " + sk.Name + "\n" + sk.Instruction)
		}
	}
	return sb.String()
}

func extractContent(resp *llms.ContentResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Content
}

func truncateLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func logMessages(l *log.Entry, prefix string, messages []llms.MessageContent) {
	for i, msg := range messages {
		var textParts []string
		for _, part := range msg.Parts {
			if tc, ok := part.(llms.TextContent); ok {
				textParts = append(textParts, tc.Text)
			}
		}
		content := strings.Join(textParts, "")
		l.WithFields(log.Fields{
			"index":       i,
			"role":        string(msg.Role),
			"content_len": len(content),
			"content":     truncateLog(content, 500),
		}).Info(prefix + " llm_message")
	}
}
