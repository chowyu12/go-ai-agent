package codeinterp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chowyu12/go-ai-agent/internal/workspace"
)

func setupSandbox(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := workspace.Init(dir); err != nil {
		t.Fatalf("workspace init: %v", err)
	}
}

func parseResult(t *testing.T, raw string) codeResult {
	t.Helper()
	var r codeResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("parse result: %v, raw: %s", err, raw)
	}
	return r
}

func TestHandler_InvalidJSON(t *testing.T) {
	setupSandbox(t)
	out, err := Handler(t.Context(), "not json")
	if err != nil {
		t.Fatal(err)
	}
	r := parseResult(t, out)
	if r.OK {
		t.Error("expected ok=false for invalid json")
	}
	if !strings.Contains(r.Error, "invalid parameters") {
		t.Errorf("unexpected error: %s", r.Error)
	}
}

func TestHandler_EmptyCode(t *testing.T) {
	setupSandbox(t)
	out, err := Handler(t.Context(), `{"language":"python","code":""}`)
	if err != nil {
		t.Fatal(err)
	}
	r := parseResult(t, out)
	if r.OK {
		t.Error("expected ok=false for empty code")
	}
	if !strings.Contains(r.Error, "code is required") {
		t.Errorf("unexpected error: %s", r.Error)
	}
}

func TestHandler_UnsupportedLanguage(t *testing.T) {
	setupSandbox(t)
	out, err := Handler(t.Context(), `{"language":"ruby","code":"puts 1"}`)
	if err != nil {
		t.Fatal(err)
	}
	r := parseResult(t, out)
	if r.OK {
		t.Error("expected ok=false for unsupported language")
	}
	if !strings.Contains(r.Error, "unsupported language") {
		t.Errorf("unexpected error: %s", r.Error)
	}
}

func TestHandler_PythonExec(t *testing.T) {
	setupSandbox(t)
	out, err := Handler(t.Context(), `{"language":"python","code":"print('hello world')"}`)
	if err != nil {
		t.Fatal(err)
	}
	r := parseResult(t, out)
	if !r.OK {
		t.Fatalf("expected ok=true, got error: %s", r.Error)
	}
	if !strings.Contains(r.Stdout, "hello world") {
		t.Errorf("unexpected stdout: %s", r.Stdout)
	}
	if r.Language != "python" {
		t.Errorf("unexpected language: %s", r.Language)
	}
	if r.ExitCode != 0 {
		t.Errorf("unexpected exit_code: %d", r.ExitCode)
	}
	if r.DurationMs <= 0 {
		t.Error("expected positive duration")
	}

	fp := filepath.Join(workspace.Sandbox(), r.File)
	if _, err := os.Stat(fp); err != nil {
		t.Errorf("sandbox file not found: %v", err)
	}
}

func TestHandler_ShellExec(t *testing.T) {
	setupSandbox(t)
	out, err := Handler(t.Context(), `{"language":"shell","code":"echo hello from shell"}`)
	if err != nil {
		t.Fatal(err)
	}
	r := parseResult(t, out)
	if !r.OK {
		t.Fatalf("expected ok=true, got error: %s", r.Error)
	}
	if !strings.Contains(r.Stdout, "hello from shell") {
		t.Errorf("unexpected stdout: %s", r.Stdout)
	}
}

func TestHandler_PythonError(t *testing.T) {
	setupSandbox(t)
	out, err := Handler(t.Context(), `{"language":"python","code":"raise ValueError('boom')"}`)
	if err != nil {
		t.Fatal(err)
	}
	r := parseResult(t, out)
	if r.OK {
		t.Error("expected ok=false for python error")
	}
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}
	if !strings.Contains(r.Stderr, "boom") {
		t.Errorf("expected stderr to contain 'boom': %s", r.Stderr)
	}
}

func TestHandler_CustomTimeout(t *testing.T) {
	setupSandbox(t)
	out, err := Handler(t.Context(), `{"language":"shell","code":"echo fast","timeout":5}`)
	if err != nil {
		t.Fatal(err)
	}
	r := parseResult(t, out)
	if !r.OK {
		t.Fatalf("expected ok=true, got error: %s", r.Error)
	}
}

func TestCheckDangerousCode_Python(t *testing.T) {
	tests := []struct {
		code    string
		blocked bool
	}{
		{"import os; os.system('rm -rf /')", true},
		{"subprocess.call(['ls'])", true},
		{"subprocess.Popen(['echo'])", true},
		{"print('hello')", false},
		{"import math; math.sqrt(4)", false},
	}
	for _, tt := range tests {
		err := checkDangerousCode("python", tt.code)
		if tt.blocked && err == nil {
			t.Errorf("expected code to be blocked: %s", tt.code)
		}
		if !tt.blocked && err != nil {
			t.Errorf("expected code to pass: %s, got: %v", tt.code, err)
		}
	}
}

func TestCheckDangerousCode_JavaScript(t *testing.T) {
	tests := []struct {
		code    string
		blocked bool
	}{
		{"require('child_process').exec('ls')", true},
		{"const cp = require('child_process')", true},
		{"process.exit(1)", true},
		{"console.log('hello')", false},
		{"const x = 1 + 2", false},
	}
	for _, tt := range tests {
		err := checkDangerousCode("javascript", tt.code)
		if tt.blocked && err == nil {
			t.Errorf("expected code to be blocked: %s", tt.code)
		}
		if !tt.blocked && err != nil {
			t.Errorf("expected code to pass: %s, got: %v", tt.code, err)
		}
	}
}

func TestCheckDangerousCode_Shell(t *testing.T) {
	tests := []struct {
		code    string
		blocked bool
	}{
		{"rm -rf /", true},
		{"rm -rf /*", true},
		{"shutdown -h now", true},
		{"crontab -r", true},
		{"echo hello", false},
		{"ls -la", false},
		{"date", false},
	}
	for _, tt := range tests {
		err := checkDangerousCode("shell", tt.code)
		if tt.blocked && err == nil {
			t.Errorf("expected code to be blocked: %s", tt.code)
		}
		if !tt.blocked && err != nil {
			t.Errorf("expected code to pass: %s, got: %v", tt.code, err)
		}
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if got := truncate(short, 100); got != short {
		t.Errorf("expected %q, got %q", short, got)
	}

	long := strings.Repeat("x", 200)
	got := truncate(long, 100)
	if len(got) <= 100 {
		t.Error("expected truncated output to include suffix")
	}
	if !strings.Contains(got, "truncated") {
		t.Error("expected truncated suffix")
	}
	if !strings.HasPrefix(got, strings.Repeat("x", 100)) {
		t.Error("expected prefix to be preserved")
	}
}

func TestMarshalResult(t *testing.T) {
	r := codeResult{OK: true, Language: "python", File: "test.py", ExitCode: 0}
	s := marshalResult(r)
	var decoded codeResult
	if err := json.Unmarshal([]byte(s), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.OK || decoded.Language != "python" {
		t.Errorf("unexpected result: %+v", decoded)
	}
}

func TestHandler_LanguageNormalization(t *testing.T) {
	setupSandbox(t)
	out, err := Handler(t.Context(), `{"language":"  PYTHON  ","code":"print(42)"}`)
	if err != nil {
		t.Fatal(err)
	}
	r := parseResult(t, out)
	if !r.OK {
		t.Fatalf("expected ok=true, got error: %s", r.Error)
	}
	if r.Language != "python" {
		t.Errorf("expected normalized language 'python', got %q", r.Language)
	}
	if !strings.Contains(r.Stdout, "42") {
		t.Errorf("unexpected stdout: %s", r.Stdout)
	}
}
