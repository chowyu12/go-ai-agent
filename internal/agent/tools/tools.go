package tools

import (
	"context"

	"github.com/chowyu12/go-ai-agent/internal/agent/tools/browser"
	"github.com/chowyu12/go-ai-agent/internal/agent/tools/builtin"
	cronTool "github.com/chowyu12/go-ai-agent/internal/agent/tools/cron"
	"github.com/chowyu12/go-ai-agent/internal/agent/tools/crontab"
	"github.com/chowyu12/go-ai-agent/internal/agent/tools/result"
	"github.com/chowyu12/go-ai-agent/internal/agent/tools/urlreader"
)

type FileResult = result.FileResult

var ParseFileResult = result.ParseFileResult

func DefaultBuiltins() map[string]func(context.Context, string) (string, error) {
	m := builtin.Handlers()
	m["url_reader"] = urlreader.Handler
	m["browser"] = browser.Handler
	m["cron_parser"] = cronTool.Handler
	m["crontab"] = crontab.Handler
	return m
}
