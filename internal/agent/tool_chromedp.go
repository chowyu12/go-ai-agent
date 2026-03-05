package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

func webpageToText(targetURL string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	var textContent string
	tasks := chromedp.Tasks{
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2 * time.Second),
		chromedp.Evaluate(`document.body.innerText`, &textContent),
	}

	if err := chromedp.Run(ctx, tasks); err != nil {
		return "", fmt.Errorf("chromedp render: %w", err)
	}

	const maxLen = 10_000
	if len(textContent) > maxLen {
		textContent = textContent[:maxLen] + "\n... (content truncated)"
	}

	return textContent, nil
}
