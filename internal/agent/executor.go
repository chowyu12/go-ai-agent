package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"

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

	if fileType == model.FileTypeImage {
		tmpPath := filepath.Join(os.TempDir(), "ai-agent-url-"+fmt.Sprintf("%d", time.Now().UnixNano()))
		if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
			l.WithError(err).Warn("[Execute] save temp image failed, skipping")
			return nil
		}
		f.StoragePath = tmpPath
	} else {
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
	subAgentLCTools := e.buildSubAgentLCTools(ctx, ag.ID, tracker)

	logResourceSummary(l, agentTools, skills, subAgentLCTools)
	l.WithFields(log.Fields{"conv": conv.UUID}).Debug("[Execute]    conversation ready")

	files := e.loadRequestFiles(ctx, req.Files, conv.ID)

	if len(agentTools) > 0 || len(subAgentLCTools) > 0 {
		l.Info("[Execute]    mode = tool-augmented")
		return e.executeWithTools(ctx, ag, prov, llmProv, agentTools, subAgentLCTools, conv, skills, req.Message, tracker, toolSkillMap, files)
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

	history, err := e.memory.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		l.WithError(err).Error("[LLM] load history failed")
		return nil, err
	}

	messages := e.buildMessages(ag, skills, history, userMsg, nil, files)
	logMessages(l, messages)

	opts := []llms.CallOption{
		llms.WithTemperature(ag.Temperature),
		llms.WithMaxTokens(ag.MaxTokens),
	}

	if err := e.memory.SaveMessage(ctx, conv.ID, "user", userMsg, 0); err != nil {
		l.WithError(err).Error("[LLM] save user message failed")
		return nil, err
	}

	l.WithFields(log.Fields{"model": ag.ModelName, "temperature": ag.Temperature, "max_tokens": ag.MaxTokens}).Info("[LLM] >> call")
	start := time.Now()
	resp, err := llmProv.GenerateContent(ctx, messages, opts...)
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
	l.WithFields(log.Fields{"duration": duration, "len": len(content), "preview": truncateLog(content, 200)}).Info("[LLM] << ok")

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        content,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[Execute] save assistant message failed")
		return nil, err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, content, model.StepSuccess, "", duration, 0, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithFields(log.Fields{"msg_id": assistantMsg.ID, "duration": duration}).Info("[Execute] << done")
	return &ExecuteResult{
		ConversationID: conv.UUID,
		Content:        content,
		Steps:          tracker.Steps(),
	}, nil
}

func (e *Executor) executeWithTools(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, agentTools []model.Tool, subAgentLCTools []tools.Tool, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker, toolSkillMap map[string]string, files []*model.File) (*ExecuteResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(ag.TimeoutSeconds())*time.Second)
	defer cancel()

	l := log.WithFields(log.Fields{"agent": ag.Name, "conv": conv.UUID})

	history, err := e.memory.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		l.WithError(err).Error("[LLM] load history failed")
		return nil, err
	}

	if err := e.memory.SaveMessage(ctx, conv.ID, "user", userMsg, 0); err != nil {
		l.WithError(err).Error("[LLM] save user message failed")
		return nil, err
	}

	lcTools := e.registry.BuildTrackedTools(agentTools, tracker, toolSkillMap)
	lcTools = append(lcTools, subAgentLCTools...)
	toolMap := make(map[string]tools.Tool, len(lcTools))
	allToolNames := make([]string, 0, len(lcTools))
	for _, t := range lcTools {
		toolMap[t.Name()] = t
		allToolNames = append(allToolNames, t.Name())
	}

	llmToolDefs := buildLLMToolDefs(agentTools, subAgentLCTools)

	messages := e.buildMessages(ag, skills, history, userMsg, allToolNames, files)
	logMessages(l, messages)

	opts := []llms.CallOption{
		llms.WithTemperature(ag.Temperature),
		llms.WithMaxTokens(ag.MaxTokens),
		llms.WithTools(llmToolDefs),
	}

	const maxIterations = 10
	var finalContent string
	calledTools := make(map[string]bool)
	totalStart := time.Now()

	for i := range maxIterations {
		l.WithFields(log.Fields{"round": i + 1, "model": ag.ModelName}).Info("[LLM] >> call")
		iterStart := time.Now()
		resp, err := llmProv.GenerateContent(ctx, messages, opts...)
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

		if len(resp.Choices) == 0 {
			l.Warn("[LLM] << empty response")
			break
		}

		choice := resp.Choices[0]

		if len(choice.ToolCalls) == 0 {
			finalContent = choice.Content
			l.WithFields(log.Fields{
				"round":    i + 1,
				"duration": iterDur,
				"len":      len(finalContent),
				"preview":  truncateLog(finalContent, 200),
			}).Info("[LLM] << final answer")
			break
		}

		tcNames := make([]string, 0, len(choice.ToolCalls))
		for _, tc := range choice.ToolCalls {
			tcNames = append(tcNames, tc.FunctionCall.Name)
		}
		l.WithFields(log.Fields{"round": i + 1, "duration": iterDur, "tool_calls": tcNames}).Info("[LLM] << tool calls requested")

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
				l.WithField("tool", toolName).Warn("[Tool] tool not registered, skipping")
				messages = append(messages, llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{ToolCallID: tc.ID, Name: toolName, Content: errMsg},
					},
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

			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{ToolCallID: tc.ID, Name: toolName, Content: toolResult},
				},
			})
		}
	}

	e.recordUsedSkillSteps(ctx, skills, toolSkillMap, calledTools, tracker)

	totalDuration := time.Since(totalStart)

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        finalContent,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[Execute] save assistant message failed")
		return nil, err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, userMsg, finalContent, model.StepSuccess, "", totalDuration, 0, &model.StepMetadata{
		Provider:    prov.Name,
		Model:       ag.ModelName,
		Temperature: ag.Temperature,
	})

	l.WithFields(log.Fields{"steps": len(tracker.Steps()), "duration": totalDuration}).Info("[Execute] << done")
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
	subAgentLCTools := e.buildSubAgentLCTools(ctx, ag.ID, tracker)

	logResourceSummary(l, agentTools, skills, subAgentLCTools)

	files := e.loadRequestFiles(ctx, req.Files, conv.ID)

	if len(agentTools) > 0 || len(subAgentLCTools) > 0 {
		l.Info("[Execute]    mode = stream + tool-augmented")
		convUUID := conv.UUID
		tracker.SetOnStep(func(step model.ExecutionStep) {
			_ = chunkHandler(model.StreamChunk{
				ConversationID: convUUID,
				Step:           &step,
			})
		})
		return e.streamWithTools(ctx, ag, prov, llmProv, agentTools, subAgentLCTools, conv, skills, req.Message, tracker, toolSkillMap, chunkHandler, files)
	}

	l.Info("[Execute]    mode = stream")
	l = l.WithField("conv", conv.UUID)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(ag.TimeoutSeconds())*time.Second)
	defer cancel()

	history, err := e.memory.LoadHistory(ctx, conv.ID, 50)
	if err != nil {
		l.WithError(err).Error("[LLM] load history failed")
		return err
	}

	messages := e.buildMessages(ag, skills, history, req.Message, nil, files)
	logMessages(l, messages)

	opts := []llms.CallOption{
		llms.WithTemperature(ag.Temperature),
		llms.WithMaxTokens(ag.MaxTokens),
	}

	var fullContent strings.Builder
	var chunkCount int
	var utf8Buf []byte

	streamHandler := func(_ context.Context, chunk []byte) error {
		chunkCount++
		if len(utf8Buf) > 0 {
			chunk = append(utf8Buf, chunk...)
			utf8Buf = nil
		}
		if !utf8.Valid(chunk) {
			l.WithField("hex", fmt.Sprintf("%x", chunk)).Debug("[Stream] incomplete UTF-8 chunk, buffering tail")
			i := len(chunk)
			for i > 0 && !utf8.Valid(chunk[:i]) {
				i--
			}
			utf8Buf = append(utf8Buf, chunk[i:]...)
			chunk = chunk[:i]
		}
		if len(chunk) == 0 {
			return nil
		}
		text := string(chunk)
		fullContent.WriteString(text)
		return chunkHandler(model.StreamChunk{
			ConversationID: conv.UUID,
			Delta:          text,
		})
	}

	if err := e.memory.SaveMessage(ctx, conv.ID, "user", req.Message, 0); err != nil {
		l.WithError(err).Error("[LLM] save user message failed")
		return err
	}

	l.WithFields(log.Fields{"model": ag.ModelName, "temperature": ag.Temperature, "max_tokens": ag.MaxTokens}).Info("[LLM] >> call (stream)")
	start := time.Now()
	_, err = llmProv.StreamContent(ctx, messages, streamHandler, opts...)
	duration := time.Since(start)

	if len(utf8Buf) > 0 {
		fullContent.Write(utf8Buf)
		_ = chunkHandler(model.StreamChunk{
			ConversationID: conv.UUID,
			Delta:          string(utf8Buf),
		})
		utf8Buf = nil
	}

	if err != nil {
		l.WithFields(log.Fields{"duration": duration, "chunks": chunkCount}).WithError(err).Error("[LLM] << failed")
		tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, req.Message, "", model.StepError, err.Error(), duration, 0, &model.StepMetadata{
			Provider:    prov.Name,
			Model:       ag.ModelName,
			Temperature: ag.Temperature,
		})
		return fmt.Errorf("stream content: %w", err)
	}

	content := fullContent.String()
	l.WithFields(log.Fields{"duration": duration, "chunks": chunkCount, "len": len(content), "preview": truncateLog(content, 200)}).Info("[LLM] << ok")

	assistantMsg := &model.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        content,
	}
	if err := e.store.CreateMessage(ctx, assistantMsg); err != nil {
		l.WithError(err).Error("[Execute] save assistant message failed")
		return err
	}

	tracker.SetMessageID(assistantMsg.ID)
	tracker.RecordStep(ctx, model.StepLLMCall, ag.ModelName, req.Message, content, model.StepSuccess, "", duration, 0, &model.StepMetadata{
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

func (e *Executor) streamWithTools(ctx context.Context, ag *model.Agent, prov *model.Provider, llmProv provider.LLMProvider, agentTools []model.Tool, subAgentLCTools []tools.Tool, conv *model.Conversation, skills []model.Skill, userMsg string, tracker *StepTracker, toolSkillMap map[string]string, chunkHandler func(chunk model.StreamChunk) error, files []*model.File) error {
	result, err := e.executeWithTools(ctx, ag, prov, llmProv, agentTools, subAgentLCTools, conv, skills, userMsg, tracker, toolSkillMap, files)
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

func (e *Executor) buildMessages(ag *model.Agent, skills []model.Skill, history []llms.MessageContent, userMsg string, toolNames []string, files []*model.File) []llms.MessageContent {
	systemPrompt := buildSystemPrompt(ag, skills, toolNames)

	var messages []llms.MessageContent
	if systemPrompt != "" {
		messages = append(messages, llms.MessageContent{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextContent{Text: systemPrompt}},
		})
	}

	messages = append(messages, history...)

	var parts []llms.ContentPart
	var textFiles []*model.File
	var imageFiles []*model.File
	for _, f := range files {
		if f.IsImage() {
			imageFiles = append(imageFiles, f)
		} else if f.IsTextual() && f.TextContent != "" {
			textFiles = append(textFiles, f)
		}
	}

	if len(textFiles) > 0 {
		var ctx strings.Builder
		ctx.WriteString("以下是用户提供的参考文件内容:\n\n")
		for _, f := range textFiles {
			ctx.WriteString(fmt.Sprintf("--- [文件: %s] ---\n%s\n---\n\n", f.Filename, f.TextContent))
		}
		ctx.WriteString("用户消息: ")
		ctx.WriteString(userMsg)
		parts = append(parts, llms.TextContent{Text: ctx.String()})
	} else {
		parts = append(parts, llms.TextContent{Text: userMsg})
	}

	for _, img := range imageFiles {
		data, err := os.ReadFile(img.StoragePath)
		if err != nil {
			log.WithError(err).WithField("file", img.Filename).Warn("[Execute] read image file failed, skipping")
			continue
		}
		parts = append(parts, llms.BinaryPart(img.ContentType, data))
	}

	messages = append(messages, llms.MessageContent{
		Role:  llms.ChatMessageTypeHuman,
		Parts: parts,
	})

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

func logResourceSummary(l *log.Entry, agentTools []model.Tool, skills []model.Skill, subAgentTools []tools.Tool) {
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

func logMessages(l *log.Entry, messages []llms.MessageContent) {
	for i, msg := range messages {
		var textParts []string
		for _, part := range msg.Parts {
			if tc, ok := part.(llms.TextContent); ok {
				textParts = append(textParts, tc.Text)
			}
		}
		content := strings.Join(textParts, "")
		l.WithFields(log.Fields{
			"idx":  i,
			"role": string(msg.Role),
			"len":  len(content),
			"text": truncateLog(content, 300),
		}).Debug("[LLM]    message")
	}
}
