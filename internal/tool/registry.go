package tool

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type Registry struct {
	builtins map[string]BuiltinHandler
}

func NewRegistry() *Registry {
	return &Registry{builtins: make(map[string]BuiltinHandler)}
}

func (r *Registry) LoadDefaults() {
	for name, handler := range DefaultBuiltins() {
		r.builtins[name] = handler
	}
}

func (r *Registry) RegisterBuiltin(name string, handler BuiltinHandler) {
	r.builtins[name] = handler
}

func (r *Registry) BuildTrackedTools(toolDefs []model.Tool, tracker *StepTracker, toolSkillMap map[string]string) []Tool {
	var result []Tool
	for _, td := range toolDefs {
		if !td.Enabled {
			continue
		}
		baseTool := r.buildTool(td)
		if baseTool == nil {
			log.WithField("tool", td.Name).Warn("no handler found for tool, skipping")
			continue
		}
		result = append(result, &TrackedTool{
			BaseTool:  baseTool,
			ToolName:  td.Name,
			SkillName: toolSkillMap[td.Name],
			Tracker:   tracker,
		})
	}
	return result
}

func (r *Registry) buildTool(td model.Tool) Tool {
	switch td.HandlerType {
	case model.HandlerBuiltin:
		handler, ok := r.builtins[td.Name]
		if !ok {
			return nil
		}
		return &DynamicTool{ToolName: td.Name, ToolDesc: td.Description, Handler: handler}
	case model.HandlerHTTP:
		var cfg model.HTTPHandlerConfig
		if json.Unmarshal(td.HandlerConfig, &cfg) != nil {
			return nil
		}
		return &DynamicTool{
			ToolName: td.Name,
			ToolDesc: td.Description,
			Handler:  NewHTTPHandler(cfg, td.TimeoutSeconds()),
		}
	case model.HandlerCommand:
		var cfg model.CommandHandlerConfig
		if json.Unmarshal(td.HandlerConfig, &cfg) != nil {
			return nil
		}
		return &DynamicTool{
			ToolName: td.Name,
			ToolDesc: td.Description,
			Handler:  NewCommandHandler(cfg, td.TimeoutSeconds()),
		}
	default:
		return nil
	}
}
