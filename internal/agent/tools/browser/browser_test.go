package browser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIsURLSafe(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid_http", "http://example.com", false},
		{"valid_https", "https://example.com/path?q=1", false},
		{"blocked_localhost", "http://localhost:8080", true},
		{"blocked_127", "http://127.0.0.1/admin", true},
		{"blocked_loopback_v6", "http://[::1]/api", true},
		{"blocked_private_10", "http://10.0.0.1/internal", true},
		{"blocked_private_172", "http://172.16.0.1/db", true},
		{"blocked_private_192", "http://192.168.1.1/router", true},
		{"blocked_file_scheme", "file:///etc/passwd", true},
		{"blocked_ftp_scheme", "ftp://example.com/file", true},
		{"valid_public_ip", "http://8.8.8.8/dns", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isURLSafe(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("isURLSafe(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestWrapUntrustedContent(t *testing.T) {
	content := "Hello World"
	r := wrapUntrustedContent(content)

	if !strings.HasPrefix(r, "[UNTRUSTED_WEB_CONTENT_START]") {
		t.Error("missing start marker")
	}
	if !strings.HasSuffix(r, "[UNTRUSTED_WEB_CONTENT_END]") {
		t.Error("missing end marker")
	}
	if !strings.Contains(r, content) {
		t.Error("content not included")
	}
}

func TestBrowserJSON(t *testing.T) {
	r := browserJSON("ok", true, "url", "https://example.com", "count", 42)
	var m map[string]any
	if err := json.Unmarshal([]byte(r), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["ok"] != true {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	if m["url"] != "https://example.com" {
		t.Errorf("url = %v", m["url"])
	}
	if m["count"] != float64(42) {
		t.Errorf("count = %v", m["count"])
	}
}

func TestBrowserParams_Parse(t *testing.T) {
	input := `{"action":"click","ref":"e5","double_click":true}`
	var p browserParams
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if p.Action != "click" {
		t.Errorf("action = %q", p.Action)
	}
	if p.Ref != "e5" {
		t.Errorf("ref = %q", p.Ref)
	}
	if !p.DoubleClick {
		t.Error("double_click should be true")
	}
}

func TestBrowserParams_ParseFillForm(t *testing.T) {
	input := `{"action":"fill_form","fields":[{"ref":"e1","value":"hello","type":"text"},{"ref":"e2","value":"pass","type":"password"}]}`
	var p browserParams
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(p.Fields) != 2 {
		t.Fatalf("fields len = %d, want 2", len(p.Fields))
	}
	if p.Fields[0].Ref != "e1" || p.Fields[0].Value != "hello" {
		t.Errorf("field[0] = %+v", p.Fields[0])
	}
}

func TestBrowserParams_ParseDialog(t *testing.T) {
	input := `{"action":"dialog","accept":false,"prompt_text":"test input"}`
	var p browserParams
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if p.Accept == nil || *p.Accept != false {
		t.Error("accept should be false")
	}
	if p.PromptText != "test input" {
		t.Errorf("prompt_text = %q", p.PromptText)
	}
}

func TestHandler_MissingAction(t *testing.T) {
	_, err := Handler(t.Context(), `{}`)
	if err == nil || !strings.Contains(err.Error(), "action is required") {
		t.Errorf("expected 'action is required', got %v", err)
	}
}

func TestHandler_UnknownAction(t *testing.T) {
	_, err := Handler(t.Context(), `{"action":"fly"}`)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("expected 'unknown action', got %v", err)
	}
}

func TestHandler_InvalidJSON(t *testing.T) {
	_, err := Handler(t.Context(), `not json`)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid arguments error, got %v", err)
	}
}

func TestBrowserManager_CloseNotRunning(t *testing.T) {
	bm := &browserManager{
		tabs: make(map[string]*tabInfo),
		refs: make(map[string]elementInfo),
	}
	r, err := bm.closeBrowser()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(r, "not running") {
		t.Errorf("expected 'not running' in result, got %q", r)
	}
}

func TestBrowserManager_RefSelector_NotFound(t *testing.T) {
	bm := &browserManager{
		refs: make(map[string]elementInfo),
	}
	_, err := bm.refSelector("e99")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found', got %v", err)
	}
}

func TestBrowserManager_RefSelector_Found(t *testing.T) {
	bm := &browserManager{
		refs: map[string]elementInfo{
			"e1": {Ref: "e1", Tag: "button", Text: "Submit"},
		},
	}
	sel, err := bm.refSelector("e1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sel != `[data-agent-ref="e1"]` {
		t.Errorf("selector = %q", sel)
	}
}

func TestBrowserManager_ResolveSelector(t *testing.T) {
	bm := &browserManager{
		refs: map[string]elementInfo{
			"e3": {Ref: "e3", Tag: "input"},
		},
	}

	t.Run("by_ref", func(t *testing.T) {
		sel, err := bm.resolveSelector(browserParams{Ref: "e3"})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if sel != `[data-agent-ref="e3"]` {
			t.Errorf("selector = %q", sel)
		}
	})

	t.Run("by_selector", func(t *testing.T) {
		sel, err := bm.resolveSelector(browserParams{Selector: "#myInput"})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if sel != "#myInput" {
			t.Errorf("selector = %q", sel)
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, err := bm.resolveSelector(browserParams{})
		if err == nil {
			t.Error("expected error for missing ref and selector")
		}
	})
}

func TestFormatSnapshot(t *testing.T) {
	r := snapshotResult{
		URL:   "https://example.com",
		Title: "Test Page",
		Elements: []elementInfo{
			{Ref: "e1", Tag: "a", Href: "/about", Text: "About Us"},
			{Ref: "e2", Tag: "button", Text: "Submit"},
			{Ref: "e3", Tag: "input", Type: "text", Placeholder: "Search..."},
		},
		Text: "Welcome to test page",
	}

	output := formatSnapshot(r)

	if !strings.Contains(output, "https://example.com") {
		t.Error("missing URL")
	}
	if !strings.Contains(output, "Test Page") {
		t.Error("missing title")
	}
	if !strings.Contains(output, "[e1]") || !strings.Contains(output, "About Us") {
		t.Error("missing element e1")
	}
	if !strings.Contains(output, "[e2]") || !strings.Contains(output, "<button") {
		t.Error("missing element e2")
	}
	if !strings.Contains(output, "[e3]") || !strings.Contains(output, `type="text"`) {
		t.Error("missing element e3")
	}
	if !strings.Contains(output, `placeholder="Search..."`) {
		t.Error("missing placeholder")
	}
	if !strings.Contains(output, "Page Text") {
		t.Error("missing page text section")
	}
}

func TestFormatSnapshot_Empty(t *testing.T) {
	r := snapshotResult{
		URL:   "about:blank",
		Title: "",
	}

	output := formatSnapshot(r)
	if !strings.Contains(output, "No interactive elements found") {
		t.Error("expected 'No interactive elements' message")
	}
}

func TestFormatSnapshot_LongHref(t *testing.T) {
	longHref := strings.Repeat("a", 200)
	r := snapshotResult{
		URL:   "https://example.com",
		Title: "Test",
		Elements: []elementInfo{
			{Ref: "e1", Tag: "a", Href: longHref},
		},
	}

	output := formatSnapshot(r)
	if strings.Contains(output, longHref) {
		t.Error("long href should be truncated")
	}
	if !strings.Contains(output, "...") {
		t.Error("truncated href should end with ...")
	}
}
