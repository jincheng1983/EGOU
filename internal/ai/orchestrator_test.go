package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAgentConfig(t *testing.T) {
	roles := []AgentRole{RolePlanner, RoleCoder, RoleReviewer, RoleUIBuilder, RoleFixer}
	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			cfg, ok := GetAgentConfig(role)
			if !ok {
				t.Fatalf("expected config for role %s", role)
			}
			if cfg.Role != role {
				t.Errorf("expected role %s, got %s", role, cfg.Role)
			}
			if cfg.Name == "" {
				t.Error("expected non-empty Name")
			}
			if cfg.SystemPrompt == "" {
				t.Error("expected non-empty SystemPrompt")
			}
			if cfg.MaxIterations <= 0 {
				t.Errorf("expected positive MaxIterations, got %d", cfg.MaxIterations)
			}
			if len(cfg.Tools) == 0 {
				t.Error("expected non-empty Tools")
			}
		})
	}

	t.Run("unknown role", func(t *testing.T) {
		_, ok := GetAgentConfig(AgentRole("unknown"))
		if ok {
			t.Error("expected false for unknown role")
		}
	})
}

func TestAllAgentConfigs(t *testing.T) {
	configs := AllAgentConfigs()
	if len(configs) != 5 {
		t.Errorf("expected 5 configs, got %d", len(configs))
	}
	seen := map[AgentRole]bool{}
	for _, cfg := range configs {
		seen[cfg.Role] = true
	}
	for _, role := range []AgentRole{RolePlanner, RoleCoder, RoleReviewer, RoleUIBuilder, RoleFixer} {
		if !seen[role] {
			t.Errorf("role %s not found in AllAgentConfigs", role)
		}
	}
}

func TestParseToolCalls(t *testing.T) {
	t.Run("single tool no params", func(t *testing.T) {
		input := "[TOOL:read_file]\n"
		calls := parseToolCalls(input)
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
		if calls[0].Tool != "read_file" {
			t.Errorf("expected tool read_file, got %s", calls[0].Tool)
		}
		if len(calls[0].Params) != 0 {
			t.Errorf("expected 0 params, got %d", len(calls[0].Params))
		}
	})

	t.Run("tool with params", func(t *testing.T) {
		input := "[TOOL:write_file path:/tmp/test.eg,content:hello world]\n"
		calls := parseToolCalls(input)
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
		if calls[0].Tool != "write_file" {
			t.Errorf("expected tool write_file, got %s", calls[0].Tool)
		}
		if calls[0].Params["path"] != "/tmp/test.eg" {
			t.Errorf("expected path param, got %q", calls[0].Params["path"])
		}
		if calls[0].Params["content"] != "hello world" {
			t.Errorf("expected content param, got %q", calls[0].Params["content"])
		}
	})

	t.Run("multiple tools", func(t *testing.T) {
		input := "Some text\n[TOOL:read_file path:a.go]\nmore text\n[TOOL:write_file path:b.go]\n"
		calls := parseToolCalls(input)
		if len(calls) != 2 {
			t.Fatalf("expected 2 calls, got %d", len(calls))
		}
		if calls[0].Tool != "read_file" {
			t.Errorf("expected first tool read_file, got %s", calls[0].Tool)
		}
		if calls[1].Tool != "write_file" {
			t.Errorf("expected second tool write_file, got %s", calls[1].Tool)
		}
	})

	t.Run("no tool calls", func(t *testing.T) {
		input := "Just regular text\nno tool calls here"
		calls := parseToolCalls(input)
		if len(calls) != 0 {
			t.Errorf("expected 0 calls, got %d", len(calls))
		}
	})

	t.Run("malformed no closing bracket", func(t *testing.T) {
		input := "[TOOL:read_file path:test\n"
		calls := parseToolCalls(input)
		if len(calls) != 0 {
			t.Errorf("expected 0 calls for malformed input, got %d", len(calls))
		}
	})

	t.Run("empty params value", func(t *testing.T) {
		input := "[TOOL:read_file path:]\n"
		calls := parseToolCalls(input)
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
		if v, ok := calls[0].Params["path"]; !ok || v != "" {
			t.Errorf("expected empty path param, got %q (ok=%v)", v, ok)
		}
	})
}

func TestTruncate(t *testing.T) {
	t.Run("short string unchanged", func(t *testing.T) {
		s := "hello"
		got := truncate(s, 100)
		if got != s {
			t.Errorf("expected unchanged, got %q", got)
		}
	})

	t.Run("exact length unchanged", func(t *testing.T) {
		s := "hello"
		got := truncate(s, 5)
		if got != s {
			t.Errorf("expected unchanged, got %q", got)
		}
	})

	t.Run("long string truncated", func(t *testing.T) {
		s := "abcdefghijklmnopqrstuvwxyz"
		got := truncate(s, 10)
		if !strings.HasPrefix(got, "abcdefghij") {
			t.Errorf("expected prefix preserved, got %q", got)
		}
		if !strings.Contains(got, "截断") {
			t.Errorf("expected truncation marker, got %q", got)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		got := truncate("", 10)
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestNewOrchestrator(t *testing.T) {
	client := NewClient("http://example.com", "key", "model")
	orch := NewOrchestrator(client)
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if orch.client != client {
		t.Error("expected client to be set")
	}
	if orch.sink != nil {
		t.Error("expected nil sink by default")
	}
}

func TestOrchestrator_SetSink(t *testing.T) {
	client := NewClient("http://example.com", "key", "model")
	orch := NewOrchestrator(client)
	called := false
	orch.SetSink(func(e PipelineEvent) {
		called = true
	})
	if orch.sink == nil {
		t.Error("expected sink to be set")
	}
	// Verify emit calls sink
	orch.emit("test", RoleCoder, 0, 1, nil)
	if !called {
		t.Error("expected sink to be called")
	}
}

func TestOrchestrator_RunAgent(t *testing.T) {
	t.Run("success with coder role", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			// Verify system prompt is set
			if len(req.Messages) < 2 {
				t.Errorf("expected at least 2 messages, got %d", len(req.Messages))
			}
			if req.Messages[0].Role != "system" {
				t.Errorf("expected first message to be system, got %s", req.Messages[0].Role)
			}
			if req.Messages[0].Content == "" {
				t.Error("expected non-empty system prompt")
			}
			// Last message should be user input
			last := req.Messages[len(req.Messages)-1]
			if last.Role != "user" || last.Content != "write a function" {
				t.Errorf("unexpected last message: %+v", last)
			}
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "Here is the code:\n```egou\n函数 测试()\n结束 函数\n```"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		orch := NewOrchestrator(client)
		result, err := orch.RunAgent(context.Background(), RoleCoder, "write a function", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}
		if result.Role != RoleCoder {
			t.Errorf("expected role coder, got %s", result.Role)
		}
		if !strings.Contains(result.Output, "```egou") {
			t.Errorf("expected code block in output, got %q", result.Output)
		}
		if result.Iterations != 1 {
			t.Errorf("expected 1 iteration, got %d", result.Iterations)
		}
	})

	t.Run("unknown role returns error", func(t *testing.T) {
		client := NewClient("http://example.com", "key", "model")
		orch := NewOrchestrator(client)
		result, err := orch.RunAgent(context.Background(), AgentRole("unknown"), "task", nil)
		if err == nil {
			t.Fatal("expected error for unknown role")
		}
		if !strings.Contains(err.Error(), "未知") {
			t.Errorf("expected '未知' in error, got %v", err)
		}
		if result.Success {
			t.Error("expected failure in result")
		}
		if result.Error == "" {
			t.Error("expected non-empty error in result")
		}
	})

	t.Run("with history messages", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ChatRequest
			json.NewDecoder(r.Body).Decode(&req)
			// system + 2 history + user = 4
			if len(req.Messages) != 4 {
				t.Errorf("expected 4 messages, got %d", len(req.Messages))
			}
			if req.Messages[1].Content != "prev question" {
				t.Errorf("expected history first, got %q", req.Messages[1].Content)
			}
			if req.Messages[2].Content != "prev answer" {
				t.Errorf("expected history second, got %q", req.Messages[2].Content)
			}
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "ok"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		orch := NewOrchestrator(client)
		history := []Message{
			{Role: "user", Content: "prev question"},
			{Role: "assistant", Content: "prev answer"},
		}
		_, err := orch.RunAgent(context.Background(), RolePlanner, "new task", history)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("emits start and end events", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "done"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		orch := NewOrchestrator(client)
		var events []PipelineEvent
		orch.SetSink(func(e PipelineEvent) {
			events = append(events, e)
		})
		_, err := orch.RunAgent(context.Background(), RoleCoder, "task", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		if events[0].Stage != "agent-start" {
			t.Errorf("expected agent-start, got %s", events[0].Stage)
		}
		if events[1].Stage != "agent-end" {
			t.Errorf("expected agent-end, got %s", events[1].Stage)
		}
		if events[1].Result == nil {
			t.Error("expected non-nil result in agent-end event")
		}
	})

	t.Run("parses tool calls from output", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "I will read the file:\n[TOOL:read_file path:main.go]\n"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		orch := NewOrchestrator(client)
		result, err := orch.RunAgent(context.Background(), RoleCoder, "task", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.ToolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
		}
		if result.ToolCalls[0].Tool != "read_file" {
			t.Errorf("expected tool read_file, got %s", result.ToolCalls[0].Tool)
		}
	})
}

func TestOrchestrator_RunPipeline(t *testing.T) {
	t.Run("empty pipeline returns error", func(t *testing.T) {
		client := NewClient("http://example.com", "key", "model")
		orch := NewOrchestrator(client)
		_, err := orch.RunPipeline(context.Background(), nil, "task", nil)
		if err == nil {
			t.Fatal("expected error for empty pipeline")
		}
		if !strings.Contains(err.Error(), "空") {
			t.Errorf("expected '空' in error, got %v", err)
		}
	})

	t.Run("success multi-stage", func(t *testing.T) {
		var callCount int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: fmt.Sprintf("output %d", callCount)}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		orch := NewOrchestrator(client)
		var stages []string
		orch.SetSink(func(e PipelineEvent) {
			stages = append(stages, e.Stage)
		})

		pipeline := []AgentRole{RolePlanner, RoleCoder}
		result, err := orch.RunPipeline(context.Background(), pipeline, "implement feature", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if callCount != 2 {
			t.Errorf("expected 2 API calls, got %d", callCount)
		}
		if !result.Success {
			t.Error("expected success")
		}
		if result.Role != RoleCoder {
			t.Errorf("expected last role coder, got %s", result.Role)
		}
		// Expected: agent-start, agent-end, agent-start, agent-end, pipeline-done
		if len(stages) != 5 {
			t.Fatalf("expected 5 events, got %d: %v", len(stages), stages)
		}
		if stages[0] != "agent-start" {
			t.Errorf("expected agent-start at 0, got %s", stages[0])
		}
		if stages[4] != "pipeline-done" {
			t.Errorf("expected pipeline-done last, got %s", stages[4])
		}
	})

	t.Run("passes previous output to next agent", func(t *testing.T) {
		var lastUserContent string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req ChatRequest
			json.NewDecoder(r.Body).Decode(&req)
			last := req.Messages[len(req.Messages)-1]
			lastUserContent = last.Content
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "plan done"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		orch := NewOrchestrator(client)
		pipeline := []AgentRole{RolePlanner, RoleCoder}
		_, err := orch.RunPipeline(context.Background(), pipeline, "implement feature", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Second agent should receive the first agent's output
		if !strings.Contains(lastUserContent, "plan done") {
			t.Errorf("expected second agent to receive first output, got %q", lastUserContent)
		}
		if !strings.Contains(lastUserContent, "上一步") {
			t.Errorf("expected '上一步' marker, got %q", lastUserContent)
		}
	})

	t.Run("unknown role at first position", func(t *testing.T) {
		// 把 unknown role 放在第一位,这样在调用 API 前就会被拦截
		client := NewClient("http://example.com", "key", "model")
		orch := NewOrchestrator(client)
		pipeline := []AgentRole{AgentRole("unknown")}
		_, err := orch.RunPipeline(context.Background(), pipeline, "task", nil)
		if err == nil {
			t.Fatal("expected error for unknown role")
		}
		if !strings.Contains(err.Error(), "未知角色") {
			t.Errorf("expected '未知角色' in error, got %v", err)
		}
	})

	t.Run("unknown role at second position", func(t *testing.T) {
		// 第一个 agent 用 mock server 让它成功,第二个 unknown role 才会被检查到
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "ok"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		orch := NewOrchestrator(client)
		pipeline := []AgentRole{RolePlanner, AgentRole("unknown")}
		_, err := orch.RunPipeline(context.Background(), pipeline, "task", nil)
		if err == nil {
			t.Fatal("expected error for unknown role")
		}
		if !strings.Contains(err.Error(), "未知角色") {
			t.Errorf("expected '未知角色' in error, got %v", err)
		}
	})

	t.Run("single agent pipeline", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Choices: []ChatChoice{
					{Message: Message{Role: "assistant", Content: "single"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		orch := NewOrchestrator(client)
		result, err := orch.RunPipeline(context.Background(), []AgentRole{RoleCoder}, "task", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Output != "single" {
			t.Errorf("expected 'single', got %q", result.Output)
		}
	})
}
