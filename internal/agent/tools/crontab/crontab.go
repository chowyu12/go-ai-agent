package crontab

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chowyu12/go-ai-agent/internal/workspace"
	"github.com/robfig/cron/v3"
)

type crontabArgs struct {
	Action     string `json:"action"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Expression string `json:"expression"`
	Command    string `json:"command"`
	Pattern    string `json:"pattern"`
	LogOutput  bool   `json:"log_output"`
}

func Handler(_ context.Context, args string) (string, error) {
	var p crontabArgs
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	switch p.Action {
	case "save_script":
		return saveScript(p)
	case "add_job":
		return addJob(p)
	case "list_jobs":
		return listJobs()
	case "remove_job":
		return removeJob(p)
	default:
		return "", fmt.Errorf("unknown action: %s, supported: save_script, add_job, list_jobs, remove_job", p.Action)
	}
}

func scriptDir() string {
	if d := workspace.CronScripts(); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".go-agent", "cron", "scripts")
}

func logDir() string {
	if d := workspace.CronLogs(); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".go-agent", "cron", "logs")
}

func defaultPath() string {
	if runtime.GOOS == "darwin" {
		return "/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"
	}
	return "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
}

func saveScript(p crontabArgs) (string, error) {
	if p.Name == "" {
		return "", fmt.Errorf("name is required for save_script")
	}
	if p.Content == "" {
		return "", fmt.Errorf("content is required for save_script")
	}

	dir := scriptDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create script dir: %w", err)
	}

	name := sanitizeName(p.Name)
	if !strings.HasSuffix(name, ".sh") {
		name += ".sh"
	}

	content := p.Content
	if !strings.HasPrefix(strings.TrimSpace(content), "#!") {
		content = "#!/bin/bash\nset -eo pipefail\nexport PATH=\"" + defaultPath() + ":$PATH\"\n\n" + content
	}

	filePath := filepath.Join(dir, name)
	if err := os.WriteFile(filePath, []byte(content), 0o755); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}

	return fmt.Sprintf("Script saved: %s\n\n%s", filePath, content), nil
}

func addJob(p crontabArgs) (string, error) {
	if p.Expression == "" {
		return "", fmt.Errorf("expression is required for add_job")
	}
	if p.Command == "" {
		return "", fmt.Errorf("command is required for add_job")
	}

	if strings.HasPrefix(p.Expression, "@every") {
		return "", fmt.Errorf("@every is not supported by system crontab; use a standard 5-field expression (e.g. '*/5 * * * *')")
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(p.Expression); err != nil {
		return "", fmt.Errorf("invalid cron expression %q: %w", p.Expression, err)
	}

	command := p.Command
	if p.LogOutput {
		if err := os.MkdirAll(logDir(), 0o755); err != nil {
			return "", fmt.Errorf("create log dir: %w", err)
		}
		logName := sanitizeName(filepath.Base(command))
		if strings.HasSuffix(logName, ".sh") {
			logName = strings.TrimSuffix(logName, ".sh")
		}
		logFile := filepath.Join(logDir(), logName+".log")
		command = fmt.Sprintf("%s >> %s 2>&1", command, logFile)
	}

	entry := fmt.Sprintf("%s %s", p.Expression, command)

	existing, _ := exec.Command("crontab", "-l").Output()
	existingStr := string(existing)
	for _, line := range strings.Split(existingStr, "\n") {
		if strings.TrimSpace(line) == entry {
			return fmt.Sprintf("Job already exists: %s", entry), nil
		}
	}

	newCrontab := ensureEnvHeader(existingStr)
	newCrontab = strings.TrimRight(newCrontab, "\n") + "\n" + entry + "\n"

	if err := installCrontab(newCrontab); err != nil {
		return "", err
	}

	return fmt.Sprintf("Cron job added:\n  %s", entry), nil
}

func ensureEnvHeader(existing string) string {
	hasShell := false
	hasPath := false
	for _, line := range strings.Split(existing, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "SHELL=") {
			hasShell = true
		}
		if strings.HasPrefix(trimmed, "PATH=") {
			hasPath = true
		}
	}

	var header strings.Builder
	if !hasShell {
		header.WriteString("SHELL=/bin/bash\n")
	}
	if !hasPath {
		header.WriteString("PATH=" + defaultPath() + "\n")
	}
	if header.Len() == 0 {
		return existing
	}
	return header.String() + existing
}

func installCrontab(content string) error {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install crontab: %w\n%s", err, string(output))
	}
	return nil
}

func listJobs() (string, error) {
	output, err := exec.Command("crontab", "-l").CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "no crontab") {
			return "No crontab entries found.", nil
		}
		return "", fmt.Errorf("list crontab: %w\n%s", err, string(output))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return "No crontab entries found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Current crontab (%d lines):\n", len(lines)))
	for i, line := range lines {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, line))
	}
	return sb.String(), nil
}

func removeJob(p crontabArgs) (string, error) {
	if p.Pattern == "" {
		return "", fmt.Errorf("pattern is required for remove_job")
	}

	existing, err := exec.Command("crontab", "-l").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read crontab: %w\n%s", err, string(existing))
	}

	var kept, removed []string
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.Contains(line, p.Pattern) {
			removed = append(removed, line)
		} else {
			kept = append(kept, line)
		}
	}

	if len(removed) == 0 {
		return fmt.Sprintf("No entries matching %q found.", p.Pattern), nil
	}

	newCrontab := strings.Join(kept, "\n")
	if !strings.HasSuffix(newCrontab, "\n") {
		newCrontab += "\n"
	}

	if err := installCrontab(newCrontab); err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Removed %d entries:\n", len(removed)))
	for _, line := range removed {
		sb.WriteString(fmt.Sprintf("  - %s\n", line))
	}
	return sb.String(), nil
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "..", "_")
	return name
}
