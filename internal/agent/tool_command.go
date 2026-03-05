package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

var dangerousPatterns = []string{
	"rm -rf /",
	"rm -rf /*",
	"rm -rf ~",
	"mkfs",
	"dd if=",
	":(){:|:&};:",
	"> /dev/sda",
	"chmod -R 777 /",
	"chown -R",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"init 0",
	"init 6",
	"kill -9 1",
	"killall",
	"pkill",
	"ssh-keygen",
	"ssh ",
	"scp ",
	"sftp ",
	"telnet ",
	"nc -l",
	"ncat -l",
	"curl.*|.*sh",
	"wget.*|.*sh",
	"useradd",
	"userdel",
	"usermod",
	"passwd",
	"visudo",
	"iptables -F",
	"iptables -X",
	"nft flush",
	"crontab -r",
	"systemctl disable",
	"service.*stop",
	"eval ",
	"exec ",
	"nohup ",
	"> /etc/",
	"tee /etc/",
	"mount ",
	"umount ",
	"fdisk ",
	"parted ",
	"wipefs",
}

func checkDangerousCommand(cmdStr string) error {
	lower := strings.ToLower(strings.TrimSpace(cmdStr))
	for _, p := range dangerousPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return fmt.Errorf("dangerous command blocked: contains '%s'", p)
		}
	}
	for _, seg := range strings.Split(lower, "|") {
		seg = strings.TrimSpace(seg)
		if strings.HasPrefix(seg, "rm ") && (strings.Contains(seg, " -r") || strings.Contains(seg, " -f")) {
			return fmt.Errorf("dangerous command blocked: recursive/force rm is not allowed")
		}
	}
	return nil
}

func commandToolHandler(ctx context.Context, cfg model.CommandHandlerConfig, timeoutSec int, input string) (string, error) {
	cmdStr := cfg.Command

	var params map[string]any
	if input != "" {
		json.Unmarshal([]byte(input), &params)
	}
	for key, val := range params {
		cmdStr = strings.ReplaceAll(cmdStr, "{"+key+"}", fmt.Sprint(val))
	}

	if err := checkDangerousCommand(cmdStr); err != nil {
		log.WithFields(log.Fields{"command": cmdStr, "reason": err}).Warn("[Tool] !! command blocked by safety check")
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	shell := cfg.Shell
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.CommandContext(ctx, shell, "-c", cmdStr)
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.WithFields(log.Fields{"command": cmdStr, "shell": shell, "timeout": timeoutSec}).Info("[Tool] >> exec command")
	err := cmd.Run()

	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\n[stderr]\n" + stderr.String()
	}

	const maxOutput = 10_000
	if len(result) > maxOutput {
		result = result[:maxOutput] + "\n... (output truncated)"
	}

	if err != nil {
		log.WithFields(log.Fields{"command": cmdStr, "error": err}).Warn("[Tool] << command failed")
		return result, fmt.Errorf("command failed: %w", err)
	}

	log.WithField("command", cmdStr).Info("[Tool] << command ok")
	return result, nil
}
