package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/prompt"
	"github.com/chowyu12/go-ai-agent/internal/tool"
)

// ============================================================
//  Agentic 执行：记忆召回 → 构建上下文 → LLM+工具循环 → 记忆提取
//
//  默认执行路径。LLM 自主决定是否调用工具（Function Calling），
//  支持多轮工具调用循环，每轮结果追加到上下文继续推理。
// ============================================================

func (e *Executor) executeAgentic(ctx context.Context, ec *execContext) (*ExecuteResult, error) {
	if t := ec.ag.TimeoutSeconds(); t > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(t)*time.Second)
		defer cancel()
	}

	totalStart := time.Now()
	var totalTokens int

	if _, err := e.convMem.SaveUserMessage(ctx, ec.conv.ID, ec.userMsg, ec.files); err != nil {
		return nil, err
	}

	// ── 记忆召回（仅 store 查询，无 LLM 调用） ──
	memories := e.longMem.Recall(ctx, ec.ag.ID, ec.conv.UserID, ec.userMsg, 5)
	if len(memories) > 0 {
		ec.tracker.RecordStep(ctx, model.StepMemoryRecall, "long_term_memory",
			ec.userMsg, formatMemories(memories), model.StepSuccess, "", 0, 0, nil)
		ec.l.WithField("count", len(memories)).Info("[Agentic] memories recalled")
	}

	// ── 构建上下文：历史消息 + System Prompt + 技能指令 + 文件 + 记忆 ──
	history, err := e.convMem.LoadHistory(ctx, ec.conv.ID, ec.ag.HistoryLimit())
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}
	messages := prompt.BuildMessages(ec.ag, ec.skills, history, ec.userMsg, ec.agentTools, ec.toolSkillMap, ec.files)

	if len(memories) > 0 {
		injectMemories(messages, memories)
	}

	toolMap, toolDefs := e.buildAgenticToolMap(ec)

	// ── LLM + 工具调用循环（LLM 自主决定是否调用工具） ──
	iterLimit := ec.ag.IterationLimit()
	var finalContent string

	for i := range iterLimit {
		req := openai.ChatCompletionRequest{
			Model:    ec.ag.ModelName,
			Messages: messages,
			Tools:    toolDefs,
		}
		applyModelCaps(&req, ec.ag, ec.l)

		ec.l.WithField("iter", i+1).Debug("[Agentic] LLM call")
		resp, llmErr := ec.llmProv.CreateChatCompletion(ctx, req)
		if llmErr != nil {
			return nil, fmt.Errorf("llm call: %w", llmErr)
		}
		totalTokens += resp.Usage.TotalTokens

		if len(resp.Choices) == 0 {
			break
		}
		choice := resp.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			finalContent = choice.Message.Content
			break
		}

		messages = append(messages, choice.Message)
		for _, tc := range choice.Message.ToolCalls {
			t, ok := toolMap[tc.Function.Name]
			if !ok {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    fmt.Sprintf("tool %q not found", tc.Function.Name),
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
				continue
			}
			output, callErr := t.Call(ctx, tc.Function.Arguments)
			content := output
			if callErr != nil {
				content = "error: " + callErr.Error()
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    content,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})
		}
	}

	// ── 异步提取长期记忆（后台 goroutine，不阻塞响应） ──
	go e.longMem.ExtractAndStore(context.WithoutCancel(ctx), ec.llmProv, ec.ag.ModelName,
		ec.ag.ID, ec.conv.UserID, ec.conv.UUID, ec.userMsg, finalContent)

	return e.saveResult(ctx, ec, finalContent, totalTokens, time.Since(totalStart))
}

// injectMemories 将记忆内容追加到 system prompt 中。
func injectMemories(messages []openai.ChatCompletionMessage, memories []model.MemoryEntry) {
	memBlock := "\n\n## 相关记忆\n\n" + formatMemories(memories)
	if len(messages) > 0 && messages[0].Role == openai.ChatMessageRoleSystem {
		messages[0].Content += memBlock
	}
}

// ============================================================
//  流式执行：复用 executeAgentic，工具调用步骤通过 tracker 实时推送，
//  最终回答作为完整 chunk 一次性发送。
// ============================================================

func (e *Executor) streamAgentic(ctx context.Context, ec *execContext, chunkHandler func(chunk model.StreamChunk) error) error {
	result, err := e.executeAgentic(ctx, ec)
	if err != nil {
		return err
	}
	if err := chunkHandler(model.StreamChunk{
		ConversationID: ec.conv.UUID,
		Delta:          result.Content,
	}); err != nil {
		return err
	}
	return chunkHandler(model.StreamChunk{
		ConversationID: ec.conv.UUID,
		Done:           true,
	})
}

// buildAgenticToolMap 构建工具映射和 LLM 工具定义。
// 合并三类工具：Agent 关联的 DB 工具、MCP 远程工具、技能 manifest 工具。
func (e *Executor) buildAgenticToolMap(ec *execContext) (map[string]tool.Tool, []openai.Tool) {
	if !ec.hasTools() {
		return nil, nil
	}
	lcTools := e.registry.BuildTrackedTools(ec.agentTools, ec.tracker, ec.toolSkillMap)
	lcTools = append(lcTools, ec.mcpTools...)
	lcTools = append(lcTools, ec.skillTools...)
	toolMap := make(map[string]tool.Tool, len(lcTools))
	for _, t := range lcTools {
		toolMap[t.Name()] = t
	}
	return toolMap, prompt.BuildLLMToolDefs(ec.agentTools, ec.mcpTools, ec.skillTools)
}

func collectToolDescriptions(ec *execContext) []string {
	var descs []string
	for _, t := range ec.agentTools {
		if t.Enabled {
			desc := t.Description
			if desc == "" {
				desc = t.Name
			}
			descs = append(descs, fmt.Sprintf("- %s: %s", t.Name, desc))
		}
	}
	for _, t := range ec.mcpTools {
		descs = append(descs, fmt.Sprintf("- %s: %s", t.Name(), t.Description()))
	}
	for _, t := range ec.skillTools {
		descs = append(descs, fmt.Sprintf("- %s: %s", t.Name(), t.Description()))
	}
	return descs
}

func formatMemories(memories []model.MemoryEntry) string {
	var sb strings.Builder
	for _, m := range memories {
		fmt.Fprintf(&sb, "- [%s|重要度%d] %s\n", m.Category, m.Importance, m.Content)
	}
	return sb.String()
}
