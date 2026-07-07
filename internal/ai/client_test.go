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

func TestMessage_JSON(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		m := Message{Role: "user", Content: "hello"}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		got := string(data)
		if !strings.Contains(got, `"role":"user"`) {
			t.Errorf("expected role in output, got %s", got)
		}
		if !strings.Contains(got, `"content":"hello"`) {
			t.Errorf("expected content in output, got %s", got)
		}
	})

	t.Run("unmarshal", func(t *testing.T) {
		data := `{"role":"assistant","content":"hi"}`
		var m Message
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if m.Role != "assistant" {
			t.Errorf("expected role assistant, got %s", m.Role)
		}
		if m.Content != "hi" {
			t.Errorf("expected content hi, got %s", m.Content)
		}
	})
}

func TestChatRequest_JSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		req := ChatRequest{
			Model:       "gpt-4",
			Messages:    []Message{{Role: "user", Content: "hi"}},
			Temperature: 0.5,
			MaxTokens:   100,
			Stream:      false,
		}
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		var got ChatRequest
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if got.Model != "gpt-4" {
			t.Errorf("expected model gpt-4, got %s", got.Model)
		}
		if got.Stream != false {
			t.Errorf("expected stream false, got %v", got.Stream)
		}
		if got.Temperature != 0.5 {
			t.Errorf("expected temperature 0.5, got %v", got.Temperature)
		}
		if got.MaxTokens != 100 {
			t.Errorf("expected max_tokens 100, got %d", got.MaxTokens)
		}
	})

	t.Run("temperature and max_tokens omitempty", func(t *testing.T) {
		req := ChatRequest{Model: "m", Messages: nil, Stream: true}
		data, _ := json.Marshal(req)
		s := string(data)
		if strings.Contains(s, "temperature") {
			t.Errorf("temperature should be omitted when zero, got %s", s)
		}
		if strings.Contains(s, "max_tokens") {
			t.Errorf("max_tokens should be omitted when zero, got %s", s)
		}
		if !strings.Contains(s, `"stream":true`) {
			t.Errorf("stream should always be present, got %s", s)
		}
	})
}

func TestNewClient(t *testing.T) {
	t.Run("trims trailing slash", func(t *testing.T) {
		c := NewClient("http://api.example.com/v1/", "key", "model")
		if c.Endpoint != "http://api.example.com/v1" {
			t.Errorf("expected trimmed endpoint, got %s", c.Endpoint)
		}
	})

	t.Run("trims /chat/completions suffix", func(t *testing.T) {
		c := NewClient("http://api.example.com/v1/chat/completions", "key", "model")
		if c.Endpoint != "http://api.example.com/v1" {
			t.Errorf("expected trimmed endpoint, got %s", c.Endpoint)
		}
	})

	t.Run("trims trailing slash and suffix combined", func(t *testing.T) {
		c := NewClient("http://api.example.com/v1/chat/completions/", "key", "model")
		if c.Endpoint != "http://api.example.com/v1" {
			t.Errorf("expected trimmed endpoint, got %s", c.Endpoint)
		}
	})

	t.Run("no trim needed", func(t *testing.T) {
		c := NewClient("http://api.example.com/v1", "key", "model")
		if c.Endpoint != "http://api.example.com/v1" {
			t.Errorf("expected unchanged endpoint, got %s", c.Endpoint)
		}
	})

	t.Run("sets fields", func(t *testing.T) {
		c := NewClient("http://api.example.com", "my-key", "my-model")
		if c.APIKey != "my-key" {
			t.Errorf("expected APIKey my-key, got %s", c.APIKey)
		}
		if c.Model != "my-model" {
			t.Errorf("expected Model my-model, got %s", c.Model)
		}
		if c.HTTPClient == nil {
			t.Error("expected non-nil HTTPClient")
		}
	})
}

func TestClient_Chat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("expected Bearer auth, got %s", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected json content-type, got %s", r.Header.Get("Content-Type"))
			}
			var req ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request failed: %v", err)
			}
			if req.Model != "test-model" {
				t.Errorf("expected model test-model, got %s", req.Model)
			}
			if req.Stream != false {
				t.Errorf("expected stream false, got %v", req.Stream)
			}
			if len(req.Messages) != 1 || req.Messages[0].Content != "hi" {
				t.Errorf("unexpected messages: %+v", req.Messages)
			}

			resp := ChatResponse{
				ID:    "chat-1",
				Model: "test-model",
				Choices: []ChatChoice{
					{Index: 0, Message: Message{Role: "assistant", Content: "hello world"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "test-key", "test-model")
		result, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "hello world" {
			t.Errorf("expected 'hello world', got %q", result)
		}
	})

	t.Run("http error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"message":"invalid api key"}}`)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		_, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("expected 401 in error, got %v", err)
		}
	})

	t.Run("api error in body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{
				Error: &ChatError{Message: "model overloaded", Type: "server_error"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		_, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "model overloaded") {
			t.Errorf("expected 'model overloaded' in error, got %v", err)
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ChatResponse{ID: "empty"}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		result, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "" {
			t.Errorf("expected empty result, got %q", result)
		}
	})
}

func TestClient_ChatStream(t *testing.T) {
	t.Run("success SSE parsing", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") != "text/event-stream" {
				t.Errorf("expected text/event-stream accept, got %s", r.Header.Get("Accept"))
			}
			var req ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if req.Stream != true {
				t.Errorf("expected stream true, got %v", req.Stream)
			}

			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		var chunks []string
		var doneCalls int
		full, err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100, func(chunk string, done bool, err error) {
			if done {
				doneCalls++
			}
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if full != "hello world" {
			t.Errorf("expected 'hello world', got %q", full)
		}
		if len(chunks) != 2 {
			t.Fatalf("expected 2 chunks, got %d", len(chunks))
		}
		if chunks[0] != "hello" || chunks[1] != " world" {
			t.Errorf("unexpected chunks: %v", chunks)
		}
		if doneCalls != 1 {
			t.Errorf("expected 1 done callback, got %d", doneCalls)
		}
	})

	t.Run("http error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "server error")
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		_, err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100, func(chunk string, done bool, err error) {})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("expected 500 in error, got %v", err)
		}
	})

	t.Run("api error in chunk", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"error\":{\"message\":\"rate limited\"}}\n\n")
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		var callbackErr error
		_, err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100, func(chunk string, done bool, err error) {
			if err != nil {
				callbackErr = err
			}
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "rate limited") {
			t.Errorf("expected 'rate limited' in error, got %v", err)
		}
		if callbackErr == nil {
			t.Error("expected callback to receive error")
		}
	})

	t.Run("empty stream with only DONE", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		full, err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100, func(chunk string, done bool, err error) {})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if full != "" {
			t.Errorf("expected empty result, got %q", full)
		}
	})

	t.Run("ignores non-data lines", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, ": comment line\n\n")
			fmt.Fprint(w, "event: ping\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}))
		defer srv.Close()

		client := NewClient(srv.URL, "key", "model")
		full, err := client.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, 0.7, 100, func(chunk string, done bool, err error) {})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if full != "ok" {
			t.Errorf("expected 'ok', got %q", full)
		}
	})
}
