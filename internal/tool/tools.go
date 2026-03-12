package tool

import (
	"context"

	"github.com/chowyu12/go-ai-agent/internal/tool/browser"
	"github.com/chowyu12/go-ai-agent/internal/tool/builtin"
	"github.com/chowyu12/go-ai-agent/internal/tool/codeinterp"
	cronTool "github.com/chowyu12/go-ai-agent/internal/tool/cron"
	"github.com/chowyu12/go-ai-agent/internal/tool/crontab"
	"github.com/chowyu12/go-ai-agent/internal/tool/result"
	"github.com/chowyu12/go-ai-agent/internal/tool/urlreader"
)

type FileResult = result.FileResult

var ParseFileResult = result.ParseFileResult

func DefaultBuiltins() map[string]func(context.Context, string) (string, error) {
	m := builtin.Handlers()
	m["url_reader"] = urlreader.Handler
	m["browser"] = browser.Handler
	m["cron_parser"] = cronTool.Handler
	m["crontab"] = crontab.Handler
	m["code_interpreter"] = codeinterp.Handler
	return m
}
