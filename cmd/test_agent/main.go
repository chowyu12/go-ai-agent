package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/agent"
	"github.com/chowyu12/go-ai-agent/internal/config"
	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/seed"
	"github.com/chowyu12/go-ai-agent/internal/store/mysql"
)

var (
	configFile = flag.String("config", "etc/config.yaml", "config file path")
	agentUUID  = flag.String("agent", "", "agent UUID (empty to list agents)")
	message    = flag.String("msg", "现在系统时间是什么时候", "message to send")
	userID     = flag.String("user", "test-cli", "user ID")
	stream     = flag.Bool("stream", false, "use streaming mode")
	verbose    = flag.Bool("v", false, "verbose debug logging")
)

func main() {
	flag.Parse()

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "15:04:05",
	})
	if *verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	store, err := mysql.New(cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect db: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	seed.Init(context.Background(), store)

	registry := agent.NewToolRegistry()
	executor := agent.NewExecutor(store, registry)
	ctx := context.Background()

	if *agentUUID == "" {
		listAgents(ctx, store)
		return
	}

	printHeader(*agentUUID, *message, *stream)

	if *stream {
		runStream(ctx, executor)
	} else {
		runBlocking(ctx, executor)
	}
}

func listAgents(ctx context.Context, s interface {
	ListAgents(ctx context.Context, q model.ListQuery) ([]*model.Agent, int64, error)
	GetAgentTools(ctx context.Context, agentID int64) ([]model.Tool, error)
	GetAgentSkills(ctx context.Context, agentID int64) ([]model.Skill, error)
	GetAgentChildren(ctx context.Context, agentID int64) ([]model.Agent, error)
}) {
	agents, total, err := s.ListAgents(ctx, model.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		fmt.Fprintf(os.Stderr, "list agents: %v\n", err)
		os.Exit(1)
	}
	if total == 0 {
		fmt.Println("No agents found. Create one via the admin dashboard first.")
		os.Exit(0)
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("  Available Agents (%d)\n", total)
	fmt.Println(strings.Repeat("=", 80))
	for _, a := range agents {
		tools, _ := s.GetAgentTools(ctx, a.ID)
		skills, _ := s.GetAgentSkills(ctx, a.ID)
		children, _ := s.GetAgentChildren(ctx, a.ID)

		toolNames := make([]string, 0, len(tools))
		for _, t := range tools {
			toolNames = append(toolNames, t.Name)
		}

		fmt.Printf("\n  UUID:       %s\n", a.UUID)
		fmt.Printf("  Name:       %s\n", a.Name)
		fmt.Printf("  Model:      %s (provider_id=%d)\n", a.ModelName, a.ProviderID)
		fmt.Printf("  Tools (%d):  %s\n", len(tools), strings.Join(toolNames, ", "))
		fmt.Printf("  Skills:     %d\n", len(skills))
		fmt.Printf("  Children:   %d\n", len(children))
	}
	fmt.Println()
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println("  Usage:")
	fmt.Printf("    go run ./cmd/test_agent --agent <UUID> --msg '你的消息'\n")
	fmt.Printf("    go run ./cmd/test_agent --agent <UUID> --msg '现在几点了' --stream\n")
	fmt.Printf("    go run ./cmd/test_agent --agent <UUID> --msg '帮我生成一个UUID' -v\n")
	fmt.Println(strings.Repeat("-", 80))
}

func printHeader(agentID, msg string, stream bool) {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  Agent:   %s\n", agentID)
	fmt.Printf("  Message: %s\n", msg)
	fmt.Printf("  Stream:  %v\n", stream)
	fmt.Println(strings.Repeat("=", 60))
}

func runBlocking(ctx context.Context, executor *agent.Executor) {
	req := model.ChatRequest{
		AgentID: *agentUUID,
		UserID:  *userID,
		Message: *message,
	}

	result, err := executor.Execute(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("  RESPONSE")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println(result.Content)
	fmt.Println()
	fmt.Printf("  Conversation ID: %s\n", result.ConversationID)

	if len(result.Steps) > 0 {
		fmt.Println()
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("  EXECUTION STEPS (%d)\n", len(result.Steps))
		fmt.Println(strings.Repeat("-", 60))
		for i, step := range result.Steps {
			fmt.Printf("  Step %d: [%s] %s  (duration: %dms, status: %s)\n",
				i+1, step.StepType, step.Name, step.DurationMs, step.Status)
			if step.Input != "" {
				fmt.Printf("    Input:  %s\n", truncate(step.Input, 200))
			}
			if step.Output != "" {
				fmt.Printf("    Output: %s\n", truncate(step.Output, 200))
			}
			if step.Error != "" {
				fmt.Printf("    Error:  %s\n", step.Error)
			}
		}
	}
	fmt.Println(strings.Repeat("=", 60))
}

func runStream(ctx context.Context, executor *agent.Executor) {
	req := model.ChatRequest{
		AgentID: *agentUUID,
		UserID:  *userID,
		Message: *message,
		Stream:  true,
	}

	fmt.Println()
	fmt.Print("  Response: ")

	err := executor.ExecuteStream(ctx, req, func(chunk model.StreamChunk) error {
		if chunk.Done {
			fmt.Println()
			fmt.Println()
			if chunk.Step != nil {
				stepJSON, _ := json.MarshalIndent(chunk.Step, "  ", "  ")
				fmt.Printf("  Last Step: %s\n", stepJSON)
			}
			fmt.Println(strings.Repeat("=", 60))
		} else {
			fmt.Print(chunk.Delta)
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nERROR: %v\n", err)
		os.Exit(1)
	}
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
