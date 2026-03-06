package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/parser"
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

func (e *Executor) loadRemoteFile(ctx context.Context, rawURL string, chatFileType model.ChatFileType) *model.File {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	l := log.WithField("url", rawURL)
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		l.WithError(err).Warn("[Execute] invalid file URL, skipping")
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		l.WithError(err).Warn("[Execute] fetch file URL failed, skipping")
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20+1))
	resp.Body.Close()
	if err != nil {
		l.WithError(err).Warn("[Execute] read file URL body failed, skipping")
		return nil
	}
	if int64(len(data)) > 20<<20 {
		l.Warn("[Execute] file URL too large (>20MB), skipping")
		return nil
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "text/plain"
	}
	filename := path.Base(rawURL)
	if filename == "" || filename == "." || filename == "/" {
		filename = "remote_file"
	}

	fileType := chatFileTypeToFileType(chatFileType, ct, filename)
	f := &model.File{
		UUID:        rawURL,
		Filename:    filename,
		ContentType: ct,
		FileSize:    int64(len(data)),
		FileType:    fileType,
	}

	ext := filepath.Ext(filename)
	if ext == "" && strings.HasPrefix(ct, "image/") {
		ext = "." + strings.TrimPrefix(strings.SplitN(ct, ";", 2)[0], "image/")
	}
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("ai-agent-url-%d%s", time.Now().UnixNano(), ext))
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		l.WithError(err).Warn("[Execute] save temp file failed, skipping")
		return nil
	}
	f.StoragePath = tmpPath

	if fileType == model.FileTypeText || fileType == model.FileTypeDocument {
		text, err := parser.ExtractText(ct, bytes.NewReader(data))
		if err != nil {
			l.WithError(err).Warn("[Execute] extract text from URL failed, using raw")
			text = string(data)
			if len(text) > 50*1024 {
				text = text[:50*1024]
			}
		}
		f.TextContent = text
	}

	l.WithFields(log.Fields{"filename": filename, "type": string(fileType), "size": len(data)}).Info("[Execute] remote file loaded")
	return f
}

func chatFileTypeToFileType(chatType model.ChatFileType, contentType, filename string) model.FileType {
	switch chatType {
	case model.ChatFileImage:
		return model.FileTypeImage
	case model.ChatFileDocument:
		return model.FileTypeDocument
	case model.ChatFileAudio, model.ChatFileVideo:
		return model.FileTypeDocument
	default:
		return classifyContentType(contentType, filename)
	}
}

func classifyContentType(contentType, filename string) model.FileType {
	ct := strings.ToLower(contentType)
	fn := strings.ToLower(filename)

	if strings.HasPrefix(ct, "image/") {
		return model.FileTypeImage
	}
	docExts := []string{".pdf", ".docx", ".doc", ".xlsx", ".xls", ".pptx", ".ppt"}
	for _, ext := range docExts {
		if strings.HasSuffix(fn, ext) {
			return model.FileTypeDocument
		}
	}
	docTypes := []string{"pdf", "word", "excel", "spreadsheet", "presentation", "officedocument"}
	for _, dt := range docTypes {
		if strings.Contains(ct, dt) {
			return model.FileTypeDocument
		}
	}
	return model.FileTypeText
}

func (e *Executor) loadRequestFiles(ctx context.Context, chatFiles []model.ChatFile, conversationID int64) []*model.File {
	var files []*model.File
	seen := make(map[string]bool)

	for _, cf := range chatFiles {
		switch cf.TransferMethod {
		case model.TransferLocalFile:
			if cf.UploadFileID == "" {
				continue
			}
			f, err := e.store.GetFileByUUID(ctx, cf.UploadFileID)
			if err != nil {
				log.WithField("upload_file_id", cf.UploadFileID).WithError(err).Warn("[Execute] load uploaded file failed, skipping")
				continue
			}
			seen[f.UUID] = true
			files = append(files, f)
		case model.TransferRemoteURL:
			if cf.URL == "" {
				continue
			}
			if seen[cf.URL] {
				continue
			}
			f := e.loadRemoteFile(ctx, cf.URL, cf.Type)
			if f != nil {
				seen[cf.URL] = true
				files = append(files, f)
			}
		}
	}

	if conversationID > 0 {
		convFiles, err := e.store.ListFilesByConversation(ctx, conversationID)
		if err == nil {
			for _, f := range convFiles {
				if !seen[f.UUID] {
					seen[f.UUID] = true
					files = append(files, f)
				}
			}
		}
	}

	if len(files) > 0 {
		names := make([]string, 0, len(files))
		for _, f := range files {
			names = append(names, fmt.Sprintf("%s(%s)", f.Filename, f.FileType))
		}
		log.WithField("files", names).Info("[Execute] files loaded for context")
	}
	return files
}

func (e *Executor) Execute(ctx context.Context, req model.ChatRequest) (*ExecuteResult, error) {
	ag, err := e.store.GetAgentByUUID(ctx, req.AgentID)
	if err != nil {
		log.WithFields(log.Fields{"agent_uuid": req.AgentID}).WithError(err).Error("[Execute] agent not found")
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	prov, err := e.store.GetProvider(ctx, ag.ProviderID)
	if err != nil {
		log.WithFields(log.Fields{"agent": ag.Name, "provider_id": ag.ProviderID}).WithError(err).Error("[Execute] provider not found")
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	l := log.WithFields(log.Fields{"agent": ag.Name, "provider": prov.Name, "model": ag.ModelName})
	l.WithField("user", req.UserID).Info("[Execute] >> start")
	if reqBody, err := json.Marshal(req); err == nil {
		l.WithField("body", string(reqBody)).Debug("[Execute]    request body")
	}

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
	l.WithFields(log.Fields{"conv": conv.UUID}).Debug("[Execute]    conversation ready")

	files := e.loadRequestFiles(ctx, req.Files, conv.ID)

	if len(agentTools) > 0 || len(subAgentTools) > 0 {
		l.Info("[Execute]    mode = tool-augmented")
		return e.executeWithTools(ctx, ag, prov, llmProv, agentTools, subAgentTools, conv, skills, req.Message, tracker, toolSkillMap, files)
	}
	l.Info("[Execute]    mode = simple")
	return e.executeSimple(ctx, ag, prov, llmProv, conv, skills, req.Message, tracker, files)
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

func (e *Executor) executeSimple(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker, files []*model.File) (*ExecuteResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(ag.TimeoutSeconds())*time.Second)
	defer cancel()

	l := log.WithFields(log.Fields{"agent": ag.Name, "conv": conv.UUID})

	history, err := e.memory.LoadHistory(ctx, conv.ID, ag.HistoryLimit())
	if err != nil {
		l.WithError(err).Error("[LLM] load history failed")
		return nil, err
	}

	messages := e.buildMessages(ag, skills, history, userMsg, nil, files)
	logMessages(l, messages)

	req := openai.ChatCompletionRequest{
		Model:    ag.ModelName,
		Messages: messages,
	}
	if ag.Temperature > 0 {
		req.Temperature = float32(ag.Temperature)
	}
	if ag.MaxTokens > 0 {
		req.MaxCompletionTokens = ag.MaxTokens
	}

	userMsgID, err := e.memory.SaveMessage(ctx, conv.ID, "user", userMsg, 0)
	if err != nil {
		l.WithError(err).Error("[LLM] save user message failed")
		return nil, err
	}
	e.linkFilesToMessage(ctx, files, conv.ID, userMsgID)

	l.WithFields(log.Fields{"model": ag.ModelName, "temperature": ag.Temperature, "max_completion_tokens": ag.MaxTokens}).Info("[LLM] >> call")
	start := time.Now()
	resp, err := llmProv.CreateChatCompletion(ctx, req)
	duration := time.Since(start)

	if err != nil {
		l.WithFields(log.Fields{"duration": duration}).WithError(err).Error("[LLM] << failed")
		tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, "", model.StepError, err.Error(), duration, 0, &model.StepMetadata{
			Provider:    prov.Name,
			Model:       ag.ModelName,
			Temperature: ag.Temperature,
		})
		return nil, fmt.Errorf("generate content: %w", err)
	}

	content := extractContent(resp)
	tokensUsed := resp.Usage.TotalTokens
	l.WithFields(log.Fields{"duration": duration, "len": len(content), "tokens": tokensUsed, "preview": truncateLog(content, 200)}).Info("[LLM] << ok")

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        content,
		TokensUsed:     tokensUsed,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[Execute] save assistant message failed")
		return nil, err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, content, model.StepSuccess, "", duration, tokensUsed, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithFields(log.Fields{"msg_id": assistantMsg.ID, "duration": duration}).Info("[Execute] << done")
	return &ExecuteResult{
		ConversationID: conv.UUID,
		Content:        content,
		TokensUsed:     tokensUsed,
		Steps:          tracker.Steps(),
	}, nil
}

func (e *Executor) executeWithTools(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, agentTools []model.Tool, subAgentTools []Tool, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker, toolSkillMap map[string]string, files []*model.File) (*ExecuteResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(ag.TimeoutSeconds())*time.Second)
	defer cancel()

	l := log.WithFields(log.Fields{"agent": ag.Name, "conv": conv.UUID})

	history, err := e.memory.LoadHistory(ctx, conv.ID, ag.HistoryLimit())
	if err != nil {
		l.WithError(err).Error("[LLM] load history failed")
		return nil, err
	}

	userMsgID, err := e.memory.SaveMessage(ctx, conv.ID, "user", userMsg, 0)
	if err != nil {
		l.WithError(err).Error("[LLM] save user message failed")
		return nil, err
	}
	e.linkFilesToMessage(ctx, files, conv.ID, userMsgID)

	lcTools := e.registry.BuildTrackedTools(agentTools, tracker, toolSkillMap)
	lcTools = append(lcTools, subAgentTools...)
	toolMap := make(map[string]Tool, len(lcTools))
	allToolNames := make([]string, 0, len(lcTools))
	for _, t := range lcTools {
		toolMap[t.Name()] = t
		allToolNames = append(allToolNames, t.Name())
	}

	toolDefs := buildLLMToolDefs(agentTools, subAgentTools)

	messages := e.buildMessages(ag, skills, history, userMsg, allToolNames, files)
	logMessages(l, messages)

	req := openai.ChatCompletionRequest{
		Model: ag.ModelName,
		Tools: toolDefs,
	}
	if ag.Temperature > 0 {
		req.Temperature = float32(ag.Temperature)
	}
	if ag.MaxTokens > 0 {
		req.MaxCompletionTokens = ag.MaxTokens
	}

	maxIterations := ag.IterationLimit()
	var finalContent string
	var totalTokens int
	calledTools := make(map[string]bool)
	totalStart := time.Now()

	for i := range maxIterations {
		req.Messages = messages
		l.WithFields(log.Fields{"round": i + 1, "model": ag.ModelName}).Info("[LLM] >> call")
		iterStart := time.Now()
		resp, err := llmProv.CreateChatCompletion(ctx, req)
		iterDur := time.Since(iterStart)

		if err != nil {
			l.WithFields(log.Fields{"round": i + 1, "duration": iterDur}).WithError(err).Error("[LLM] << failed")
			tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, "", model.StepError, err.Error(), iterDur, 0, &model.StepMetadata{
				Provider:    prov.Name,
				Model:       ag.ModelName,
				Temperature: ag.Temperature,
			})
			return nil, fmt.Errorf("generate content: %w", err)
		}

		totalTokens += resp.Usage.TotalTokens

		if len(resp.Choices) == 0 {
			l.Warn("[LLM] << empty response")
			break
		}

		choice := resp.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			finalContent = choice.Message.Content
			l.WithFields(log.Fields{
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
		l.WithFields(log.Fields{"round": i + 1, "duration": iterDur, "tool_calls": tcNames}).Info("[LLM] << tool calls requested")

		messages = append(messages, choice.Message)

		var pendingParts []openai.ChatMessagePart
		for _, tc := range choice.Message.ToolCalls {
			toolName := tc.Function.Name
			toolArgs := tc.Function.Arguments

			tool, ok := toolMap[toolName]
			if !ok {
				errMsg := fmt.Sprintf("tool %q not found", toolName)
				l.WithField("tool", toolName).Warn("[Tool] tool not registered, skipping")
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    errMsg,
					ToolCallID: tc.ID,
					Name:       toolName,
				})
				continue
			}

			l.WithFields(log.Fields{"tool": toolName, "args": truncateLog(toolArgs, 200)}).Info("[Tool] >> invoke")
			calledTools[toolName] = true
			callStart := time.Now()
			output, callErr := tool.Call(ctx, toolArgs)
			callDur := time.Since(callStart)
			toolResult := output
			if callErr != nil {
				toolResult = fmt.Sprintf("error: %s", callErr)
				l.WithFields(log.Fields{"tool": toolName, "duration": callDur}).WithError(callErr).Error("[Tool] << failed")
			} else {
				l.WithFields(log.Fields{"tool": toolName, "duration": callDur, "preview": truncateLog(output, 200)}).Info("[Tool] << ok")
			}

			toolMsg, fileParts := e.buildToolResponseParts(ctx, tc.ID, toolName, toolResult, callErr == nil, l)
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

	e.recordUsedSkillSteps(ctx, skills, toolSkillMap, calledTools, tracker)

	totalDuration := time.Since(totalStart)

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        finalContent,
		TokensUsed:     totalTokens,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[Execute] save assistant message failed")
		return nil, err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, finalContent, model.StepSuccess, "", totalDuration, totalTokens, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithFields(log.Fields{"steps": len(tracker.Steps()), "duration": totalDuration, "total_tokens": totalTokens}).Info("[Execute] << done")
	return &ExecuteResult{
		ConversationID: conv.UUID,
		Content:        finalContent,
		TokensUsed:     totalTokens,
		Steps:          tracker.Steps(),
	}, nil
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

func (e *Executor) ExecuteStream(ctx context.Context, req model.ChatRequest, chunkHandler func(chunk model.StreamChunk) error) error {
	ag, err := e.store.GetAgentByUUID(ctx, req.AgentID)
	if err != nil {
		log.WithField("agent_uuid", req.AgentID).WithError(err).Error("[Execute] agent not found")
		return fmt.Errorf("agent not found: %w", err)
	}

	prov, err := e.store.GetProvider(ctx, ag.ProviderID)
	if err != nil {
		log.WithFields(log.Fields{"agent": ag.Name, "provider_id": ag.ProviderID}).WithError(err).Error("[Execute] provider not found")
		return fmt.Errorf("provider not found: %w", err)
	}

	l := log.WithFields(log.Fields{"agent": ag.Name, "provider": prov.Name, "model": ag.ModelName})
	l.WithField("user", req.UserID).Info("[Execute] >> start (stream)")

	llmProv, err := e.providerFactory(prov, ag.ModelName)
	if err != nil {
		l.WithError(err).Error("[Execute] create llm provider failed")
		return fmt.Errorf("create llm provider: %w", err)
	}

	skills, err := e.store.GetAgentSkills(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Execute] get skills failed")
		return fmt.Errorf("get agent skills: %w", err)
	}

	agentTools, toolSkillMap, err := e.collectTools(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Execute] collect tools failed")
		return err
	}

	isNewConv := req.ConversationID == ""
	conv, err := e.memory.GetOrCreateConversation(ctx, req.ConversationID, ag.ID, req.UserID)
	if err != nil {
		l.WithError(err).Error("[Execute] get/create conversation failed")
		return fmt.Errorf("get conversation: %w", err)
	}
	if isNewConv {
		e.memory.AutoSetTitle(ctx, conv.ID, req.Message)
	}

	tracker := NewStepTracker(e.store, conv.ID)
	subAgentTools := e.buildSubAgentTools(ctx, ag.ID, tracker)

	logResourceSummary(l, agentTools, skills, subAgentTools)

	files := e.loadRequestFiles(ctx, req.Files, conv.ID)

	if len(agentTools) > 0 || len(subAgentTools) > 0 {
		l.Info("[Execute]    mode = stream + tool-augmented")
		convUUID := conv.UUID
		tracker.SetOnStep(func(step model.ExecutionStep) {
			_ = chunkHandler(model.StreamChunk{
				ConversationID: convUUID,
				Step:           &step,
			})
		})
		return e.streamWithTools(ctx, ag, prov, llmProv, agentTools, subAgentTools, conv, skills, req.Message, tracker, toolSkillMap, chunkHandler, files)
	}

	l.Info("[Execute]    mode = stream")
	l = l.WithField("conv", conv.UUID)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(ag.TimeoutSeconds())*time.Second)
	defer cancel()

	history, err := e.memory.LoadHistory(ctx, conv.ID, ag.HistoryLimit())
	if err != nil {
		l.WithError(err).Error("[LLM] load history failed")
		return err
	}

	messages := e.buildMessages(ag, skills, history, req.Message, nil, files)
	logMessages(l, messages)

	apiReq := openai.ChatCompletionRequest{
		Model:    ag.ModelName,
		Messages: messages,
		Stream:   true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}
	if ag.Temperature > 0 {
		apiReq.Temperature = float32(ag.Temperature)
	}
	if ag.MaxTokens > 0 {
		apiReq.MaxCompletionTokens = ag.MaxTokens
	}

	userMsgID, err := e.memory.SaveMessage(ctx, conv.ID, "user", req.Message, 0)
	if err != nil {
		l.WithError(err).Error("[LLM] save user message failed")
		return err
	}
	e.linkFilesToMessage(ctx, files, conv.ID, userMsgID)

	l.WithFields(log.Fields{"model": ag.ModelName, "temperature": ag.Temperature, "max_completion_tokens": ag.MaxTokens}).Info("[LLM] >> call (stream)")
	start := time.Now()
	stream, err := llmProv.CreateChatCompletionStream(ctx, apiReq)
	if err != nil {
		duration := time.Since(start)
		l.WithFields(log.Fields{"duration": duration}).WithError(err).Error("[LLM] << stream create failed")
		tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, req.Message, "", model.StepError, err.Error(), duration, 0, &model.StepMetadata{
			Provider:    prov.Name,
			Model:       ag.ModelName,
			Temperature: ag.Temperature,
		})
		return fmt.Errorf("stream content: %w", err)
	}
	defer stream.Close()

	var fullContent strings.Builder
	var chunkCount int
	var streamTokens int

	for {
		response, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			duration := time.Since(start)
			l.WithFields(log.Fields{"duration": duration, "chunks": chunkCount}).WithError(recvErr).Error("[LLM] << stream recv failed")
			tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, req.Message, "", model.StepError, recvErr.Error(), duration, 0, &model.StepMetadata{
				Provider:    prov.Name,
				Model:       ag.ModelName,
				Temperature: ag.Temperature,
			})
			return fmt.Errorf("stream content: %w", recvErr)
		}
		if response.Usage != nil {
			streamTokens = response.Usage.TotalTokens
		}
		if len(response.Choices) == 0 {
			continue
		}
		delta := response.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		chunkCount++
		fullContent.WriteString(delta)
		if err := chunkHandler(model.StreamChunk{
			ConversationID: conv.UUID,
			Delta:          delta,
		}); err != nil {
			return err
		}
	}

	duration := time.Since(start)
	content := fullContent.String()
	l.WithFields(log.Fields{"duration": duration, "chunks": chunkCount, "tokens": streamTokens, "len": len(content), "preview": truncateLog(content, 200)}).Info("[LLM] << ok")

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        content,
		TokensUsed:     streamTokens,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[Execute] save assistant message failed")
		return err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, req.Message, content, model.StepSuccess, "", duration, streamTokens, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithField("duration", duration).Info("[Execute] << done")
	return chunkHandler(model.StreamChunk{
		ConversationID: conv.UUID,
		Done:           true,
		Steps:          tracker.Steps(),
	})
}

func (e *Executor) streamWithTools(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, agentTools []model.Tool, subAgentTools []Tool, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker, toolSkillMap map[string]string, chunkHandler func(chunk model.StreamChunk) error, files []*model.File) error {
	result, err := e.executeWithTools(ctx, ag, prov, llmProv, agentTools, subAgentTools, conv, skills, userMsg, tracker, toolSkillMap, files)
	if err != nil {
		return err
	}

	runes := []rune(result.Content)
	const runeChunkSize = 50
	for i := 0; i < len(runes); i += runeChunkSize {
		end := min(i+runeChunkSize, len(runes))
		if err := chunkHandler(model.StreamChunk{
			ConversationID: conv.UUID,
			Delta:          string(runes[i:end]),
		}); err != nil {
			return err
		}
	}

	return chunkHandler(model.StreamChunk{
		ConversationID: conv.UUID,
		Done:           true,
	})
}

func (e *Executor) recordUsedSkillSteps(ctx context.Context, skills []model.Skill, toolSkillMap map[string]string, calledTools map[string]bool, tracker *StepTracker) {
	usedSkills := make(map[string]bool)
	for toolName := range calledTools {
		if skillName, ok := toolSkillMap[toolName]; ok {
			usedSkills[skillName] = true
		}
	}

	for _, sk := range skills {
		if !usedSkills[sk.Name] {
			continue
		}

		var calledToolNames []string
		for toolName, skillName := range toolSkillMap {
			if skillName == sk.Name && calledTools[toolName] {
				calledToolNames = append(calledToolNames, toolName)
			}
		}

		input := sk.Instruction
		if input == "" {
			input = "(no instruction)"
		}
		output := fmt.Sprintf("used %d tools: %s", len(calledToolNames), strings.Join(calledToolNames, ", "))

		tracker.RecordStep(ctx, model.StepSkillMatch, sk.Name, input, output, model.StepSuccess, "", 0, 0, &model.StepMetadata{
			SkillName:  sk.Name,
			SkillTools: calledToolNames,
		})

		log.WithFields(log.Fields{
			"skill":      sk.Name,
			"used_tools": calledToolNames,
		}).Info("[Skill] skill used")
	}
}

func (e *Executor) buildToolResponseParts(ctx context.Context, toolCallID, toolName, toolResult string, ok bool, l *log.Entry) (openai.ChatCompletionMessage, []openai.ChatMessagePart) {
	toolMsg := func(content string) openai.ChatCompletionMessage {
		return openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    content,
			ToolCallID: toolCallID,
			Name:       toolName,
		}
	}

	if !ok {
		return toolMsg(toolResult), nil
	}

	fr := parseFileResult(toolResult)
	if fr == nil {
		return toolMsg(toolResult), nil
	}

	data, err := os.ReadFile(fr.Path)
	if err != nil {
		l.WithError(err).WithField("path", fr.Path).Warn("[Tool] << read file failed, using text fallback")
		return toolMsg(fr.Description), nil
	}

	l.WithFields(log.Fields{"tool": toolName, "path": fr.Path, "mime": fr.MimeType, "size": len(data)}).Info("[Tool] << attaching file to response")

	if strings.HasPrefix(fr.MimeType, "image/") {
		imgPart := e.imagePartForToolFile(ctx, fr, data)
		return toolMsg(fr.Description), []openai.ChatMessagePart{imgPart}
	}

	content := string(data)
	const maxFileContent = 10_000
	if len(content) > maxFileContent {
		content = content[:maxFileContent] + "\n... (content truncated)"
	}
	return toolMsg(fmt.Sprintf("%s\n\n%s", fr.Description, content)), nil
}

func (e *Executor) imagePartForToolFile(_ context.Context, fr *toolFileResult, data []byte) openai.ChatMessagePart {
	mimeType := fr.MimeType
	if !strings.HasPrefix(mimeType, "image/") {
		if detected := http.DetectContentType(data); strings.HasPrefix(detected, "image/") {
			mimeType = detected
		} else {
			mimeType = "image/png"
		}
	}
	dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	log.WithFields(log.Fields{"file": filepath.Base(fr.Path), "mime": mimeType, "size": len(data)}).Debug("[Execute] attaching tool image via base64")
	return openai.ChatMessagePart{
		Type:     openai.ChatMessagePartTypeImageURL,
		ImageURL: &openai.ChatMessageImageURL{URL: dataURL},
	}
}

func (e *Executor) buildMessages(ag *model.Agent, skills []model.Skill, history []openai.ChatCompletionMessage, userMsg string, toolNames []string, files []*model.File) []openai.ChatCompletionMessage {
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
			part, err := e.imagePartForFile(img)
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

func extractContent(resp openai.ChatCompletionResponse) string {
	if len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}

func (e *Executor) linkFilesToMessage(ctx context.Context, files []*model.File, conversationID, messageID int64) {
	for _, f := range files {
		if f.ID == 0 {
			continue
		}
		if err := e.store.LinkFileToMessage(ctx, f.ID, conversationID, messageID); err != nil {
			log.WithFields(log.Fields{"file": f.Filename, "msg_id": messageID}).WithError(err).Warn("[Execute] link file to message failed")
		}
	}
}

func (e *Executor) imagePartForFile(f *model.File) (openai.ChatMessagePart, error) {
	data, err := os.ReadFile(f.StoragePath)
	if err != nil {
		return openai.ChatMessagePart{}, err
	}
	mimeType := f.ContentType
	if !strings.HasPrefix(mimeType, "image/") {
		if detected := http.DetectContentType(data); strings.HasPrefix(detected, "image/") {
			mimeType = detected
		} else {
			mimeType = "image/png"
		}
	}
	dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	log.WithFields(log.Fields{"file": f.Filename, "mime": mimeType, "size": len(data)}).Debug("[Execute] attaching image via base64")
	return openai.ChatMessagePart{
		Type:     openai.ChatMessagePartTypeImageURL,
		ImageURL: &openai.ChatMessageImageURL{URL: dataURL},
	}, nil
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

var _ Tool = (*subAgentTool)(nil)

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

func (e *Executor) buildSubAgentTools(ctx context.Context, agentID int64, tracker *StepTracker) []Tool {
	children, err := e.store.GetAgentChildren(ctx, agentID)
	if err != nil || len(children) == 0 {
		return nil
	}
	result := make([]Tool, 0, len(children))
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

func logResourceSummary(l *log.Entry, agentTools []model.Tool, skills []model.Skill, subAgentTools []Tool) {
	toolNames := make([]string, 0, len(agentTools))
	for _, t := range agentTools {
		toolNames = append(toolNames, t.Name)
	}
	skillNames := make([]string, 0, len(skills))
	for _, s := range skills {
		skillNames = append(skillNames, s.Name)
	}
	subNames := make([]string, 0, len(subAgentTools))
	for _, s := range subAgentTools {
		subNames = append(subNames, s.Name())
	}
	l.WithFields(log.Fields{
		"tools":      toolNames,
		"skills":     skillNames,
		"sub_agents": subNames,
	}).Info("[Execute]    resources loaded")

	for _, sk := range skills {
		fields := log.Fields{"skill": sk.Name, "has_instruction": sk.Instruction != ""}
		if sk.Instruction != "" {
			fields["instruction_len"] = len(sk.Instruction)
		}
		l.WithFields(fields).Debug("[Execute]    skill detail")
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
