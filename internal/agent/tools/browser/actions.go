package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/agent/tools/result"
)

func (bm *browserManager) actionNavigate(_ context.Context, p browserParams) (string, error) {
	if p.URL == "" {
		return "", fmt.Errorf("url is required for navigate")
	}
	if err := isURLSafe(p.URL); err != nil {
		return "", err
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	if err := chromedp.Run(tabCtx, chromedp.Navigate(p.URL)); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}
	_ = chromedp.Run(tabCtx, chromedp.WaitReady("body", chromedp.ByQuery))

	bm.mu.Lock()
	bm.refs = make(map[string]elementInfo)
	bm.mu.Unlock()

	bm.updateTabInfo(tabCtx)

	var title string
	_ = chromedp.Run(tabCtx, chromedp.Title(&title))

	pageText := fetchPageText(tabCtx, 3000)
	meta := browserJSON("ok", true, "url", p.URL, "title", title)
	if pageText == "" {
		return meta, nil
	}
	return meta + "\n\n" + wrapUntrustedContent(pageText), nil
}

func (bm *browserManager) actionScreenshot(_ context.Context, p browserParams) (string, error) {
	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	var buf []byte

	switch {
	case p.Ref != "":
		sel, selErr := bm.refSelector(p.Ref)
		if selErr != nil {
			return "", selErr
		}
		if err := chromedp.Run(tabCtx, chromedp.Screenshot(sel, &buf, chromedp.ByQuery)); err != nil {
			return "", fmt.Errorf("screenshot element: %w", err)
		}
	case p.FullPage:
		if err := chromedp.Run(tabCtx, chromedp.FullScreenshot(&buf, 90)); err != nil {
			return "", fmt.Errorf("full screenshot: %w", err)
		}
	default:
		if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			var captureErr error
			buf, captureErr = page.CaptureScreenshot().Do(ctx)
			return captureErr
		})); err != nil {
			return "", fmt.Errorf("screenshot: %w", err)
		}
	}

	filePath := bm.tempFilePath(".png")
	if err := os.WriteFile(filePath, buf, 0o644); err != nil {
		return "", fmt.Errorf("save screenshot: %w", err)
	}

	return result.NewFileResult(filePath, "image/png", "Browser screenshot"), nil
}

func (bm *browserManager) actionSnapshot(_ context.Context, p browserParams) (string, error) {
	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}
	return bm.takeSnapshot(tabCtx, p.Selector)
}

func (bm *browserManager) actionGetText(_ context.Context, p browserParams) (string, error) {
	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	js := `(document.body&&document.body.innerText||'').substring(0,10000)`
	if p.Ref != "" {
		sel, selErr := bm.refSelector(p.Ref)
		if selErr != nil {
			return "", selErr
		}
		js = fmt.Sprintf(`(function(){var el=document.querySelector(%q);return el?el.innerText.substring(0,10000):''})()`, sel)
	} else if p.Selector != "" {
		js = fmt.Sprintf(`(function(){var el=document.querySelector(%q);return el?el.innerText.substring(0,10000):''})()`, p.Selector)
	}

	var text string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(js, &text)); err != nil {
		return "", fmt.Errorf("get_text: %w", err)
	}

	return wrapUntrustedContent(text), nil
}

func (bm *browserManager) actionEvaluate(_ context.Context, p browserParams) (string, error) {
	if p.Expression == "" {
		return "", fmt.Errorf("expression is required for evaluate")
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	evalCtx, cancel := context.WithTimeout(tabCtx, 10*time.Second)
	defer cancel()

	var evalResult any
	if err := chromedp.Run(evalCtx, chromedp.Evaluate(p.Expression, &evalResult)); err != nil {
		return "", fmt.Errorf("evaluate: %w", err)
	}

	data, _ := json.MarshalIndent(evalResult, "", "  ")
	return wrapUntrustedContent(string(data)), nil
}

func (bm *browserManager) actionPDF(_ context.Context, p browserParams) (string, error) {
	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	var buf []byte
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var printErr error
		buf, _, printErr = page.PrintToPDF().Do(ctx)
		return printErr
	})); err != nil {
		return "", fmt.Errorf("pdf: %w", err)
	}

	filePath := bm.tempFilePath(".pdf")
	if err := os.WriteFile(filePath, buf, 0o644); err != nil {
		return "", fmt.Errorf("save pdf: %w", err)
	}

	return result.NewFileResult(filePath, "application/pdf", "Browser page PDF"), nil
}

func (bm *browserManager) actionClick(_ context.Context, p browserParams) (string, error) {
	sel, err := bm.resolveSelector(p)
	if err != nil {
		return "", err
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	switch {
	case p.DoubleClick:
		if err := chromedp.Run(tabCtx, chromedp.DoubleClick(sel, chromedp.ByQuery)); err != nil {
			return "", fmt.Errorf("double click: %w", err)
		}
	case p.Button == "right":
		js := fmt.Sprintf(`(function(){var el=document.querySelector(%q);if(!el)return false;el.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,button:2}));return true})()`, sel)
		var ok bool
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(js, &ok)); err != nil || !ok {
			return "", fmt.Errorf("right click failed")
		}
	case p.Button == "middle":
		js := fmt.Sprintf(`(function(){var el=document.querySelector(%q);if(!el)return false;el.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,button:1}));return true})()`, sel)
		var ok bool
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(js, &ok)); err != nil || !ok {
			return "", fmt.Errorf("middle click failed")
		}
	default:
		if err := chromedp.Run(tabCtx, chromedp.Click(sel, chromedp.ByQuery)); err != nil {
			return "", fmt.Errorf("click: %w", err)
		}
	}

	time.Sleep(300 * time.Millisecond)
	bm.updateTabInfo(tabCtx)

	var currentURL string
	_ = chromedp.Run(tabCtx, chromedp.Location(&currentURL))
	return browserJSON("ok", true, "url", currentURL), nil
}

func (bm *browserManager) actionType(_ context.Context, p browserParams) (string, error) {
	if p.Text == "" {
		return "", fmt.Errorf("text is required for type")
	}
	sel, err := bm.resolveSelector(p)
	if err != nil {
		return "", err
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	if p.Slowly {
		for _, ch := range p.Text {
			if err := chromedp.Run(tabCtx, chromedp.SendKeys(sel, string(ch), chromedp.ByQuery)); err != nil {
				return "", fmt.Errorf("type slowly: %w", err)
			}
			time.Sleep(80 * time.Millisecond)
		}
	} else {
		if err := chromedp.Run(tabCtx, chromedp.SendKeys(sel, p.Text, chromedp.ByQuery)); err != nil {
			return "", fmt.Errorf("type: %w", err)
		}
	}

	if p.Submit {
		if err := chromedp.Run(tabCtx, chromedp.SendKeys(sel, "\r", chromedp.ByQuery)); err != nil {
			log.WithError(err).Warn("[Browser] submit (Enter) failed")
		}
		time.Sleep(500 * time.Millisecond)
		bm.updateTabInfo(tabCtx)
	}

	return browserJSON("ok", true), nil
}

func (bm *browserManager) actionHover(_ context.Context, p browserParams) (string, error) {
	sel, err := bm.resolveSelector(p)
	if err != nil {
		return "", err
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	js := fmt.Sprintf(`(function(){
		var el=document.querySelector(%q);
		if(!el) return false;
		el.dispatchEvent(new MouseEvent('mouseover',{bubbles:true}));
		el.dispatchEvent(new MouseEvent('mouseenter',{bubbles:true}));
		return true;
	})()`, sel)

	var ok bool
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(js, &ok)); err != nil || !ok {
		return "", fmt.Errorf("hover failed on %q", sel)
	}

	return browserJSON("ok", true), nil
}

func (bm *browserManager) actionDrag(_ context.Context, p browserParams) (string, error) {
	if p.StartRef == "" || p.EndRef == "" {
		return "", fmt.Errorf("start_ref and end_ref are required for drag")
	}

	startSel, err := bm.refSelector(p.StartRef)
	if err != nil {
		return "", err
	}
	endSel, err := bm.refSelector(p.EndRef)
	if err != nil {
		return "", err
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	js := fmt.Sprintf(`(function(){
		var s=document.querySelector(%q), e=document.querySelector(%q);
		if(!s||!e) return 'elements not found';
		var dt=new DataTransfer();
		var sr=s.getBoundingClientRect(), er=e.getBoundingClientRect();
		s.dispatchEvent(new DragEvent('dragstart',{bubbles:true,cancelable:true,dataTransfer:dt,clientX:sr.left+sr.width/2,clientY:sr.top+sr.height/2}));
		e.dispatchEvent(new DragEvent('dragover',{bubbles:true,cancelable:true,dataTransfer:dt,clientX:er.left+er.width/2,clientY:er.top+er.height/2}));
		e.dispatchEvent(new DragEvent('drop',{bubbles:true,cancelable:true,dataTransfer:dt,clientX:er.left+er.width/2,clientY:er.top+er.height/2}));
		s.dispatchEvent(new DragEvent('dragend',{bubbles:true,cancelable:true,dataTransfer:dt}));
		return 'ok';
	})()`, startSel, endSel)

	var dragResult string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(js, &dragResult)); err != nil {
		return "", fmt.Errorf("drag: %w", err)
	}
	if dragResult != "ok" {
		return "", fmt.Errorf("drag: %s", dragResult)
	}

	return browserJSON("ok", true), nil
}

func (bm *browserManager) actionSelect(_ context.Context, p browserParams) (string, error) {
	sel, err := bm.resolveSelector(p)
	if err != nil {
		return "", err
	}
	if len(p.Values) == 0 {
		return "", fmt.Errorf("values is required for select")
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	valuesJS, _ := json.Marshal(p.Values)
	js := fmt.Sprintf(`(function(){
		var el=document.querySelector(%q);
		if(!el||el.tagName!=='SELECT') return 'not a select element';
		var vals=%s;
		Array.from(el.options).forEach(function(opt){
			opt.selected=vals.indexOf(opt.value)>=0;
		});
		el.dispatchEvent(new Event('change',{bubbles:true}));
		return 'ok';
	})()`, sel, string(valuesJS))

	var selectResult string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(js, &selectResult)); err != nil {
		return "", fmt.Errorf("select: %w", err)
	}
	if selectResult != "ok" {
		return "", fmt.Errorf("select: %s", selectResult)
	}

	return browserJSON("ok", true), nil
}

func (bm *browserManager) actionFillForm(_ context.Context, p browserParams) (string, error) {
	if len(p.Fields) == 0 {
		return "", fmt.Errorf("fields is required for fill_form")
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	var filled int
	for _, f := range p.Fields {
		sel, selErr := bm.refSelector(f.Ref)
		if selErr != nil {
			return "", fmt.Errorf("fill field %s: %w", f.Ref, selErr)
		}

		clearJS := fmt.Sprintf(`(function(){var el=document.querySelector(%q);if(el){el.value='';el.dispatchEvent(new Event('input',{bubbles:true}))}})()`, sel)
		_ = chromedp.Run(tabCtx, chromedp.Evaluate(clearJS, nil))

		if err := chromedp.Run(tabCtx, chromedp.SendKeys(sel, f.Value, chromedp.ByQuery)); err != nil {
			return "", fmt.Errorf("fill field %s: %w", f.Ref, err)
		}
		filled++
	}

	return browserJSON("ok", true, "filled", filled), nil
}

func (bm *browserManager) actionScroll(_ context.Context, p browserParams) (string, error) {
	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	if p.Ref != "" {
		sel, selErr := bm.refSelector(p.Ref)
		if selErr != nil {
			return "", selErr
		}
		if err := chromedp.Run(tabCtx, chromedp.ScrollIntoView(sel, chromedp.ByQuery)); err != nil {
			return "", fmt.Errorf("scroll to element: %w", err)
		}
		return browserJSON("ok", true, "scrolled_to", p.Ref), nil
	}

	if p.Selector != "" {
		if err := chromedp.Run(tabCtx, chromedp.ScrollIntoView(p.Selector, chromedp.ByQuery)); err != nil {
			return "", fmt.Errorf("scroll to selector: %w", err)
		}
		return browserJSON("ok", true, "scrolled_to", p.Selector), nil
	}

	js := fmt.Sprintf(`window.scrollTo(0,%d)`, p.ScrollY)
	if p.ScrollY == 0 {
		js = `window.scrollTo(0,document.body.scrollHeight)`
	}
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(js, nil)); err != nil {
		return "", fmt.Errorf("scroll: %w", err)
	}

	return browserJSON("ok", true), nil
}

func (bm *browserManager) actionUpload(_ context.Context, p browserParams) (string, error) {
	sel, err := bm.resolveSelector(p)
	if err != nil {
		return "", err
	}
	if len(p.Paths) == 0 {
		return "", fmt.Errorf("paths is required for upload")
	}

	for _, path := range p.Paths {
		if _, statErr := os.Stat(path); statErr != nil {
			return "", fmt.Errorf("file not found: %s", path)
		}
	}

	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	if err := chromedp.Run(tabCtx, chromedp.SetUploadFiles(sel, p.Paths, chromedp.ByQuery)); err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}

	changeJS := fmt.Sprintf(`(function(){var el=document.querySelector(%q);if(el)el.dispatchEvent(new Event('change',{bubbles:true}))})()`, sel)
	_ = chromedp.Run(tabCtx, chromedp.Evaluate(changeJS, nil))

	return browserJSON("ok", true, "uploaded", len(p.Paths)), nil
}

func (bm *browserManager) actionWait(_ context.Context, p browserParams) (string, error) {
	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	if p.WaitTime > 0 {
		time.Sleep(time.Duration(p.WaitTime) * time.Millisecond)
		return browserJSON("ok", true, "waited_ms", p.WaitTime), nil
	}

	timeout := 15 * time.Second
	waitCtx, cancel := context.WithTimeout(tabCtx, timeout)
	defer cancel()

	if p.WaitSelector != "" {
		if err := chromedp.Run(waitCtx, chromedp.WaitVisible(p.WaitSelector, chromedp.ByQuery)); err != nil {
			return "", fmt.Errorf("wait: selector %q not visible within timeout: %w", p.WaitSelector, err)
		}
		return browserJSON("ok", true, "found_selector", p.WaitSelector), nil
	}

	if p.WaitText != "" {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-waitCtx.Done():
				return "", fmt.Errorf("wait timeout: text %q not found", p.WaitText)
			case <-ticker.C:
				var text string
				_ = chromedp.Run(tabCtx, chromedp.Evaluate(`document.body.innerText`, &text))
				if strings.Contains(text, p.WaitText) {
					return browserJSON("ok", true, "found_text", p.WaitText), nil
				}
			}
		}
	}

	if p.WaitURL != "" {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-waitCtx.Done():
				return "", fmt.Errorf("wait timeout: URL %q not matched", p.WaitURL)
			case <-ticker.C:
				var currentURL string
				_ = chromedp.Run(tabCtx, chromedp.Location(&currentURL))
				if strings.Contains(currentURL, p.WaitURL) {
					return browserJSON("ok", true, "url", currentURL), nil
				}
			}
		}
	}

	return browserJSON("ok", true, "message", "no wait condition specified"), nil
}

func (bm *browserManager) actionDialog(_ context.Context, p browserParams) (string, error) {
	tabCtx, err := bm.getTabCtx(p.TargetID)
	if err != nil {
		return "", err
	}

	accept := true
	if p.Accept != nil {
		accept = *p.Accept
	}

	action := page.HandleJavaScriptDialog(accept)
	if p.PromptText != "" {
		action = action.WithPromptText(p.PromptText)
	}

	if err := chromedp.Run(tabCtx, action); err != nil {
		return "", fmt.Errorf("dialog: %w", err)
	}

	return browserJSON("ok", true, "accepted", accept), nil
}

func (bm *browserManager) actionTabs() (string, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	type tabEntry struct {
		ID     string `json:"id"`
		URL    string `json:"url"`
		Title  string `json:"title"`
		Active bool   `json:"active"`
	}

	tabs := make([]tabEntry, 0, len(bm.tabs))
	for _, t := range bm.tabs {
		tabs = append(tabs, tabEntry{
			ID: t.id, URL: t.url, Title: t.title,
			Active: t.id == bm.activeTab,
		})
	}

	data, _ := json.Marshal(map[string]any{"tabs": tabs})
	return string(data), nil
}

func (bm *browserManager) actionOpenTab(p browserParams) (string, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	tabCtx, tabCancel := chromedp.NewContext(bm.allocCtx)

	targetURL := "about:blank"
	if p.URL != "" {
		if err := isURLSafe(p.URL); err != nil {
			tabCancel()
			return "", err
		}
		targetURL = p.URL
	}

	if err := chromedp.Run(tabCtx, chromedp.Navigate(targetURL)); err != nil {
		tabCancel()
		return "", fmt.Errorf("open_tab: %w", err)
	}
	_ = chromedp.Run(tabCtx, chromedp.WaitReady("body", chromedp.ByQuery))

	var title string
	_ = chromedp.Run(tabCtx, chromedp.Title(&title))

	tabID := uuid.New().String()[:8]
	bm.tabs[tabID] = &tabInfo{
		id: tabID, ctx: tabCtx, cancel: tabCancel,
		url: targetURL, title: title,
	}
	bm.activeTab = tabID
	bm.refs = make(map[string]elementInfo)

	pageText := fetchPageText(tabCtx, 3000)
	meta := browserJSON("ok", true, "target_id", tabID, "url", targetURL, "title", title)
	if pageText == "" {
		return meta, nil
	}
	return meta + "\n\n" + wrapUntrustedContent(pageText), nil
}

func (bm *browserManager) actionCloseTab(p browserParams) (string, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	targetID := p.TargetID
	if targetID == "" {
		targetID = bm.activeTab
	}

	tab, ok := bm.tabs[targetID]
	if !ok {
		return "", fmt.Errorf("tab %q not found", targetID)
	}

	if len(bm.tabs) <= 1 {
		return "", fmt.Errorf("cannot close the last tab, use close action to stop browser")
	}

	tab.cancel()
	delete(bm.tabs, targetID)

	if bm.activeTab == targetID {
		for id := range bm.tabs {
			bm.activeTab = id
			break
		}
		bm.refs = make(map[string]elementInfo)
	}

	log.WithField("tab", targetID).Info("[Browser] tab closed")
	return browserJSON("ok", true, "closed", targetID, "active", bm.activeTab), nil
}
