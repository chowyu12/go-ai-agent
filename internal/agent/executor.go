package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"

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

type ProviderFactory func(p *model.Provider, modelName string) (provider.LLMProvider, error)

type ExecutorOption func(*Executor)

func WithProviderFactory(f ProviderFactory) ExecutorOption {
	return func(e *Executor) { e.providerFactory = f }
}

type Executor struct {
	store           store.Store
	registry        *ToolRegistry
	memory          *MemoryManager
	providerFactory ProviderFactory
}

func NewExecutor(s store.Store, registry *ToolRegistry, opts ...ExecutorOption) *Executor {
	e := &Executor{
		store:           s,
		registry:        registry,
		memory:          NewMemoryManager(s),
		providerFactory: provider.NewFromProvider,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
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

	llmProv, err := e.providerFactory(prov, ag.ModelName)
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

	subAgentLCTools := e.buildSubAgentLCTools(ctx, ag.ID, tracker)
	l.WithField("sub_agent_count", len(subAgentLCTools)).Info("[Execute] collected sub-agents")

	if len(agentTools) > 0 || len(subAgentLCTools) > 0 {
		l.Info("[Execute] using tool-augmented execution")
		return e.executeWithTools(ctx, ag, prov, llmProv, agentTools, subAgentLCTools, conv, skills, req.Message, tracker)
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

	messages := e.buildMessages(ag, skills, history, userMsg, nil)
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

func (e *Executor) executeWithTools(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, agentTools []model.Tool, subAgentLCTools []tools.Tool, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker) (*ExecuteResult, error) {
	l := log.WithFields(log.Fields{"agent": ag.Name, "conv_id": conv.ID, "mode": "function_calling"})

	history, err := e.memory.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		l.WithError(err).Error("[ToolExec] load history failed")
		return nil, err
	}
	l.WithField("history_count", len(history)).Info("[ToolExec] history loaded")

	if err := e.memory.SaveMessage(ctx, conv.ID, "user", userMsg, 0); err != nil {
		l.WithError(err).Error("[ToolExec] save user message failed")
		return nil, err
	}

	lcTools := e.registry.BuildTrackedTools(agentTools, tracker)
	lcTools = append(lcTools, subAgentLCTools...)
	toolMap := make(map[string]tools.Tool, len(lcTools))
	for _, t := range lcTools {
		toolMap[t.Name()] = t
	}

	llmToolDefs := buildLLMToolDefs(agentTools, subAgentLCTools)
	allToolNames := make([]string, 0, len(toolMap))
	for name := range toolMap {
		allToolNames = append(allToolNames, name)
	}
	l.WithFields(log.Fields{"tools": allToolNames, "provider": prov.Name, "model": ag.ModelName}).Info("[ToolExec] prepared function-calling tools")

	messages := e.buildMessages(ag, skills, history, userMsg, allToolNames)
	logMessages(l, "[ToolExec]", messages)

	opts := []llms.CallOption{
		llms.WithTemperature(ag.Temperature),
		llms.WithMaxTokens(ag.MaxTokens),
		llms.WithTools(llmToolDefs),
	}

	const maxIterations = 10
	var finalContent string
	totalStart := time.Now()

	for i := range maxIterations {
		l.WithField("iteration", i+1).Info("[ToolExec] >>>>>> LLM call")
		iterStart := time.Now()
		resp, err := llmProv.GenerateContent(ctx, messages, opts...)
		iterDur := time.Since(iterStart)

		if err != nil {
			l.WithFields(log.Fields{"iteration": i + 1, "duration": iterDur}).WithError(err).Error("[ToolExec] LLM call failed")
			tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, "", model.StepError, err.Error(), iterDur, 0, &model.StepMetadata{
				Provider:    prov.Name,
				Model:       ag.ModelName,
				Temperature: ag.Temperature,
			})
			return nil, fmt.Errorf("generate content: %w", err)
		}

		if len(resp.Choices) == 0 {
			l.Warn("[ToolExec] empty response from LLM")
			break
		}

		choice := resp.Choices[0]

		if len(choice.ToolCalls) == 0 {
			finalContent = choice.Content
			l.WithFields(log.Fields{
				"iteration":   i + 1,
				"duration":    iterDur,
				"content_len": len(finalContent),
				"preview":     truncateLog(finalContent, 300),
			}).Info("[ToolExec] <<<<<< got final answer (no more tool calls)")
			break
		}

		l.WithFields(log.Fields{
			"iteration":       i + 1,
			"tool_call_count": len(choice.ToolCalls),
			"duration":        iterDur,
		}).Info("[ToolExec] LLM requested tool calls")

		aiParts := make([]llms.ContentPart, 0, len(choice.ToolCalls)+1)
		if choice.Content != "" {
			aiParts = append(aiParts, llms.TextContent{Text: choice.Content})
		}
		for _, tc := range choice.ToolCalls {
			aiParts = append(aiParts, tc)
		}
		messages = append(messages, llms.MessageContent{
			Role:  llms.ChatMessageTypeAI,
			Parts: aiParts,
		})

		for _, tc := range choice.ToolCalls {
			toolName := tc.FunctionCall.Name
			toolArgs := tc.FunctionCall.Arguments

			tool, ok := toolMap[toolName]
			if !ok {
				errMsg := fmt.Sprintf("tool %q not found", toolName)
				l.WithField("tool", toolName).Warn("[ToolExec] " + errMsg)
				messages = append(messages, llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{ToolCallID: tc.ID, Name: toolName, Content: errMsg},
					},
				})
				continue
			}

			l.WithFields(log.Fields{"tool": toolName, "args": truncateLog(toolArgs, 300)}).Info("[ToolExec] >>> executing tool")
			output, callErr := tool.Call(ctx, toolArgs)
			toolResult := output
			if callErr != nil {
				toolResult = fmt.Sprintf("error: %s", callErr)
				l.WithFields(log.Fields{"tool": toolName}).WithError(callErr).Error("[ToolExec] <<< tool failed")
			} else {
				l.WithFields(log.Fields{"tool": toolName, "output_preview": truncateLog(output, 300)}).Info("[ToolExec] <<< tool succeeded")
			}

			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{ToolCallID: tc.ID, Name: toolName, Content: toolResult},
				},
			})
		}
	}

	totalDuration := time.Since(totalStart)

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        finalContent,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[ToolExec] save assistant message failed")
		return nil, err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, finalContent, model.StepSuccess, "", totalDuration, 0, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithFields(log.Fields{"msg_id": assistantMsg.ID, "steps": len(tracker.Steps()), "total_duration": totalDuration}).Info("[ToolExec] completed")
	return &ExecuteResult{
		ConversationID: conv.UUID,
		Content:        finalContent,
		Steps:          tracker.Steps(),
	}, nil
}

func buildLLMToolDefs(modelTools []model.Tool, subAgentTools []tools.Tool) []llms.Tool {
	var result []llms.Tool

	for _, mt := range modelTools {
		if !mt.Enabled {
			continue
		}
		fd := &llms.FunctionDefinition{
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
		result = append(result, llms.Tool{Type: "function", Function: fd})
	}

	for _, t := range subAgentTools {
		result = append(result, llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
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

	llmProv, err := e.providerFactory(prov, ag.ModelName)
	if err != nil {
		l.WithError(err).Error("[Stream] create llm provider failed")
		return fmt.Errorf("create llm provider: %w", err)
	}

	skills, err := e.store.GetAgentSkills(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Stream] get skills failed")
		return fmt.Errorf("get agent skills: %w", err)
	}

	agentTools, err := e.collectTools(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Stream] collect tools failed")
		return err
	}

	conv, err := e.memory.GetOrCreateConversation(ctx, req.ConversationID, ag.ID, req.UserID)
	if err != nil {
		l.WithError(err).Error("[Stream] get/create conversation failed")
		return fmt.Errorf("get conversation: %w", err)
	}
	l = l.WithField("conv_id", conv.ID)
	l.WithField("conv_uuid", conv.UUID).Info("[Stream] conversation ready")

	tracker := NewStepTracker(e.store, conv.ID)

	subAgentLCTools := e.buildSubAgentLCTools(ctx, ag.ID, tracker)

	if len(agentTools) > 0 || len(subAgentLCTools) > 0 {
		l.WithFields(log.Fields{"tool_count": len(agentTools), "sub_agent_count": len(subAgentLCTools)}).Info("[Stream] tools detected, using tool-augmented streaming")
		return e.streamWithTools(ctx, ag, prov, llmProv, agentTools, subAgentLCTools, conv, skills, req.Message, tracker, chunkHandler)
	}

	history, err := e.memory.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		l.WithError(err).Error("[Stream] load history failed")
		return err
	}
	l.WithField("history_count", len(history)).Info("[Stream] history loaded")

	messages := e.buildMessages(ag, skills, history, req.Message, nil)
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

func (e *Executor) streamWithTools(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, agentTools []model.Tool, subAgentLCTools []tools.Tool, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker, chunkHandler func(chunk model.StreamChunk) error) error {
	l := log.WithFields(log.Fields{"agent": ag.Name, "conv_id": conv.ID, "mode": "stream_tools"})

	result, err := e.executeWithTools(ctx, ag, prov, llmProv, agentTools, subAgentLCTools, conv, skills, userMsg, tracker)
	if err != nil {
		l.WithError(err).Error("[StreamTools] tool execution failed")
		return err
	}

	content := result.Content
	const chunkSize = 100
	for i := 0; i < len(content); i += chunkSize {
		end := min(i+chunkSize, len(content))
		if err := chunkHandler(model.StreamChunk{
			ConversationID: conv.UUID,
			Delta:          content[i:end],
		}); err != nil {
			return err
		}
	}

	var lastStep *model.ExecutionStep
	if steps := tracker.Steps(); len(steps) > 0 {
		last := steps[len(steps)-1]
		lastStep = &last
	}

	l.WithField("response_len", len(content)).Info("[StreamTools] completed")
	return chunkHandler(model.StreamChunk{
		ConversationID: conv.UUID,
		Done:           true,
		Step:           lastStep,
	})
}

func (e *Executor) buildMessages(ag *model.Agent, skills []model.Skill, history []llms.MessageContent, userMsg string, toolNames []string) []llms.MessageContent {
	systemPrompt := buildSystemPrompt(ag, skills, toolNames)

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

func buildSystemPrompt(ag *model.Agent, skills []model.Skill, toolNames []string) string {
	var sb strings.Builder
	if ag.SystemPrompt != "" {
		sb.WriteString(ag.SystemPrompt)
	}
	for _, sk := range skills {
		if sk.Instruction != "" {
			sb.WriteString("\n\n## Skill: " + sk.Name + "\n" + sk.Instruction)
		}
	}
	if len(toolNames) > 0 {
		sb.WriteString("\n\n## 工具使用策略\n")
		sb.WriteString("你拥有以下工具: " + strings.Join(toolNames, ", ") + "\n")
		sb.WriteString("请在回答问题时优先使用可用的工具来获取信息或执行操作，而不是仅依赖你的内置知识。\n")
		sb.WriteString("思考步骤：1. 分析用户问题 2. 判断哪些工具可以帮助回答 3. 调用工具 4. 综合工具结果给出最终回答。\n")
		sb.WriteString("如果问题可以通过工具获得更准确的答案，必须优先调用工具。\n")
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

type subAgentTool struct {
	agentUUID   string
	agentName   string
	description string
	executor    *Executor
	tracker     *StepTracker
}

func (t *subAgentTool) Name() string {
	return "delegate_" + sanitizeToolName(t.agentName)
}

func (t *subAgentTool) Description() string {
	desc := t.description
	if desc == "" {
		desc = "A sub-agent"
	}
	return fmt.Sprintf("将任务委托给子Agent '%s': %s。输入你要委托的具体任务描述。", t.agentName, desc)
}

func (t *subAgentTool) Call(ctx context.Context, input string) (string, error) {
	start := time.Now()
	result, err := t.executor.Execute(ctx, model.ChatRequest{
		AgentID: t.agentUUID,
		UserID:  "system",
		Message: input,
	})
	duration := time.Since(start)

	status := model.StepSuccess
	errMsg := ""
	output := ""
	if err != nil {
		status = model.StepError
		errMsg = err.Error()
	} else {
		output = result.Content
	}

	t.tracker.RecordStep(ctx, model.StepAgentCall, t.agentName, input, output, status, errMsg, duration, 0, &model.StepMetadata{
		AgentUUID: t.agentUUID,
		AgentName: t.agentName,
	})

	if err != nil {
		return "", err
	}
	return result.Content, nil
}

var _ tools.Tool = (*subAgentTool)(nil)

func sanitizeToolName(name string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	s := sb.String()
	if s == "" {
		return "agent"
	}
	return s
}

func (e *Executor) buildSubAgentLCTools(ctx context.Context, agentID int64, tracker *StepTracker) []tools.Tool {
	children, err := e.store.GetAgentChildren(ctx, agentID)
	if err != nil || len(children) == 0 {
		return nil
	}
	result := make([]tools.Tool, 0, len(children))
	for _, child := range children {
		result = append(result, &subAgentTool{
			agentUUID:   child.UUID,
			agentName:   child.Name,
			description: child.Description,
			executor:    e,
			tracker:     tracker,
		})
	}
	return result
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
