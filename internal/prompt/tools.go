package prompt

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/tool"
)

func BuildLLMToolDefs(modelTools []model.Tool, mcpTools []tool.Tool, skillTools []tool.Tool) []openai.Tool {
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

	for _, tools := range [][]tool.Tool{mcpTools, skillTools} {
		for _, t := range tools {
			mt, ok := t.(*tool.TrackedTool)
			if !ok {
				continue
			}
			dt, ok := mt.BaseTool.(*tool.DynamicTool)
			if !ok {
				continue
			}
			params := dt.Params
			if params == nil {
				params = map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}
			}
			result = append(result, openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        dt.ToolName,
					Description: dt.ToolDesc,
					Parameters:  params,
				},
			})
		}
	}

	return result
}
