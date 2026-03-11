package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/provider"
	"github.com/chowyu12/go-ai-agent/internal/tool"
)

type Reflector struct{}

func NewReflector() *Reflector { return &Reflector{} }

const reflectStepSystemPrompt = `你是一个质量评估专家。评估当前步骤的执行结果。

目标: %s
步骤: %s
执行结果: %s

输出评估(JSON格式):
{
  "goal_met": false,
  "quality": 4,
  "issues": ["发现的问题"],
  "suggestions": ["改进建议"],
  "need_replan": false,
  "summary": "简要评估总结"
}

评估标准:
- quality: 1=完全失败 2=差 3=一般 4=良好 5=优秀
- need_replan: 仅在方向性错误或关键步骤失败时设为true
- goal_met: 仅在整体目标已经达成时设为true
只输出JSON。`

func (r *Reflector) ReflectOnStep(ctx context.Context, llm provider.LLMProvider, modelName, goal string, step *model.PlanStep, result string) (*model.Reflection, error) {
	systemPrompt := fmt.Sprintf(reflectStepSystemPrompt,
		goal, step.Description, tool.Truncate(result, 2000))

	content, err := provider.Complete(ctx, llm, modelName, systemPrompt, "请评估")
	if err != nil {
		return defaultReflection(), nil
	}

	ref, err := parseReflection(content)
	if err != nil {
		return defaultReflection(), nil
	}
	return ref, nil
}

const reflectPlanSystemPrompt = `你是一个质量评估专家。对整体任务执行进行最终评估。

目标: %s
完成标准: %s

执行记录:
%s

输出最终评估(JSON格式，同上)。着重评估目标是否达成、整体质量如何。
只输出JSON。`

func (r *Reflector) ReflectOnPlan(ctx context.Context, llm provider.LLMProvider, modelName string, plan *model.Plan) (*model.Reflection, error) {
	criteria := strings.Join(plan.Goal.SuccessCriteria, "; ")

	var history strings.Builder
	for _, s := range plan.Steps {
		fmt.Fprintf(&history, "- 步骤%d [%s]: %s → %s\n",
			s.ID, s.Status, s.Description, tool.Truncate(s.Result, 300))
	}

	systemPrompt := fmt.Sprintf(reflectPlanSystemPrompt,
		plan.Goal.Description, criteria, history.String())

	content, err := provider.Complete(ctx, llm, modelName, systemPrompt, "请做最终评估")
	if err != nil {
		return defaultReflection(), nil
	}

	ref, err := parseReflection(content)
	if err != nil {
		return defaultReflection(), nil
	}
	return ref, nil
}

func defaultReflection() *model.Reflection {
	return &model.Reflection{
		Quality: 3,
		Summary: "评估过程出错，默认通过",
	}
}

func parseReflection(raw string) (*model.Reflection, error) {
	cleaned := provider.ExtractJSON(raw)
	var r model.Reflection
	if err := json.Unmarshal([]byte(cleaned), &r); err != nil {
		return nil, err
	}
	return &r, nil
}
