package crontab

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

func baseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".agent-cron")
}

func scriptDir() string { return filepath.Join(baseDir(), "scripts") }
func logDir() string    { return filepath.Join(baseDir(), "logs") }

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
		content = "#!/bin/bash\nset -euo pipefail\n\n" + content
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
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == entry {
			return fmt.Sprintf("Job already exists: %s", entry), nil
		}
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf(`(crontab -l 2>/dev/null; echo %q) | crontab -`, entry))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("add crontab: %w\n%s", err, string(output))
	}

	return fmt.Sprintf("Cron job added:\n  %s", entry), nil
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
	sb.WriteString(fmt.Sprintf("Current crontab (%d entries):\n", len(lines)))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
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
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`echo %q | crontab -`, newCrontab))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("update crontab: %w\n%s", err, string(output))
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
