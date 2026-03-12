package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/memory"
	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/provider"
	"github.com/chowyu12/go-ai-agent/internal/skill"
	"github.com/chowyu12/go-ai-agent/internal/store"
	"github.com/chowyu12/go-ai-agent/internal/tool"
	"github.com/chowyu12/go-ai-agent/internal/tool/mcp"
	"github.com/chowyu12/go-ai-agent/internal/workspace"
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

func WithLongTermMemory(ltm *memory.LongTermMemory) ExecutorOption {
	return func(e *Executor) { e.longMem = ltm }
}

type Executor struct {
	store           store.Store
	registry        *tool.Registry
	convMem         *memory.Manager
	longMem         *memory.LongTermMemory
	providerFactory ProviderFactory
}

func NewExecutor(s store.Store, registry *tool.Registry, opts ...ExecutorOption) *Executor {
	e := &Executor{
		store:           s,
		registry:        registry,
		convMem:         memory.NewManager(s, s),
		longMem:         memory.NewLongTermMemory(memory.NewInMemoryStore()),
		providerFactory: provider.NewFromProvider,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type execContext struct {
	ag      *model.Agent
	prov    *model.Provider
	llmProv provider.LLMProvider
	conv    *model.Conversation
	skills  []model.Skill
	tracker *tool.StepTracker
	files   []*model.File
	userMsg string
	l       *log.Entry

	agentTools []model.Tool
	mcpTools   []tool.Tool
	skillTools    []tool.Tool
	mcpManager    *mcp.Manager
	toolSkillMap  map[string]string
}

func (ec *execContext) hasTools() bool {
	return len(ec.agentTools) > 0 || len(ec.mcpTools) > 0 || len(ec.skillTools) > 0
}

func (ec *execContext) closeMCP() {
	if ec.mcpManager != nil {
		ec.mcpManager.Close()
	}
}

func (ec *execContext) stepMeta() *model.StepMetadata {
	return &model.StepMetadata{
		Provider:    ec.prov.Name,
		Model:       ec.ag.ModelName,
		Temperature: ec.ag.Temperature,
	}
}

func (e *Executor) prepare(ctx context.Context, req model.ChatRequest) (*execContext, error) {
	ag, err := e.store.GetAgentByUUID(ctx, req.AgentID)
	if err != nil {
		log.WithField("agent_uuid", req.AgentID).WithError(err).Error("[Prepare] agent not found")
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	prov, err := e.store.GetProvider(ctx, ag.ProviderID)
	if err != nil {
		log.WithFields(log.Fields{"agent": ag.Name, "provider_id": ag.ProviderID}).WithError(err).Error("[Prepare] provider not found")
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	l := log.WithFields(log.Fields{"agent": ag.Name, "provider": prov.Name, "model": ag.ModelName})

	llmProv, err := e.providerFactory(prov, ag.ModelName)
	if err != nil {
		l.WithError(err).Error("[Prepare] create llm provider failed")
		return nil, fmt.Errorf("create llm provider: %w", err)
	}

	agentTools, toolSkillMap, err := e.collectTools(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Prepare] collect tools failed")
		return nil, err
	}

	skills, err := e.store.GetAgentSkills(ctx, ag.ID)
	if err != nil {
		l.WithError(err).Error("[Prepare] get skills failed")
		return nil, fmt.Errorf("get agent skills: %w", err)
	}

	isNewConv := req.ConversationID == ""
	conv, err := e.convMem.GetOrCreateConversation(ctx, req.ConversationID, ag.ID, req.UserID)
	if err != nil {
		l.WithError(err).Error("[Prepare] get/create conversation failed")
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	if isNewConv {
		e.convMem.AutoSetTitle(ctx, conv.ID, req.Message)
	}

	tracker := tool.NewStepTracker(e.store, conv.ID)

	mcpManager, mcpTools := e.connectMCPServers(ctx, ag.ID, tracker, toolSkillMap)
	skillTools := e.buildSkillManifestTools(skills, tracker, toolSkillMap)

	logResourceSummary(l, agentTools, skills)

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
		agentTools: agentTools,
		mcpTools:   mcpTools,
		skillTools:    skillTools,
		mcpManager:    mcpManager,
		toolSkillMap:  toolSkillMap,
	}, nil
}

func (e *Executor) connectMCPServers(ctx context.Context, agentID int64, tracker *tool.StepTracker, toolSkillMap map[string]string) (*mcp.Manager, []tool.Tool) {
	servers, err := e.store.GetAgentMCPServers(ctx, agentID)
	if err != nil {
		log.WithError(err).Warn("[MCP] get agent mcp servers failed")
		return nil, nil
	}
	if len(servers) == 0 {
		return nil, nil
	}

	mgr := mcp.NewManager()
	if err := mgr.Connect(ctx, servers); err != nil {
		log.WithError(err).Warn("[MCP] connect failed")
		return nil, nil
	}
	if !mgr.HasTools() {
		mgr.Close()
		return nil, nil
	}

	infos := mgr.Tools()
	mcpTools := make([]tool.Tool, 0, len(infos))
	for _, info := range infos {
		info := info
		toolSkillMap[info.Name] = "mcp:" + info.ServerName
		base := &tool.DynamicTool{
			ToolName: info.Name,
			ToolDesc: info.Description,
			Params:   info.Parameters,
			Handler: func(ctx context.Context, input string) (string, error) {
				return mgr.CallTool(ctx, info.Name, input)
			},
		}
		mcpTools = append(mcpTools, &tool.TrackedTool{
			BaseTool:  base,
			ToolName:  info.Name,
			SkillName: "mcp:" + info.ServerName,
			Tracker:   tracker,
		})
	}
	log.WithField("count", len(mcpTools)).Info("[MCP] tools loaded")
	return mgr, mcpTools
}

func (e *Executor) buildSkillManifestTools(skills []model.Skill, tracker *tool.StepTracker, toolSkillMap map[string]string) []tool.Tool {
	var result []tool.Tool
	for _, sk := range skills {
		if !sk.Enabled || len(sk.ToolDefs) == 0 {
			continue
		}
		var toolDefs []model.SkillManifestTool
		if err := json.Unmarshal(sk.ToolDefs, &toolDefs); err != nil {
			log.WithError(err).WithField("skill", sk.Name).Warn("[Skill] parse tool_defs failed")
			continue
		}
		for _, td := range toolDefs {
			td := td
			toolSkillMap[td.Name] = sk.Name
			var handler func(ctx context.Context, input string) (string, error)

			if sk.MainFile != "" && sk.DirName != "" {
				skillDir := workspace.SkillDir(sk.DirName)
				if skillDir != "" {
					mainFile := sk.MainFile
					handler = func(ctx context.Context, input string) (string, error) {
						return skill.RunTool(ctx, skillDir, mainFile, td.Name, input, nil, 0)
					}
				}
			}
			if handler == nil {
				instruction := sk.Instruction
				handler = func(_ context.Context, input string) (string, error) {
					return fmt.Sprintf("[skill:%s] 请根据技能指令处理。输入: %s\n指令: %s", sk.Name, input, instruction), nil
				}
			}

			base := &tool.DynamicTool{
				ToolName: td.Name,
				ToolDesc: td.Description,
				Params:   td.Parameters,
				Handler:  handler,
			}
			result = append(result, &tool.TrackedTool{
				BaseTool:  base,
				ToolName:  td.Name,
				SkillName: sk.Name,
				Tracker:   tracker,
			})
		}
		log.WithFields(log.Fields{"skill": sk.Name, "manifest_tools": len(toolDefs)}).Debug("[Prepare]    skill manifest tools loaded")
	}
	return result
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
			log.WithFields(log.Fields{"skill": sk.Name, "tools": names}).Debug("[Prepare]    skill contributed tools")
		}
		agentTools = append(agentTools, skillTools...)
	}
	return agentTools, toolSkillMap, nil
}

func (e *Executor) saveResult(ctx context.Context, ec *execContext, content string, tokensUsed int, duration time.Duration) (*ExecuteResult, error) {
	msgID, err := e.convMem.SaveAssistantMessage(ctx, ec.conv.ID, content, tokensUsed)
	if err != nil {
		ec.l.WithError(err).Error("[Agentic] save assistant message failed")
		return nil, err
	}

	ec.tracker.SetMessageID(msgID)
	ec.tracker.RecordStep(ctx, model.StepLLMCall, ec.ag.ModelName, ec.userMsg, content, model.StepSuccess, "", duration, tokensUsed, ec.stepMeta())

	ec.l.WithFields(log.Fields{"msg_id": msgID, "duration": duration, "tokens": tokensUsed}).Info("[Agentic] << done")
	return &ExecuteResult{
		ConversationID: ec.conv.UUID,
		Content:        content,
		TokensUsed:     tokensUsed,
		Steps:          ec.tracker.Steps(),
	}, nil
}
