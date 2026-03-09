package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type browserParams struct {
	Action       string      `json:"action"`
	URL          string      `json:"url"`
	Ref          string      `json:"ref"`
	Text         string      `json:"text"`
	Expression   string      `json:"expression"`
	Selector     string      `json:"selector"`
	FullPage     bool        `json:"full_page"`
	Submit       bool        `json:"submit"`
	Slowly       bool        `json:"slowly"`
	Button       string      `json:"button"`
	DoubleClick  bool        `json:"double_click"`
	StartRef     string      `json:"start_ref"`
	EndRef       string      `json:"end_ref"`
	Values       []string    `json:"values"`
	Fields       []formField `json:"fields"`
	TargetID     string      `json:"target_id"`
	WaitTime     int         `json:"wait_time"`
	WaitText     string      `json:"wait_text"`
	WaitSelector string      `json:"wait_selector"`
	WaitURL      string      `json:"wait_url"`
	Accept       *bool       `json:"accept"`
	PromptText   string      `json:"prompt_text"`
	Paths        []string    `json:"paths"`
	ScrollY      int         `json:"scroll_y"`
}

type formField struct {
	Ref   string `json:"ref"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type tabInfo struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	url    string
	title  string
}

type browserManager struct {
	mu          sync.Mutex
	allocCtx    context.Context
	allocCancel context.CancelFunc
	tabs        map[string]*tabInfo
	activeTab   string
	refs        map[string]elementInfo
	started     bool
	tmpDir      string
}

var defaultBrowser = &browserManager{
	tabs: make(map[string]*tabInfo),
	refs: make(map[string]elementInfo),
}

func Handler(ctx context.Context, args string) (string, error) {
	var p browserParams
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if p.Action == "" {
		return "", fmt.Errorf("action is required")
	}

	bm := defaultBrowser

	if p.Action == "close" {
		return bm.closeBrowser()
	}

	if err := bm.ensureStarted(); err != nil {
		return "", fmt.Errorf("start browser: %w", err)
	}

	switch p.Action {
	case "navigate":
		return bm.actionNavigate(ctx, p)
	case "screenshot":
		return bm.actionScreenshot(ctx, p)
	case "snapshot":
		return bm.actionSnapshot(ctx, p)
	case "get_text":
		return bm.actionGetText(ctx, p)
	case "evaluate":
		return bm.actionEvaluate(ctx, p)
	case "pdf":
		return bm.actionPDF(ctx, p)
	case "click":
		return bm.actionClick(ctx, p)
	case "type":
		return bm.actionType(ctx, p)
	case "hover":
		return bm.actionHover(ctx, p)
	case "drag":
		return bm.actionDrag(ctx, p)
	case "select":
		return bm.actionSelect(ctx, p)
	case "fill_form":
		return bm.actionFillForm(ctx, p)
	case "scroll":
		return bm.actionScroll(ctx, p)
	case "upload":
		return bm.actionUpload(ctx, p)
	case "wait":
		return bm.actionWait(ctx, p)
	case "dialog":
		return bm.actionDialog(ctx, p)
	case "tabs":
		return bm.actionTabs()
	case "open_tab":
		return bm.actionOpenTab(p)
	case "close_tab":
		return bm.actionCloseTab(p)
	default:
		return "", fmt.Errorf("unknown action: %s", p.Action)
	}
}

func (bm *browserManager) ensureStarted() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.started {
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "browser-agent-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	bm.tmpDir = tmpDir

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.WindowSize(1280, 720),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	bm.allocCtx = allocCtx
	bm.allocCancel = allocCancel

	tabCtx, tabCancel := chromedp.NewContext(allocCtx,
		chromedp.WithErrorf(log.Errorf),
	)
	if err := chromedp.Run(tabCtx, chromedp.Navigate("about:blank")); err != nil {
		tabCancel()
		allocCancel()
		bm.tmpDir = ""
		os.RemoveAll(tmpDir)
		return fmt.Errorf("init browser: %w", err)
	}

	tabID := uuid.New().String()[:8]
	bm.tabs[tabID] = &tabInfo{
		id: tabID, ctx: tabCtx, cancel: tabCancel,
		url: "about:blank", title: "New Tab",
	}
	bm.activeTab = tabID
	bm.started = true

	log.WithField("tab", tabID).Info("[Browser] started")
	return nil
}

func (bm *browserManager) closeBrowser() (string, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.started {
		return browserJSON("ok", true, "message", "browser not running"), nil
	}

	for _, tab := range bm.tabs {
		tab.cancel()
	}
	if bm.allocCancel != nil {
		bm.allocCancel()
	}
	if bm.tmpDir != "" {
		os.RemoveAll(bm.tmpDir)
	}

	bm.tabs = make(map[string]*tabInfo)
	bm.refs = make(map[string]elementInfo)
	bm.activeTab = ""
	bm.started = false
	bm.tmpDir = ""

	log.Info("[Browser] closed")
	return browserJSON("ok", true, "message", "browser closed"), nil
}

func (bm *browserManager) getTabCtx(targetID string) (context.Context, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	id := targetID
	if id == "" {
		id = bm.activeTab
	}
	tab, ok := bm.tabs[id]
	if !ok {
		return nil, fmt.Errorf("tab %q not found", id)
	}
	return tab.ctx, nil
}

func (bm *browserManager) refSelector(ref string) (string, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if _, ok := bm.refs[ref]; !ok {
		return "", fmt.Errorf("ref %q not found, run snapshot action first", ref)
	}
	return fmt.Sprintf(`[data-agent-ref="%s"]`, ref), nil
}

func (bm *browserManager) resolveSelector(p browserParams) (string, error) {
	if p.Ref != "" {
		return bm.refSelector(p.Ref)
	}
	if p.Selector != "" {
		return p.Selector, nil
	}
	return "", fmt.Errorf("ref or selector is required")
}

func (bm *browserManager) tempFilePath(ext string) string {
	return filepath.Join(bm.tmpDir, fmt.Sprintf("browser_%s%s", uuid.New().String()[:8], ext))
}

func (bm *browserManager) updateTabInfo(tabCtx context.Context) {
	var currentURL, title string
	_ = chromedp.Run(tabCtx, chromedp.Location(&currentURL))
	_ = chromedp.Run(tabCtx, chromedp.Title(&title))

	bm.mu.Lock()
	defer bm.mu.Unlock()
	if tab, ok := bm.tabs[bm.activeTab]; ok {
		if currentURL != "" {
			tab.url = currentURL
		}
		if title != "" {
			tab.title = title
		}
	}
}

func isURLSafe(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("blocked scheme %q: only http/https allowed", scheme)
	}

	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("blocked host: localhost")
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("blocked private/loopback IP: %s", host)
		}
	}

	return nil
}

func fetchPageText(tabCtx context.Context, maxLen int) string {
	js := fmt.Sprintf(`(document.body&&document.body.innerText||'').substring(0,%d)`, maxLen)
	var text string
	_ = chromedp.Run(tabCtx, chromedp.Evaluate(js, &text))
	return strings.TrimSpace(text)
}

func wrapUntrustedContent(content string) string {
	return "[UNTRUSTED_WEB_CONTENT_START]\n" + content + "\n[UNTRUSTED_WEB_CONTENT_END]"
}

func browserJSON(fields ...any) string {
	m := make(map[string]any)
	for i := 0; i+1 < len(fields); i += 2 {
		if key, ok := fields[i].(string); ok {
			m[key] = fields[i+1]
		}
	}
	data, _ := json.Marshal(m)
	return string(data)
}
