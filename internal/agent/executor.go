package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/provider"
	"github.com/chowyu12/go-ai-agent/internal/store"
)

// ============================================================
//  类型定义
// ============================================================

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
		memory:          NewMemoryManager(s, s),
		providerFactory: provider.NewFromProvider,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// execContext 聚合单次执行所需的全部上下文。
type execContext struct {
	ag      *model.Agent
	prov    *model.Provider
	llmProv provider.LLMProvider
	conv    *model.Conversation
	skills  []model.Skill
	tracker *StepTracker
	files   []*model.File
	userMsg string
	l       *log.Entry

	agentTools    []model.Tool
	subAgentTools []Tool
	toolSkillMap  map[string]string
}

func (ec *execContext) hasTools() bool {
	return len(ec.agentTools) > 0 || len(ec.subAgentTools) > 0
}

func (ec *execContext) stepMeta() *model.StepMetadata {
	return &model.StepMetadata{
		Provider:    ec.prov.Name,
		Model:       ec.ag.ModelName,
		Temperature: ec.ag.Temperature,
	}
}

// ============================================================
//  对外入口
// ============================================================

func (e *Executor) Execute(ctx context.Context, req model.ChatRequest) (*ExecuteResult, error) {
	ec, err := e.prepare(ctx, req)
	if err != nil {
		return nil, err
	}

	ec.l.WithField("user", req.UserID).Info("[Execute] >> start")
	if body, err := json.Marshal(req); err == nil {
		ec.l.WithField("body", string(body)).Debug("[Execute]    request body")
	}

	return e.execute(ctx, ec)
}

func (e *Executor) ExecuteStream(ctx context.Context, req model.ChatRequest, chunkHandler func(chunk model.StreamChunk) error) error {
	ec, err := e.prepare(ctx, req)
	if err != nil {
		return err
	}

	ec.l.WithField("user", req.UserID).Info("[Execute] >> start (stream)")

	ec.tracker.SetOnStep(func(step model.ExecutionStep) {
		_ = chunkHandler(model.StreamChunk{
			ConversationID: ec.conv.UUID,
			Step:           &step,
		})
	})

	return e.stream(ctx, ec, chunkHandler)
}

// ============================================================
//  准备阶段：构建 execContext
// ============================================================

func (e *Executor) prepare(ctx context.Context, req model.ChatRequest) (*execContext, error) {
	ag, err := e.store.GetAgentByUUID(ctx, req.AgentID)
	if err != nil {
		log.WithField("agent_uuid", req.AgentID).WithError(err).Error("[Execute] agent not found")
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	prov, err := e.store.GetProvider(ctx, ag.ProviderID)
	if err != nil {
		log.WithFields(log.Fields{"agent": ag.Name, "provider_id": ag.ProviderID}).WithError(err).Error("[Execute] provider not found")
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	l := log.WithFields(log.Fields{"agent": ag.Name, "provider": prov.Name, "model": ag.ModelName})

	llmProv, err := e.providerFactory(prov, ag.ModelName)
	if err != nil {
		l.WithError(err).Error("[Execute] create llm provider failed")
		return nil, fmt.Errorf("create llm provider: %w", err)
	}

	agentTools, toolSkillMap, err := e.collectTools(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Execute] collect tools failed")
		return nil, err
	}

	skills, err := e.store.GetAgentSkills(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Execute] get skills failed")
		return nil, fmt.Errorf("get agent skills: %w", err)
	}

	isNewConv := req.ConversationID == ""
	conv, err := e.memory.GetOrCreateConversation(ctx, req.ConversationID, ag.ID, req.UserID)
	if err != nil {
		l.WithError(err).Error("[Execute] get/create conversation failed")
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	if isNewConv {
		e.memory.AutoSetTitle(ctx, conv.ID, req.Message)
	}

	tracker := NewStepTracker(e.store, conv.ID)
	subAgentTools := e.buildSubAgentTools(ctx, ag.ID, tracker)

	logResourceSummary(l, agentTools, skills, subAgentTools)

	files := e.loadRequestFiles(ctx, req.Files, conv.ID)

	return &execContext{
		ag:            ag,
		prov:          prov,
		llmProv:       llmProv,
		conv:          conv,
		skills:        skills,
		tracker:       tracker,
		files:         files,
		userMsg:       req.Message,
		l:             l.WithField("conv", conv.UUID),
		agentTools:    agentTools,
		subAgentTools: subAgentTools,
		toolSkillMap:  toolSkillMap,
	}, nil
}

func (e *Executor) collectTools(ctx context.Context, agentID int64) ([]model.Tool, map[string]string, error) {
	agentTools, err := e.store.GetAgentTools(ctx, agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("get agent tools: %w", err)
	}

	toolSkillMap := make(map[string]string)

	skills, err := e.store.GetAgentSkills(ctx, agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("get agent skills: %w", err)
	}
	for _, sk := range skills {
		skillTools, err := e.store.GetSkillTools(ctx, sk.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("get skill tools: %w", err)
		}
		if len(skillTools) > 0 {
			names := make([]string, 0, len(skillTools))
			for _, t := range skillTools {
				names = append(names, t.Name)
				toolSkillMap[t.Name] = sk.Name
			}
			log.WithFields(log.Fields{"skill": sk.Name, "tools": names}).Debug("[Execute]    skill contributed tools")
		}
		agentTools = append(agentTools, skillTools...)
	}
	return agentTools, toolSkillMap, nil
}

// ============================================================
//  核心执行（非流式，统一有/无工具）
// ============================================================

func (e *Executor) execute(ctx context.Context, ec *execContext) (*ExecuteResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(ec.ag.TimeoutSeconds())*time.Second)
	defer cancel()

	history, err := e.memory.LoadHistory(ctx, ec.conv.ID, ec.ag.HistoryLimit())
	if err != nil {
		ec.l.WithError(err).Error("[LLM] load history failed")
		return nil, err
	}

	if _, err := e.memory.SaveUserMessage(ctx, ec.conv.ID, ec.userMsg, ec.files); err != nil {
		ec.l.WithError(err).Error("[LLM] save user message failed")
		return nil, err
	}

	var toolMap map[string]Tool
	var allToolNames []string
	var toolDefs []openai.Tool
	calledTools := make(map[string]bool)

	if ec.hasTools() {
		lcTools := e.registry.BuildTrackedTools(ec.agentTools, ec.tracker, ec.toolSkillMap)
		lcTools = append(lcTools, ec.subAgentTools...)
		toolMap = make(map[string]Tool, len(lcTools))
		allToolNames = make([]string, 0, len(lcTools))
		for _, t := range lcTools {
			toolMap[t.Name()] = t
			allToolNames = append(allToolNames, t.Name())
		}
		toolDefs = buildLLMToolDefs(ec.agentTools, ec.subAgentTools)
		ec.l.Info("[Execute]    mode = tool-augmented")
	} else {
		ec.l.Info("[Execute]    mode = simple")
	}

	messages := buildMessages(ec.ag, ec.skills, history, ec.userMsg, allToolNames, ec.files)
	logMessages(ec.l, messages)

	req := openai.ChatCompletionRequest{
		Model: ec.ag.ModelName,
		Tools: toolDefs,
	}
	applyModelCaps(&req, ec.ag, ec.l)

	var finalContent string
	var totalTokens int
	totalStart := time.Now()

	for i := range ec.ag.IterationLimit() {
		req.Messages = messages
		ec.l.WithFields(log.Fields{"round": i + 1, "model": ec.ag.ModelName}).Info("[LLM] >> call")
		iterStart := time.Now()
		resp, err := ec.llmProv.CreateChatCompletion(ctx, req)
		iterDur := time.Since(iterStart)

		if err != nil {
			ec.l.WithFields(log.Fields{"round": i + 1, "duration": iterDur}).WithError(err).Error("[LLM] << failed")
			ec.tracker.RecordStep(ctx, model.StepLLMCall, ec.ag.ModelName, ec.userMsg, "", model.StepError, err.Error(), iterDur, 0, ec.stepMeta())
			return nil, fmt.Errorf("generate content: %w", err)
		}

		totalTokens += resp.Usage.TotalTokens

		if len(resp.Choices) == 0 {
			ec.l.Warn("[LLM] << empty response")
			break
		}

		choice := resp.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			finalContent = choice.Message.Content
			ec.l.WithFields(log.Fields{
				"round":    i + 1,
				"duration": iterDur,
				"tokens":   resp.Usage.TotalTokens,
				"len":      len(finalContent),
				"preview":  truncateLog(finalContent, 200),
			}).Info("[LLM] << final answer")
			break
		}

		tcNames := make([]string, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			tcNames = append(tcNames, tc.Function.Name)
		}
		ec.l.WithFields(log.Fields{"round": i + 1, "duration": iterDur, "tool_calls": tcNames}).Info("[LLM] << tool calls requested")

		messages = append(messages, choice.Message)

		var pendingParts []openai.ChatMessagePart
		for _, tc := range choice.Message.ToolCalls {
			toolName := tc.Function.Name
			toolArgs := tc.Function.Arguments

			tool, ok := toolMap[toolName]
			if !ok {
				errMsg := fmt.Sprintf("tool %q not found", toolName)
				ec.l.WithField("tool", toolName).Warn("[Tool] tool not registered, skipping")
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    errMsg,
					ToolCallID: tc.ID,
					Name:       toolName,
				})
				continue
			}

			ec.l.WithFields(log.Fields{"tool": toolName, "args": truncateLog(toolArgs, 200)}).Info("[Tool] >> invoke")
			calledTools[toolName] = true
			callStart := time.Now()
			output, callErr := tool.Call(ctx, toolArgs)
			callDur := time.Since(callStart)
			toolResult := output
			if callErr != nil {
				toolResult = fmt.Sprintf("error: %s", callErr)
				ec.l.WithFields(log.Fields{"tool": toolName, "duration": callDur}).WithError(callErr).Error("[Tool] << failed")
			} else {
				ec.l.WithFields(log.Fields{"tool": toolName, "duration": callDur, "preview": truncateLog(output, 200)}).Info("[Tool] << ok")
			}

			toolMsg, fileParts := e.buildToolResponseParts(ctx, tc.ID, toolName, toolResult, callErr == nil, ec.l)
			messages = append(messages, toolMsg)
			pendingParts = append(pendingParts, fileParts...)
		}
		if len(pendingParts) > 0 {
			parts := append([]openai.ChatMessagePart{
				{Type: openai.ChatMessagePartTypeText, Text: "工具返回了以下文件:"},
			}, pendingParts...)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:         openai.ChatMessageRoleUser,
				MultiContent: parts,
			})
		}
	}

	if ec.hasTools() {
		e.recordUsedSkillSteps(ctx, ec.skills, ec.toolSkillMap, calledTools, ec.tracker)
	}

	return e.saveResult(ctx, ec, finalContent, totalTokens, time.Since(totalStart))
}

// ============================================================
//  流式执行（统一有/无工具，均走真 SSE 流式）
// ============================================================

func (e *Executor) stream(ctx context.Context, ec *execContext, chunkHandler func(chunk model.StreamChunk) error) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(ec.ag.TimeoutSeconds())*time.Second)
	defer cancel()

	history, err := e.memory.LoadHistory(ctx, ec.conv.ID, ec.ag.HistoryLimit())
	if err != nil {
		ec.l.WithError(err).Error("[LLM] load history failed")
		return err
	}

	if _, err := e.memory.SaveUserMessage(ctx, ec.conv.ID, ec.userMsg, ec.files); err != nil {
		ec.l.WithError(err).Error("[LLM] save user message failed")
		return err
	}

	var toolMap map[string]Tool
	var allToolNames []string
	var toolDefs []openai.Tool
	calledTools := make(map[string]bool)

	if ec.hasTools() {
		lcTools := e.registry.BuildTrackedTools(ec.agentTools, ec.tracker, ec.toolSkillMap)
		lcTools = append(lcTools, ec.subAgentTools...)
		toolMap = make(map[string]Tool, len(lcTools))
		allToolNames = make([]string, 0, len(lcTools))
		for _, t := range lcTools {
			toolMap[t.Name()] = t
			allToolNames = append(allToolNames, t.Name())
		}
		toolDefs = buildLLMToolDefs(ec.agentTools, ec.subAgentTools)
		ec.l.Info("[Execute]    mode = stream + tool-augmented")
	} else {
		ec.l.Info("[Execute]    mode = stream")
	}

	messages := buildMessages(ec.ag, ec.skills, history, ec.userMsg, allToolNames, ec.files)
	logMessages(ec.l, messages)

	var totalTokens int
	var finalContent string
	totalStart := time.Now()

	for i := range ec.ag.IterationLimit() {
		apiReq := openai.ChatCompletionRequest{
			Model:    ec.ag.ModelName,
			Messages: messages,
			Tools:    toolDefs,
			Stream:   true,
			StreamOptions: &openai.StreamOptions{
				IncludeUsage: true,
			},
		}
		applyModelCaps(&apiReq, ec.ag, ec.l)

		ec.l.WithFields(log.Fields{"round": i + 1, "model": ec.ag.ModelName}).Info("[LLM] >> call (stream)")
		iterStart := time.Now()

		s, err := ec.llmProv.CreateChatCompletionStream(ctx, apiReq)
		if err != nil {
			iterDur := time.Since(iterStart)
			ec.l.WithFields(log.Fields{"round": i + 1, "duration": iterDur}).WithError(err).Error("[LLM] << stream create failed")
			ec.tracker.RecordStep(ctx, model.StepLLMCall, ec.ag.ModelName, ec.userMsg, "", model.StepError, err.Error(), iterDur, 0, ec.stepMeta())
			return fmt.Errorf("stream content: %w", err)
		}

		var iterContent strings.Builder
		var toolCalls []openai.ToolCall
		var finishReason openai.FinishReason
		var roundTokens int

		for {
			response, recvErr := s.Recv()
			if errors.Is(recvErr, io.EOF) {
				break
			}
			if recvErr != nil {
				s.Close()
				iterDur := time.Since(iterStart)
				ec.l.WithFields(log.Fields{"round": i + 1, "duration": iterDur}).WithError(recvErr).Error("[LLM] << stream recv failed")
				ec.tracker.RecordStep(ctx, model.StepLLMCall, ec.ag.ModelName, ec.userMsg, "", model.StepError, recvErr.Error(), iterDur, 0, ec.stepMeta())
				return fmt.Errorf("stream content: %w", recvErr)
			}

			if response.Usage != nil {
				roundTokens = response.Usage.TotalTokens
			}
			if len(response.Choices) == 0 {
				continue
			}

			choice := response.Choices[0]
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}

			if choice.Delta.Content != "" {
				iterContent.WriteString(choice.Delta.Content)
				if err := chunkHandler(model.StreamChunk{
					ConversationID: ec.conv.UUID,
					Delta:          choice.Delta.Content,
				}); err != nil {
					s.Close()
					return err
				}
			}

			for _, tc := range choice.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				for len(toolCalls) <= idx {
					toolCalls = append(toolCalls, openai.ToolCall{Type: openai.ToolTypeFunction})
				}
				if tc.ID != "" {
					toolCalls[idx].ID = tc.ID
				}
				if tc.Type != "" {
					toolCalls[idx].Type = tc.Type
				}
				toolCalls[idx].Function.Name += tc.Function.Name
				toolCalls[idx].Function.Arguments += tc.Function.Arguments
			}
		}
		s.Close()

		totalTokens += roundTokens
		iterDur := time.Since(iterStart)
		content := iterContent.String()

		if finishReason != openai.FinishReasonToolCalls || len(toolCalls) == 0 {
			finalContent = content
			ec.l.WithFields(log.Fields{
				"round":    i + 1,
				"duration": iterDur,
				"tokens":   roundTokens,
				"len":      len(finalContent),
				"preview":  truncateLog(finalContent, 200),
			}).Info("[LLM] << final answer (stream)")
			break
		}

		tcNames := make([]string, 0, len(toolCalls))
		for _, tc := range toolCalls {
			tcNames = append(tcNames, tc.Function.Name)
		}
		ec.l.WithFields(log.Fields{"round": i + 1, "duration": iterDur, "tokens": roundTokens, "tool_calls": tcNames}).Info("[LLM] << tool calls requested (stream)")

		messages = append(messages, openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   content,
			ToolCalls: toolCalls,
		})

		var pendingParts []openai.ChatMessagePart
		for _, tc := range toolCalls {
			toolName := tc.Function.Name
			toolArgs := tc.Function.Arguments

			tool, ok := toolMap[toolName]
			if !ok {
				errMsg := fmt.Sprintf("tool %q not found", toolName)
				ec.l.WithField("tool", toolName).Warn("[Tool] tool not registered, skipping")
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    errMsg,
					ToolCallID: tc.ID,
					Name:       toolName,
				})
				continue
			}

			ec.l.WithFields(log.Fields{"tool": toolName, "args": truncateLog(toolArgs, 200)}).Info("[Tool] >> invoke")
			calledTools[toolName] = true
			callStart := time.Now()
			output, callErr := tool.Call(ctx, toolArgs)
			callDur := time.Since(callStart)
			toolResult := output
			if callErr != nil {
				toolResult = fmt.Sprintf("error: %s", callErr)
				ec.l.WithFields(log.Fields{"tool": toolName, "duration": callDur}).WithError(callErr).Error("[Tool] << failed")
			} else {
				ec.l.WithFields(log.Fields{"tool": toolName, "duration": callDur, "preview": truncateLog(output, 200)}).Info("[Tool] << ok")
			}

			toolMsg, fileParts := e.buildToolResponseParts(ctx, tc.ID, toolName, toolResult, callErr == nil, ec.l)
			messages = append(messages, toolMsg)
			pendingParts = append(pendingParts, fileParts...)
		}

		if len(pendingParts) > 0 {
			parts := append([]openai.ChatMessagePart{
				{Type: openai.ChatMessagePartTypeText, Text: "工具返回了以下文件:"},
			}, pendingParts...)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:         openai.ChatMessageRoleUser,
				MultiContent: parts,
			})
		}
	}

	if ec.hasTools() {
		e.recordUsedSkillSteps(ctx, ec.skills, ec.toolSkillMap, calledTools, ec.tracker)
	}

	if _, err := e.saveResult(ctx, ec, finalContent, totalTokens, time.Since(totalStart)); err != nil {
		return err
	}

	return chunkHandler(model.StreamChunk{
		ConversationID: ec.conv.UUID,
		Done:           true,
	})
}

// ============================================================
//  结果持久化
// ============================================================

func (e *Executor) saveResult(ctx context.Context, ec *execContext, content string, tokensUsed int, duration time.Duration) (*ExecuteResult, error) {
	msgID, err := e.memory.SaveAssistantMessage(ctx, ec.conv.ID, content, tokensUsed)
	if err != nil {
		ec.l.WithError(err).Error("[Execute] save assistant message failed")
		return nil, err
	}

	ec.tracker.SetMessageID(msgID)
	ec.tracker.RecordStep(ctx, model.StepLLMCall, ec.ag.ModelName, ec.userMsg, content, model.StepSuccess, "", duration, tokensUsed, ec.stepMeta())

	ec.l.WithFields(log.Fields{"msg_id": msgID, "duration": duration, "tokens": tokensUsed}).Info("[Execute] << done")
	return &ExecuteResult{
		ConversationID: ec.conv.UUID,
		Content:        content,
		TokensUsed:     tokensUsed,
		Steps:          ec.tracker.Steps(),
	}, nil
}
