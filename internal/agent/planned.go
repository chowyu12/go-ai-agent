package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/chowyu12/go-ai-agent/internal/brain"
	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/tool"
)

const (
	maxPlanRevisions  = 3
	maxStepIterations = 5
)

// executePlanned 使用完整的 Plan → Think → Act → Reflect 认知循环。
// 当前未启用，默认执行路径为 executeAgentic（直接 LLM+工具循环）。
// 保留供未来复杂多步骤任务场景按需启用。
func (e *Executor) executePlanned(ctx context.Context, ec *execContext) (*ExecuteResult, error) {
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

	toolMap, toolDefs := e.buildAgenticToolMap(ec)
	planner := brain.NewPlanner()
	reasoner := brain.NewReasoner()
	reflector := brain.NewReflector()

	memories := e.longMem.Recall(ctx, ec.ag.ID, ec.conv.UserID, ec.userMsg, 5)
	if len(memories) > 0 {
		ec.tracker.RecordStep(ctx, model.StepMemoryRecall, "long_term_memory",
			ec.userMsg, formatMemories(memories), model.StepSuccess, "", 0, 0, nil)
	}

	toolDescs := collectToolDescriptions(ec)
	planStart := time.Now()
	plan, err := planner.GeneratePlan(ctx, ec.llmProv, ec.ag.ModelName, ec.userMsg, toolDescs, memories)
	if err != nil {
		return nil, fmt.Errorf("agentic planning: %w", err)
	}
	planJSON, _ := json.MarshalIndent(plan, "", "  ")
	ec.tracker.RecordStep(ctx, model.StepPlanning, "plan_generation",
		ec.userMsg, string(planJSON), model.StepSuccess, "", time.Since(planStart), 0, nil)

	var allResults []plannedStepResult

	for {
		step := plan.NextPendingStep()
		if step == nil {
			break
		}
		plan.UpdateStep(step.ID, model.PlanStepActive, "")

		thinkStart := time.Now()
		thought, _ := reasoner.Think(ctx, ec.llmProv, ec.ag.ModelName,
			plan.Goal.Description, step, plan.CompletedSteps())
		thoughtJSON, _ := json.Marshal(thought)
		ec.tracker.RecordStep(ctx, model.StepThinking, fmt.Sprintf("think_step_%d", step.ID),
			step.Description, string(thoughtJSON), model.StepSuccess, "", time.Since(thinkStart), 0, nil)

		result, tokens, actErr := e.executePlanStep(ctx, ec, plan, step, thought, toolMap, toolDefs)
		totalTokens += tokens

		if actErr != nil {
			plan.UpdateStep(step.ID, model.PlanStepFailed, actErr.Error())
		} else {
			plan.UpdateStep(step.ID, model.PlanStepCompleted, result)
			allResults = append(allResults, plannedStepResult{stepID: step.ID, content: result})
		}

		stepOutput := result
		if actErr != nil {
			stepOutput = actErr.Error()
		}
		refStart := time.Now()
		reflection, _ := reflector.ReflectOnStep(ctx, ec.llmProv, ec.ag.ModelName,
			plan.Goal.Description, step, stepOutput)
		refJSON, _ := json.Marshal(reflection)
		ec.tracker.RecordStep(ctx, model.StepReflection, fmt.Sprintf("reflect_step_%d", step.ID),
			tool.Truncate(stepOutput, 500), string(refJSON), model.StepSuccess, "", time.Since(refStart), 0, nil)

		if reflection.GoalMet {
			break
		}

		if reflection.NeedReplan && plan.Revision < maxPlanRevisions {
			replanStart := time.Now()
			revised, revErr := planner.RevisePlan(ctx, ec.llmProv, ec.ag.ModelName, plan, reflection.Summary)
			if revErr == nil {
				plan = revised
				rpJSON, _ := json.MarshalIndent(plan, "", "  ")
				ec.tracker.RecordStep(ctx, model.StepPlanning, "plan_revision",
					reflection.Summary, string(rpJSON), model.StepSuccess, "", time.Since(replanStart), 0, nil)
			}
		}
	}

	frStart := time.Now()
	finalRef, _ := reflector.ReflectOnPlan(ctx, ec.llmProv, ec.ag.ModelName, plan)
	frJSON, _ := json.Marshal(finalRef)
	ec.tracker.RecordStep(ctx, model.StepReflection, "final_reflection",
		plan.Goal.Description, string(frJSON), model.StepSuccess, "", time.Since(frStart), 0, nil)

	finalAnswer, synthTokens, _ := e.synthesizePlan(ctx, ec, plan, allResults, memories)
	totalTokens += synthTokens

	go e.longMem.ExtractAndStore(context.WithoutCancel(ctx), ec.llmProv, ec.ag.ModelName,
		ec.ag.ID, ec.conv.UserID, ec.conv.UUID, ec.userMsg, finalAnswer)

	return e.saveResult(ctx, ec, finalAnswer, totalTokens, time.Since(totalStart))
}

type plannedStepResult struct {
	stepID  int
	content string
}

func (e *Executor) executePlanStep(ctx context.Context, ec *execContext, plan *model.Plan, step *model.PlanStep, thought *model.Thought, toolMap map[string]tool.Tool, toolDefs []openai.Tool) (string, int, error) {
	stepPrompt := buildStepPrompt(ec.ag.SystemPrompt, plan, step, thought)
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: stepPrompt},
		{Role: openai.ChatMessageRoleUser, Content: step.Description},
	}

	var totalTokens int
	iterLimit := min(maxStepIterations, ec.ag.IterationLimit())

	for i := range iterLimit {
		req := openai.ChatCompletionRequest{
			Model:    ec.ag.ModelName,
			Messages: messages,
			Tools:    toolDefs,
		}
		applyModelCaps(&req, ec.ag, ec.l)

		ec.l.WithFields(map[string]any{"step": step.ID, "iter": i + 1}).Debug("[Planned] step LLM call")
		resp, err := ec.llmProv.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", totalTokens, fmt.Errorf("step %d llm: %w", step.ID, err)
		}
		totalTokens += resp.Usage.TotalTokens

		if len(resp.Choices) == 0 {
			break
		}
		choice := resp.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, totalTokens, nil
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

	return "", totalTokens, nil
}

const synthesizePlanPrompt = `根据任务计划和各步骤执行结果，生成最终回答。

目标: %s

执行结果:
%s
%s
综合所有信息给用户一个完整、准确、有条理的回答。不要提及内部执行过程。`

func (e *Executor) synthesizePlan(ctx context.Context, ec *execContext, plan *model.Plan, results []plannedStepResult, memories []model.MemoryEntry) (string, int, error) {
	if len(results) == 1 && len(plan.Steps) <= 1 {
		return results[0].content, 0, nil
	}
	if len(results) == 0 {
		return "任务执行完成，但未产生有效结果。", 0, nil
	}

	var sb strings.Builder
	for _, r := range results {
		fmt.Fprintf(&sb, "步骤%d:\n%s\n\n", r.stepID, r.content)
	}

	memCtx := ""
	if len(memories) > 0 {
		memCtx = "\n参考记忆:\n" + formatMemories(memories)
	}

	sysPrompt := fmt.Sprintf(synthesizePlanPrompt, plan.Goal.Description, sb.String(), memCtx)
	resp, err := ec.llmProv.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: ec.ag.ModelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
			{Role: openai.ChatMessageRoleUser, Content: ec.userMsg},
		},
	})
	if err != nil {
		return sb.String(), 0, nil
	}
	return extractContent(resp), resp.Usage.TotalTokens, nil
}

func buildStepPrompt(basePrompt string, plan *model.Plan, step *model.PlanStep, thought *model.Thought) string {
	var sb strings.Builder
	if basePrompt != "" {
		sb.WriteString(basePrompt)
		sb.WriteString("\n\n")
	}

	fmt.Fprintf(&sb, "## 当前任务\n\n目标: %s\n当前步骤(#%d): %s\n",
		plan.Goal.Description, step.ID, step.Description)

	if completed := plan.CompletedSteps(); len(completed) > 0 {
		sb.WriteString("\n已完成步骤:\n")
		for _, s := range completed {
			fmt.Fprintf(&sb, "- #%d %s → %s\n", s.ID, s.Description, tool.Truncate(s.Result, 200))
		}
	}

	if thought.Reasoning != "" {
		fmt.Fprintf(&sb, "\n推理: %s\n", thought.Reasoning)
	}

	sb.WriteString("\n请完成当前步骤。需要时调用工具，否则直接给出结果。")
	return sb.String()
}
