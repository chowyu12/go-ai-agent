package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/provider"
)

type Planner struct{}

func NewPlanner() *Planner { return &Planner{} }

const planSystemPrompt = `你是一个任务规划专家。根据用户请求和可用工具，生成结构化执行计划。

可用工具:
%s

以JSON格式输出:
{
  "goal": {
    "description": "目标描述",
    "success_criteria": ["完成标准1", "完成标准2"]
  },
  "steps": [
    {"id": 1, "description": "步骤描述", "tool_hint": "建议工具(可选)", "depends_on": []}
  ]
}

规则:
1. 每步只做一件事，步骤原子化
2. 用depends_on声明步骤间的前置依赖(引用步骤id)
3. 简单任务1-2步，复杂任务不超过6步
4. 只输出JSON`

func (p *Planner) GeneratePlan(ctx context.Context, llm provider.LLMProvider, modelName, userMsg string, toolDescs []string, memories []model.MemoryEntry) (*model.Plan, error) {
	toolList := "无"
	if len(toolDescs) > 0 {
		toolList = strings.Join(toolDescs, "\n")
	}

	systemPrompt := fmt.Sprintf(planSystemPrompt, toolList)

	userPrompt := userMsg
	if len(memories) > 0 {
		var sb strings.Builder
		sb.WriteString("相关历史记忆:\n")
		for _, m := range memories {
			fmt.Fprintf(&sb, "- [%s] %s\n", m.Category, m.Content)
		}
		sb.WriteString("\n用户请求: ")
		sb.WriteString(userMsg)
		userPrompt = sb.String()
	}

	content, err := provider.Complete(ctx, llm, modelName, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
	}

	plan, err := parsePlan(content)
	if err != nil {
		return p.fallbackPlan(userMsg), nil
	}

	plan.Status = model.PlanActive
	for i := range plan.Steps {
		plan.Steps[i].Status = model.PlanStepPending
	}
	return plan, nil
}

const replanSystemPrompt = `你是一个任务规划专家。基于执行反馈，修订当前计划。

当前计划:
%s

反馈:
%s

输出修订后的完整计划(JSON格式同上)。保留已完成步骤(status设为completed)，调整或替换未完成步骤。只输出JSON。`

func (p *Planner) RevisePlan(ctx context.Context, llm provider.LLMProvider, modelName string, current *model.Plan, feedback string) (*model.Plan, error) {
	planJSON, _ := json.MarshalIndent(current, "", "  ")
	systemPrompt := fmt.Sprintf(replanSystemPrompt, string(planJSON), feedback)

	content, err := provider.Complete(ctx, llm, modelName, systemPrompt, "请修订计划")
	if err != nil {
		return current, nil
	}

	revised, err := parsePlan(content)
	if err != nil {
		return current, nil
	}

	revised.Status = model.PlanActive
	revised.Revision = current.Revision + 1
	return revised, nil
}

func (p *Planner) fallbackPlan(userMsg string) *model.Plan {
	return &model.Plan{
		Goal: model.Goal{
			Description:     userMsg,
			SuccessCriteria: []string{"完成用户请求"},
		},
		Steps: []model.PlanStep{
			{ID: 1, Description: userMsg, Status: model.PlanStepPending},
		},
		Status: model.PlanActive,
	}
}

func parsePlan(raw string) (*model.Plan, error) {
	cleaned := provider.ExtractJSON(raw)
	var plan model.Plan
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return nil, err
	}
	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("empty plan")
	}
	return &plan, nil
}
