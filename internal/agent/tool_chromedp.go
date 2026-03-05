package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type ScreenshotOption struct {
	Filename  string
	FullPage  bool
	Width     int
	Height    int
	Quality   int
	OutputDir string
	Timeout   time.Duration
}

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

func webpageToImage(url string, opt *ScreenshotOption) (string, error) {
	if opt == nil {
		opt = &ScreenshotOption{}
	}
	if opt.OutputDir == "" {
		opt.OutputDir = os.TempDir()
	}
	if opt.Filename == "" {
		opt.Filename = fmt.Sprintf("screenshot_%d.png", time.Now().UnixMilli())
	}
	if opt.Width <= 0 {
		opt.Width = 1920
	}
	if opt.Height <= 0 {
		opt.Height = 1080
	}
	if opt.Quality < 1 || opt.Quality > 100 {
		opt.Quality = 95
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 30 * time.Second
	}

	if err := os.MkdirAll(opt.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	outputPath := filepath.Join(opt.OutputDir, opt.Filename)

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, opt.Timeout)
	defer cancel()

	var screenshot []byte

	tasks := chromedp.Tasks{
		emulation.SetDeviceMetricsOverride(int64(opt.Width), int64(opt.Height), 1.0, false),
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2 * time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			screenshot, err = page.CaptureScreenshot().
				WithQuality(int64(opt.Quality)).
				WithCaptureBeyondViewport(opt.FullPage).
				WithFromSurface(true).
				Do(ctx)
			return err
		}),
	}

	if err := chromedp.Run(ctx, tasks); err != nil {
		return "", fmt.Errorf("screenshot failed: %w", err)
	}

	if err := os.WriteFile(outputPath, screenshot, 0644); err != nil {
		return "", fmt.Errorf("save image: %w", err)
	}

	return outputPath, nil
}
