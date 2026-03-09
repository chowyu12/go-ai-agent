package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type cronArgs struct {
	Expression string `json:"expression"`
	Count      int    `json:"count"`
	Timezone   string `json:"timezone"`
}

func Handler(_ context.Context, args string) (string, error) {
	var p cronArgs
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if p.Expression == "" {
		return "", fmt.Errorf("expression is required")
	}
	if p.Count <= 0 {
		p.Count = 5
	}
	if p.Count > 20 {
		p.Count = 20
	}

	loc := time.Local
	if p.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(p.Timezone)
		if err != nil {
			return "", fmt.Errorf("invalid timezone %q: %w", p.Timezone, err)
		}
	}

	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(p.Expression)
	if err != nil {
		return "", fmt.Errorf("invalid cron expression %q: %w", p.Expression, err)
	}

	now := time.Now().In(loc)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Expression: %s\n", p.Expression))
	sb.WriteString(fmt.Sprintf("Description: %s\n", describe(p.Expression)))
	sb.WriteString(fmt.Sprintf("Timezone: %s\n\n", loc))
	sb.WriteString(fmt.Sprintf("Next %d executions:\n", p.Count))

	t := now
	for i := range p.Count {
		t = sched.Next(t)
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, t.Format("2006-01-02 15:04:05 MST")))
	}

	return sb.String(), nil
}

func describe(expr string) string {
	expr = strings.TrimSpace(expr)

	if strings.HasPrefix(expr, "@") {
		switch expr {
		case "@yearly", "@annually":
			return "每年1月1日 00:00 执行"
		case "@monthly":
			return "每月1日 00:00 执行"
		case "@weekly":
			return "每周日 00:00 执行"
		case "@daily", "@midnight":
			return "每天 00:00 执行"
		case "@hourly":
			return "每小时 执行"
		}
		if strings.HasPrefix(expr, "@every ") {
			return fmt.Sprintf("每隔 %s 执行", strings.TrimPrefix(expr, "@every "))
		}
		return expr
	}

	fields := strings.Fields(expr)
	hasSeconds := len(fields) == 6

	var minute, hour, dom, month, dow string
	if hasSeconds {
		minute, hour, dom, month, dow = fields[1], fields[2], fields[3], fields[4], fields[5]
	} else if len(fields) == 5 {
		minute, hour, dom, month, dow = fields[0], fields[1], fields[2], fields[3], fields[4]
	} else {
		return expr
	}

	var parts []string

	if hasSeconds {
		parts = append(parts, descField(fields[0], "秒"))
	}
	parts = append(parts, descField(minute, "分"))
	parts = append(parts, descField(hour, "时"))
	parts = append(parts, descField(dom, "日"))
	parts = append(parts, descMonth(month))
	parts = append(parts, descDow(dow))

	var desc []string
	for _, p := range parts {
		if p != "" {
			desc = append(desc, p)
		}
	}
	if len(desc) == 0 {
		return "每分钟执行"
	}
	return strings.Join(desc, ", ")
}

func descField(f, unit string) string {
	if f == "*" {
		return ""
	}
	if strings.HasPrefix(f, "*/") {
		return fmt.Sprintf("每%s%s", strings.TrimPrefix(f, "*/"), unit)
	}
	return fmt.Sprintf("%s=%s", unit, f)
}

func descMonth(f string) string {
	if f == "*" {
		return ""
	}
	return fmt.Sprintf("月=%s", f)
}

func descDow(f string) string {
	if f == "*" || f == "?" {
		return ""
	}
	dayNames := map[string]string{
		"0": "日", "1": "一", "2": "二", "3": "三",
		"4": "四", "5": "五", "6": "六", "7": "日",
		"SUN": "日", "MON": "一", "TUE": "二", "WED": "三",
		"THU": "四", "FRI": "五", "SAT": "六",
	}
	if name, ok := dayNames[strings.ToUpper(f)]; ok {
		return fmt.Sprintf("周%s", name)
	}
	return fmt.Sprintf("周=%s", f)
}
