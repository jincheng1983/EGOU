// Package ai — 危险工具人机确认（P2-11，吸取 NxEGO3）
//
// 设计目标：
//   - AI 调用写文件/删除文件/执行命令等危险工具时，阻塞等待前端确认
//   - confirmChans map 管理待确认请求，30 秒超时自动拒绝
//   - 事件名 ai-tool-confirm 通过 Wails Events 发送到前端
//   - 前端 ConfirmToolCall(toolCallID, approved) 回写结果
//
// 使用示例（在 IDEService 中）：
//   tm := ai.NewToolManager(app)
//   req := tm.RequestConfirmation("write_file", "/path/to/file", "覆盖现有文件？")
//   // tm 会通过 Wails 事件通知前端，前端调用 ConfirmToolCall(req.ID, true) 回写
//   approved, err := req.Wait()  // 阻塞直到确认或超时
//   if approved { 执行写文件 }
package ai

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ToolRiskLevel 工具风险等级
type ToolRiskLevel string

const (
	RiskSafe     ToolRiskLevel = "safe"      // 安全：读取文件、查询
	RiskModerate ToolRiskLevel = "moderate"  // 中等：创建新文件、修改配置
	RiskDangerous ToolRiskLevel = "dangerous" // 危险：覆盖文件、删除、执行命令
)

// IsDangerous 判断工具是否需要人机确认
func IsDangerous(toolName string) bool {
	switch toolName {
	case "write_file", "delete_file", "run_build", "run_command", "overwrite_file":
		return true
	}
	return false
}

// ToolConfirmRequest 是一次工具确认请求
type ToolConfirmRequest struct {
	ID        string         // 请求 ID（前端回写时用）
	Tool      string         // 工具名
	Summary   string         // 给用户看的简短说明
	Params    map[string]string // 工具参数
	Risk      ToolRiskLevel  // 风险等级
	CreatedAt time.Time
	ch        chan bool      // 确认结果通道
	done      bool
	mu        sync.Mutex
}

// Wait 阻塞等待确认结果，超时返回 false
func (r *ToolConfirmRequest) Wait() (bool, error) {
	select {
	case approved := <-r.ch:
		return approved, nil
	case <-time.After(30 * time.Second):
		r.mu.Lock()
		r.done = true
		r.mu.Unlock()
		return false, fmt.Errorf("确认超时（30秒）")
	}
}

// ToolManager 管理待确认的工具调用请求
type ToolManager struct {
	mu       sync.RWMutex
	pending  map[string]*ToolConfirmRequest // requestID -> request
	emitter  func(req *ToolConfirmRequest)  // 事件发射器（Wails Events）
}

// NewToolManager 创建工具管理器
// emitter 用于把请求发送到前端（如通过 Wails Events.Emit）
func NewToolManager(emitter func(req *ToolConfirmRequest)) *ToolManager {
	return &ToolManager{
		pending:  map[string]*ToolConfirmRequest{},
		emitter:  emitter,
	}
}

// generateID 生成请求 ID（16 字符 hex）
func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("tc_%d", time.Now().UnixNano())
	}
	return "tc_" + hex.EncodeToString(b)
}

// RequestConfirmation 发起一次工具确认请求
// 返回请求对象，调用 req.Wait() 阻塞等待结果
func (tm *ToolManager) RequestConfirmation(tool string, summary string, params map[string]string) *ToolConfirmRequest {
	risk := RiskDangerous
	if !IsDangerous(tool) {
		risk = RiskModerate
	}
	req := &ToolConfirmRequest{
		ID:        generateID(),
		Tool:      tool,
		Summary:   summary,
		Params:    params,
		Risk:      risk,
		CreatedAt: time.Now(),
		ch:        make(chan bool, 1),
	}

	tm.mu.Lock()
	tm.pending[req.ID] = req
	tm.mu.Unlock()

	// 通过 emitter 发送事件到前端
	if tm.emitter != nil {
		tm.emitter(req)
	}

	return req
}

// ConfirmToolCall 由前端回写确认结果
// approved=true 表示用户同意，false 表示拒绝
// 返回 false 表示请求不存在或已被处理
func (tm *ToolManager) ConfirmToolCall(requestID string, approved bool) bool {
	tm.mu.Lock()
	req, ok := tm.pending[requestID]
	if !ok {
		tm.mu.Unlock()
		return false
	}
	delete(tm.pending, requestID)
	tm.mu.Unlock()

	req.mu.Lock()
	defer req.mu.Unlock()
	if req.done {
		return false
	}
	req.done = true
	// 非阻塞写入（channel 有 1 缓冲）
	req.ch <- approved
	return true
}

// PendingCount 返回待确认的请求数量
func (tm *ToolManager) PendingCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.pending)
}

// ListPending 返回所有待确认请求的列表（用于前端刷新）
func (tm *ToolManager) ListPending() []*ToolConfirmRequest {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	out := make([]*ToolConfirmRequest, 0, len(tm.pending))
	for _, req := range tm.pending {
		out = append(out, req)
	}
	return out
}

// Cancel 主动取消一个待确认请求（如用户关闭对话框）
func (tm *ToolManager) Cancel(requestID string) bool {
	tm.mu.Lock()
	req, ok := tm.pending[requestID]
	if !ok {
		tm.mu.Unlock()
		return false
	}
	delete(tm.pending, requestID)
	tm.mu.Unlock()

	req.mu.Lock()
	defer req.mu.Unlock()
	if req.done {
		return false
	}
	req.done = true
	req.ch <- false
	return true
}

// CleanupExpired 清理超过 5 分钟仍未确认的请求（防止内存泄漏）
func (tm *ToolManager) CleanupExpired() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	expired := []string{}
	for id, req := range tm.pending {
		if time.Since(req.CreatedAt) > 5*time.Minute {
			expired = append(expired, id)
		}
	}
	for _, id := range expired {
		delete(tm.pending, id)
	}
	return len(expired)
}
