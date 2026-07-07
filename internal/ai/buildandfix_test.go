package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCompileErrors(t *testing.T) {
	t.Run("with multiple errors", func(t *testing.T) {
		output := `./main.go:10:3: undefined: foo
main.go:15:5: cannot use x (type int) as type string
some other line without match
./runner.go:20:1: missing semicolon`
		errs := parseCompileErrors(output)
		if len(errs) != 3 {
			t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
		}
		if !strings.Contains(errs[0], "main.go:10:3") {
			t.Errorf("expected first error about main.go:10:3, got %s", errs[0])
		}
		if !strings.Contains(errs[0], "undefined: foo") {
			t.Errorf("expected error message, got %s", errs[0])
		}
		if !strings.Contains(errs[1], "main.go:15:5") {
			t.Errorf("expected second error about main.go:15:5, got %s", errs[1])
		}
		if !strings.Contains(errs[2], "runner.go:20:1") {
			t.Errorf("expected third error about runner.go:20:1, got %s", errs[2])
		}
	})

	t.Run("no errors in output", func(t *testing.T) {
		output := "Build successful, no errors"
		errs := parseCompileErrors(output)
		if len(errs) != 0 {
			t.Errorf("expected 0 errors, got %d", len(errs))
		}
	})

	t.Run("empty output", func(t *testing.T) {
		errs := parseCompileErrors("")
		if len(errs) != 0 {
			t.Errorf("expected 0 errors, got %d", len(errs))
		}
	})

	t.Run("non-go file not matched", func(t *testing.T) {
		output := "main.txt:10:3: some error"
		errs := parseCompileErrors(output)
		if len(errs) != 0 {
			t.Errorf("expected 0 errors for non-go file, got %d", len(errs))
		}
	})

	t.Run("single error without dot prefix", func(t *testing.T) {
		output := "main.go:5:1: expected declaration"
		errs := parseCompileErrors(output)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if !strings.Contains(errs[0], "main.go:5:1") {
			t.Errorf("unexpected error: %s", errs[0])
		}
	})
}

func TestExtractCodeBlock(t *testing.T) {
	t.Run("nxg block", func(t *testing.T) {
		input := "Here is the code:\n```egou\n函数 测试()\n    调试输出(\"hello\")\n结束 函数\n```\nDone."
		got := extractCodeBlock(input)
		if !strings.Contains(got, "函数 测试()") {
			t.Errorf("expected function in output, got %q", got)
		}
		if !strings.Contains(got, "结束 函数") {
			t.Errorf("expected end function in output, got %q", got)
		}
		if !strings.Contains(got, "调试输出") {
			t.Errorf("expected debug output in result, got %q", got)
		}
	})

	t.Run("go block", func(t *testing.T) {
		input := "```go\npackage main\nfunc main() {}\n```"
		got := extractCodeBlock(input)
		if !strings.Contains(got, "package main") {
			t.Errorf("expected package main, got %q", got)
		}
		if !strings.Contains(got, "func main()") {
			t.Errorf("expected func main, got %q", got)
		}
	})

	t.Run("plain block without language", func(t *testing.T) {
		input := "```\ncode here\n```"
		got := extractCodeBlock(input)
		if got != "code here" {
			t.Errorf("expected 'code here', got %q", got)
		}
	})

	t.Run("egou block", func(t *testing.T) {
		input := "```egou\ncode\n```"
		got := extractCodeBlock(input)
		if got != "code" {
			t.Errorf("expected 'code', got %q", got)
		}
	})

	t.Run("no code block returns empty", func(t *testing.T) {
		input := "Just text, no code block here"
		got := extractCodeBlock(input)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("multiple blocks returns last", func(t *testing.T) {
		input := "```egou\nfirst\n```\nText\n```egou\nsecond\n```"
		got := extractCodeBlock(input)
		if got != "second" {
			t.Errorf("expected 'second', got %q", got)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		input := "```egou\n   padded code   \n```"
		got := extractCodeBlock(input)
		if got != "padded code" {
			t.Errorf("expected trimmed 'padded code', got %q", got)
		}
	})
}

func TestBuildAndFix(t *testing.T) {
	t.Run("build succeeds immediately", func(t *testing.T) {
		buildFn := func(source, projectPath string) (string, error) {
			return "build success", nil
		}
		var events []FixEvent
		sink := func(e FixEvent) {
			events = append(events, e)
		}
		finalSrc, output, success, err := BuildAndFix("initial code", "/project", buildFn, nil, 3, sink)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !success {
			t.Error("expected success")
		}
		if finalSrc != "initial code" {
			t.Errorf("expected unchanged source, got %q", finalSrc)
		}
		if output != "build success" {
			t.Errorf("expected 'build success', got %q", output)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d: %+v", len(events), events)
		}
		if events[0].Stage != "build-start" {
			t.Errorf("expected build-start, got %s", events[0].Stage)
		}
		if events[1].Stage != "build-success" {
			t.Errorf("expected build-success, got %s", events[1].Stage)
		}
		if events[0].Round != 1 {
			t.Errorf("expected round 1, got %d", events[0].Round)
		}
	})

	t.Run("build fails no AI client returns error", func(t *testing.T) {
		buildFn := func(source, projectPath string) (string, error) {
			return "./main.go:10:3: undefined: foo", fmt.Errorf("build failed")
		}
		_, _, success, err := BuildAndFix("code", "/project", buildFn, nil, 3, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if success {
			t.Error("expected failure")
		}
		if !strings.Contains(err.Error(), "AI Client") {
			t.Errorf("expected AI Client error, got %v", err)
		}
	})

	t.Run("build fails unparseable error returns original error", func(t *testing.T) {
		buildFn := func(source, projectPath string) (string, error) {
			return "some random error without file:line:col format", fmt.Errorf("failed")
		}
		_, _, success, err := BuildAndFix("code", "/project", buildFn, nil, 3, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if success {
			t.Error("expected failure")
		}
		if !strings.Contains(err.Error(), "failed") {
			t.Errorf("expected original error, got %v", err)
		}
	})

	t.Run("fix and succeed on second round", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "Here is the fix:\n```egou\nfixed code\n```"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")

		callCount := 0
		buildFn := func(source, projectPath string) (string, error) {
			callCount++
			if callCount == 1 {
				return "./main.go:10:3: undefined: foo", fmt.Errorf("build failed")
			}
			return "build success", nil
		}

		var stages []string
		sink := func(e FixEvent) {
			stages = append(stages, e.Stage)
		}

		finalSrc, _, success, err := BuildAndFix("initial code", "/project", buildFn, client, 3, sink)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !success {
			t.Error("expected success")
		}
		if finalSrc != "fixed code" {
			t.Errorf("expected 'fixed code', got %q", finalSrc)
		}
		if callCount != 2 {
			t.Errorf("expected 2 build calls, got %d", callCount)
		}
		// Expected: build-start, build-failed, fix-start, fix-applied, build-start, build-success
		if len(stages) != 6 {
			t.Fatalf("expected 6 events, got %d: %v", len(stages), stages)
		}
		if stages[0] != "build-start" {
			t.Errorf("expected build-start at 0, got %s", stages[0])
		}
		if stages[1] != "build-failed" {
			t.Errorf("expected build-failed at 1, got %s", stages[1])
		}
		if stages[2] != "fix-start" {
			t.Errorf("expected fix-start at 2, got %s", stages[2])
		}
		if stages[3] != "fix-applied" {
			t.Errorf("expected fix-applied at 3, got %s", stages[3])
		}
		if stages[4] != "build-start" {
			t.Errorf("expected second build-start at 4, got %s", stages[4])
		}
		if stages[5] != "build-success" {
			t.Errorf("expected build-success at 5, got %s", stages[5])
		}
	})

	t.Run("AI returns no code block returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "I can't fix this, here is why: ..."}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		buildFn := func(source, projectPath string) (string, error) {
			return "./main.go:10:3: undefined: foo", fmt.Errorf("failed")
		}
		_, _, success, err := BuildAndFix("code", "/project", buildFn, client, 3, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if success {
			t.Error("expected failure")
		}
		if !strings.Contains(err.Error(), "代码块") {
			t.Errorf("expected 代码块 error, got %v", err)
		}
	})

	t.Run("max rounds exceeded", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "```egou\nstill broken\n```"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		buildFn := func(source, projectPath string) (string, error) {
			return "./main.go:10:3: undefined: foo", fmt.Errorf("failed")
		}

		var lastStage string
		sink := func(e FixEvent) {
			lastStage = e.Stage
		}

		_, _, success, err := BuildAndFix("code", "/project", buildFn, client, 2, sink)
		if err == nil {
			t.Fatal("expected error")
		}
		if success {
			t.Error("expected failure")
		}
		if !strings.Contains(err.Error(), "最大修复轮数") {
			t.Errorf("expected max rounds error, got %v", err)
		}
		if lastStage != "max-rounds-exceeded" {
			t.Errorf("expected last stage 'max-rounds-exceeded', got %s", lastStage)
		}
	})

	t.Run("maxRounds zero uses default", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "```egou\ncode\n```"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		buildCount := 0
		buildFn := func(source, projectPath string) (string, error) {
			buildCount++
			return "./main.go:10:3: undefined: foo", fmt.Errorf("failed")
		}
		_, _, _, _ = BuildAndFix("code", "/project", buildFn, client, 0, nil)
		// DefaultMaxRounds = 3, so build should be called 3 times
		if buildCount != DefaultMaxRounds {
			t.Errorf("expected %d build calls, got %d", DefaultMaxRounds, buildCount)
		}
	})

	t.Run("sink receives source on fix-applied", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "```egou\nnew source\n```"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		buildFn := func(source, projectPath string) (string, error) {
			return "./main.go:10:3: undefined: foo", fmt.Errorf("failed")
		}

		var fixAppliedSource string
		var gotErrors []string
		sink := func(e FixEvent) {
			if e.Stage == "fix-applied" {
				fixAppliedSource = e.Source
				gotErrors = e.Errors
			}
		}

		_, _, _, _ = BuildAndFix("initial", "/project", buildFn, client, 1, sink)
		if fixAppliedSource != "new source" {
			t.Errorf("expected 'new source' in fix-applied event, got %q", fixAppliedSource)
		}
		if len(gotErrors) == 0 {
			t.Error("expected errors in fix-applied event")
		}
	})
}
