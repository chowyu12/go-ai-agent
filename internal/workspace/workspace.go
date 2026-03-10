package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

var root string

func Init(dir string) error {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get user home: %w", err)
		}
		dir = filepath.Join(home, ".go-agent")
	}
	root = dir

	for _, sub := range []string{
		"",
		"uploads",
		"skills",
		"cron/scripts",
		"cron/logs",
		"tmp",
	} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return fmt.Errorf("create workspace dir %q: %w", sub, err)
		}
	}
	return nil
}

func Root() string { return root }

func Uploads() string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, "uploads")
}

func CronDir() string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, "cron")
}

func CronScripts() string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, "cron", "scripts")
}

func CronLogs() string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, "cron", "logs")
}

func Skills() string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, "skills")
}

func SkillDir(dirName string) string {
	if root == "" || dirName == "" {
		return ""
	}
	return filepath.Join(root, "skills", dirName)
}

func Tmp() string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, "tmp")
}
