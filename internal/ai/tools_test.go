package ai

import (
	"strings"
	"testing"
	"time"
)

func TestIsDangerous(t *testing.T) {
	dangerous := []string{"write_file", "delete_file", "run_build", "run_command", "overwrite_file"}
	for _, tool := range dangerous {
		t.Run("dangerous_"+tool, func(t *testing.T) {
			if !IsDangerous(tool) {
				t.Errorf("expected %s to be dangerous", tool)
			}
		})
	}

	safe := []string{"read_file", "list_files", "search", "query", ""}
	for _, tool := range safe {
		t.Run("safe_"+tool, func(t *testing.T) {
			if IsDangerous(tool) {
				t.Errorf("expected %s to not be dangerous", tool)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	t.Run("has tc_ prefix", func(t *testing.T) {
		id := generateID()
		if !strings.HasPrefix(id, "tc_") {
			t.Errorf("expected tc_ prefix, got %s", id)
		}
	})

	t.Run("unique across calls", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := generateID()
			if ids[id] {
				t.Errorf("duplicate id generated: %s", id)
			}
			ids[id] = true
		}
	})
}

func TestNewToolManager(t *testing.T) {
	t.Run("nil emitter", func(t *testing.T) {
		tm := NewToolManager(nil)
		if tm == nil {
			t.Fatal("expected non-nil manager")
		}
		if tm.PendingCount() != 0 {
			t.Errorf("expected 0 pending, got %d", tm.PendingCount())
		}
	})

	t.Run("with emitter", func(t *testing.T) {
		tm := NewToolManager(func(req *ToolConfirmRequest) {})
		if tm == nil {
			t.Fatal("expected non-nil manager")
		}
		if tm.PendingCount() != 0 {
			t.Errorf("expected 0 pending, got %d", tm.PendingCount())
		}
	})
}

func TestToolManager_RequestConfirmation(t *testing.T) {
	t.Run("sets fields for dangerous tool", func(t *testing.T) {
		var emitted *ToolConfirmRequest
		tm := NewToolManager(func(req *ToolConfirmRequest) {
			emitted = req
		})
		req := tm.RequestConfirmation("write_file", "overwrite test.txt", map[string]string{"path": "/tmp/test.txt"})

		if req.ID == "" {
			t.Error("expected non-empty ID")
		}
		if !strings.HasPrefix(req.ID, "tc_") {
			t.Errorf("expected tc_ prefix, got %s", req.ID)
		}
		if req.Tool != "write_file" {
			t.Errorf("expected tool write_file, got %s", req.Tool)
		}
		if req.Summary != "overwrite test.txt" {
			t.Errorf("expected summary, got %s", req.Summary)
		}
		if req.Params["path"] != "/tmp/test.txt" {
			t.Errorf("expected path param, got %s", req.Params["path"])
		}
		if req.Risk != RiskDangerous {
			t.Errorf("expected RiskDangerous, got %s", req.Risk)
		}
		if req.CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt")
		}
		if tm.PendingCount() != 1 {
			t.Errorf("expected 1 pending, got %d", tm.PendingCount())
		}
		if emitted == nil {
			t.Fatal("expected emitter to be called")
		}
		if emitted.ID != req.ID {
			t.Errorf("expected emitted req to match, got %s vs %s", emitted.ID, req.ID)
		}
	})

	t.Run("sets moderate risk for safe tool", func(t *testing.T) {
		tm := NewToolManager(nil)
		req := tm.RequestConfirmation("read_file", "read config", nil)
		if req.Risk != RiskModerate {
			t.Errorf("expected RiskModerate, got %s", req.Risk)
		}
	})

	t.Run("emitter not called when nil", func(t *testing.T) {
		tm := NewToolManager(nil)
		// Should not panic
		req := tm.RequestConfirmation("write_file", "test", nil)
		if req == nil {
			t.Error("expected non-nil request")
		}
	})

	t.Run("multiple requests get unique IDs", func(t *testing.T) {
		tm := NewToolManager(nil)
		req1 := tm.RequestConfirmation("write_file", "task1", nil)
		req2 := tm.RequestConfirmation("delete_file", "task2", nil)
		if req1.ID == req2.ID {
			t.Error("expected different IDs")
		}
		if tm.PendingCount() != 2 {
			t.Errorf("expected 2 pending, got %d", tm.PendingCount())
		}
	})
}

func TestToolManager_ConfirmToolCall(t *testing.T) {
	t.Run("approved true returns result via Wait", func(t *testing.T) {
		tm := NewToolManager(nil)
		req := tm.RequestConfirmation("write_file", "test", nil)

		// Confirm before Wait - channel is buffered (cap 1)
		ok := tm.ConfirmToolCall(req.ID, true)
		if !ok {
			t.Error("expected ConfirmToolCall to return true")
		}

		approved, waitErr := req.Wait()
		if waitErr != nil {
			t.Errorf("expected no error from Wait, got %v", waitErr)
		}
		if !approved {
			t.Error("expected approved true")
		}
		if tm.PendingCount() != 0 {
			t.Errorf("expected 0 pending after confirm, got %d", tm.PendingCount())
		}
	})

	t.Run("approved false returns result via Wait", func(t *testing.T) {
		tm := NewToolManager(nil)
		req := tm.RequestConfirmation("delete_file", "test", nil)

		ok := tm.ConfirmToolCall(req.ID, false)
		if !ok {
			t.Error("expected ConfirmToolCall to return true")
		}

		approved, waitErr := req.Wait()
		if waitErr != nil {
			t.Errorf("expected no error, got %v", waitErr)
		}
		if approved {
			t.Error("expected approved false")
		}
	})

	t.Run("non-existent id returns false", func(t *testing.T) {
		tm := NewToolManager(nil)
		ok := tm.ConfirmToolCall("nonexistent", true)
		if ok {
			t.Error("expected false for non-existent id")
		}
	})

	t.Run("confirm twice second returns false", func(t *testing.T) {
		tm := NewToolManager(nil)
		req := tm.RequestConfirmation("write_file", "test", nil)

		ok1 := tm.ConfirmToolCall(req.ID, true)
		if !ok1 {
			t.Error("expected first confirm to return true")
		}
		ok2 := tm.ConfirmToolCall(req.ID, true)
		if ok2 {
			t.Error("expected second confirm to return false")
		}
	})

	t.Run("Wait then confirm works", func(t *testing.T) {
		tm := NewToolManager(nil)
		req := tm.RequestConfirmation("write_file", "test", nil)

		// Use a goroutine to Wait, then confirm
		type result struct {
			approved bool
			err      error
		}
		ch := make(chan result, 1)
		go func() {
			a, err := req.Wait()
			ch <- result{a, err}
		}()

		// Small delay to ensure goroutine is waiting
		time.Sleep(50 * time.Millisecond)
		tm.ConfirmToolCall(req.ID, true)

		select {
		case r := <-ch:
			if r.err != nil {
				t.Errorf("expected no error, got %v", r.err)
			}
			if !r.approved {
				t.Error("expected approved true")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Wait did not return within 1 second")
		}
	})
}

func TestToolManager_Cancel(t *testing.T) {
	t.Run("cancel pending request", func(t *testing.T) {
		tm := NewToolManager(nil)
		req := tm.RequestConfirmation("write_file", "test", nil)

		ok := tm.Cancel(req.ID)
		if !ok {
			t.Error("expected Cancel to return true")
		}

		approved, waitErr := req.Wait()
		if waitErr != nil {
			t.Errorf("expected no error, got %v", waitErr)
		}
		if approved {
			t.Error("expected approved false after cancel")
		}
		if tm.PendingCount() != 0 {
			t.Errorf("expected 0 pending after cancel, got %d", tm.PendingCount())
		}
	})

	t.Run("cancel non-existent returns false", func(t *testing.T) {
		tm := NewToolManager(nil)
		ok := tm.Cancel("nonexistent")
		if ok {
			t.Error("expected false for non-existent id")
		}
	})

	t.Run("cancel after confirm returns false", func(t *testing.T) {
		tm := NewToolManager(nil)
		req := tm.RequestConfirmation("write_file", "test", nil)
		tm.ConfirmToolCall(req.ID, true)
		ok := tm.Cancel(req.ID)
		if ok {
			t.Error("expected false for cancel after confirm")
		}
	})
}

func TestToolManager_ListPending(t *testing.T) {
	t.Run("empty when no requests", func(t *testing.T) {
		tm := NewToolManager(nil)
		list := tm.ListPending()
		if len(list) != 0 {
			t.Errorf("expected 0 pending, got %d", len(list))
		}
	})

	t.Run("returns all pending requests", func(t *testing.T) {
		tm := NewToolManager(nil)
		req1 := tm.RequestConfirmation("write_file", "task1", nil)
		req2 := tm.RequestConfirmation("delete_file", "task2", nil)
		list := tm.ListPending()
		if len(list) != 2 {
			t.Fatalf("expected 2 pending, got %d", len(list))
		}
		ids := map[string]bool{req1.ID: true, req2.ID: true}
		for _, r := range list {
			if !ids[r.ID] {
				t.Errorf("unexpected id %s", r.ID)
			}
		}
	})

	t.Run("empty after all confirmed", func(t *testing.T) {
		tm := NewToolManager(nil)
		req := tm.RequestConfirmation("write_file", "test", nil)
		tm.ConfirmToolCall(req.ID, true)
		list := tm.ListPending()
		if len(list) != 0 {
			t.Errorf("expected 0 pending, got %d", len(list))
		}
	})
}

func TestToolManager_PendingCount(t *testing.T) {
	tm := NewToolManager(nil)
	if tm.PendingCount() != 0 {
		t.Fatalf("expected 0, got %d", tm.PendingCount())
	}
	req1 := tm.RequestConfirmation("write_file", "t1", nil)
	if tm.PendingCount() != 1 {
		t.Errorf("expected 1, got %d", tm.PendingCount())
	}
	req2 := tm.RequestConfirmation("write_file", "t2", nil)
	if tm.PendingCount() != 2 {
		t.Errorf("expected 2, got %d", tm.PendingCount())
	}
	tm.ConfirmToolCall(req1.ID, true)
	if tm.PendingCount() != 1 {
		t.Errorf("expected 1 after confirm, got %d", tm.PendingCount())
	}
	tm.Cancel(req2.ID)
	if tm.PendingCount() != 0 {
		t.Errorf("expected 0 after cancel, got %d", tm.PendingCount())
	}
}

func TestToolManager_CleanupExpired(t *testing.T) {
	t.Run("no expired when fresh", func(t *testing.T) {
		tm := NewToolManager(nil)
		tm.RequestConfirmation("write_file", "test", nil)
		count := tm.CleanupExpired()
		if count != 0 {
			t.Errorf("expected 0 expired, got %d", count)
		}
		if tm.PendingCount() != 1 {
			t.Errorf("expected 1 still pending, got %d", tm.PendingCount())
		}
	})

	t.Run("removes expired requests", func(t *testing.T) {
		tm := NewToolManager(nil)
		req1 := tm.RequestConfirmation("write_file", "test1", nil)
		req2 := tm.RequestConfirmation("write_file", "test2", nil)

		// Manually age req1 by modifying CreatedAt
		tm.mu.Lock()
		req1.CreatedAt = time.Now().Add(-10 * time.Minute)
		tm.mu.Unlock()

		count := tm.CleanupExpired()
		if count != 1 {
			t.Errorf("expected 1 expired, got %d", count)
		}
		if tm.PendingCount() != 1 {
			t.Errorf("expected 1 still pending, got %d", tm.PendingCount())
		}
		// req2 should still be there
		list := tm.ListPending()
		if len(list) != 1 || list[0].ID != req2.ID {
			t.Errorf("expected req2 to remain, got %v", list)
		}
	})

	t.Run("removes multiple expired", func(t *testing.T) {
		tm := NewToolManager(nil)
		req1 := tm.RequestConfirmation("write_file", "test1", nil)
		req2 := tm.RequestConfirmation("write_file", "test2", nil)
		req3 := tm.RequestConfirmation("write_file", "test3", nil)

		tm.mu.Lock()
		req1.CreatedAt = time.Now().Add(-10 * time.Minute)
		req2.CreatedAt = time.Now().Add(-6 * time.Minute)
		tm.mu.Unlock()

		count := tm.CleanupExpired()
		if count != 2 {
			t.Errorf("expected 2 expired, got %d", count)
		}
		if tm.PendingCount() != 1 {
			t.Errorf("expected 1 remaining, got %d", tm.PendingCount())
		}
		list := tm.ListPending()
		if len(list) != 1 || list[0].ID != req3.ID {
			t.Errorf("expected req3 to remain, got %v", list)
		}
	})
}

// 注意:Wait() 的 30 秒超时由于是硬编码,直接测试会使测试套件过慢。
// 上面的 ConfirmToolCall 和 Cancel 路径已覆盖相同的 select/channel 机制。
// 这里用一个非阻塞方式验证 Wait 在无确认时会阻塞(不等待 30 秒)。
func TestToolManager_Wait_BlocksWithoutConfirmation(t *testing.T) {
	tm := NewToolManager(nil)
	req := tm.RequestConfirmation("write_file", "test", nil)

	type waitResult struct {
		approved bool
		err      error
	}
	done := make(chan waitResult, 1)

	go func() {
		a, err := req.Wait()
		done <- waitResult{a, err}
	}()

	// Wait should still be blocking after 100ms
	select {
	case <-done:
		t.Fatal("Wait returned before confirmation - should be blocking")
	case <-time.After(100 * time.Millisecond):
		// Expected: still blocking
	}

	// Clean up by cancelling
	tm.Cancel(req.ID)

	select {
	case r := <-done:
		if r.approved {
			t.Error("expected approved false after cancel")
		}
		if r.err != nil {
			t.Errorf("expected no error, got %v", r.err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Wait did not return after cancel within 1 second")
	}
}
