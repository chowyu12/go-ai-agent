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

type Reasoner struct{}

func NewReasoner() *Reasoner { return &Reasoner{} }

const thinkSystemPrompt = `你是一个AI推理引擎。执行操作前先进行深度思考和推理。

目标: %s
当前步骤: %s

已完成步骤及结果:
%s

输出你的思考过程(JSON格式):
{
  "reasoning": "详细推理过程：分析当前状态、可用信息、最佳策略",
  "next_action": "建议的下一步操作(工具名称或answer表示直接回答)",
  "action_input": "操作的输入描述",
  "confidence": 4
}

confidence说明: 1=极不确定 2=较不确定 3=一般 4=较确定 5=非常确定
只输出JSON。`

func (r *Reasoner) Think(ctx context.Context, llm provider.LLMProvider, modelName, goal string, step *model.PlanStep, completed []model.PlanStep) (*model.Thought, error) {
	var history strings.Builder
	if len(completed) == 0 {
		history.WriteString("（尚无已完成步骤）")
	} else {
		for _, s := range completed {
			fmt.Fprintf(&history, "- 步骤%d [%s]: %s → %s\n",
				s.ID, s.Status, s.Description, tool.Truncate(s.Result, 200))
		}
	}

	systemPrompt := fmt.Sprintf(thinkSystemPrompt, goal, step.Description, history.String())

	content, err := provider.Complete(ctx, llm, modelName, systemPrompt, "请分析并思考")
	if err != nil {
		return &model.Thought{
			Reasoning:  "思考过程出错，将直接执行当前步骤",
			NextAction: "continue",
			Confidence: 2,
		}, nil
	}

	thought, err := parseThought(content)
	if err != nil {
		return &model.Thought{
			Reasoning:  content,
			NextAction: "continue",
			Confidence: 3,
		}, nil
	}
	return thought, nil
}

func parseThought(raw string) (*model.Thought, error) {
	cleaned := provider.ExtractJSON(raw)
	var t model.Thought
	if err := json.Unmarshal([]byte(cleaned), &t); err != nil {
		return nil, err
	}
	return &t, nil
}
