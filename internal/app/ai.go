// ai.go 实现 AI 编排相关方法：单 Agent / Pipeline / 流式 Chat / 工具确认 / BuildAndFix。
//
// 第七版对应方法直接迁移，仅按第八版命名规约重命名：
//   - "NxEGOU 中文编程语言" → "EGOU 中文编程语言"

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"egou/internal/ai"
	"egou/internal/runner"
)

// AIAgentResult 是 Agent 执行结果（前端绑定用）
type AIAgentResult struct {
	Role      string        `json:"role"`
	Output    string        `json:"output"`
	ToolCalls []ai.ToolCall `json:"toolCalls,omitempty"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
}

// AIMessage 单条对话消息
type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RunAgent 调用单个 Agent 角色（P2-9 接入）
// 通过事件 ide:ai-agent-event 回传 agent-start/agent-end 状态
func (s *IDEService) RunAgent(endpoint, apiKey, model string, role string, userInput string, history []AIMessage) AIAgentResult {
	client := ai.NewClient(endpoint, apiKey, model)
	orch := ai.NewOrchestrator(client)
	orch.SetSink(func(ev ai.PipelineEvent) {
		if s.app != nil {
			s.app.Event.Emit("ide:ai-agent-event", map[string]any{
				"stage": ev.Stage,
				"role":  string(ev.Role),
				"index": ev.Index,
				"total": ev.Total,
			})
		}
	})

	var hist []ai.Message
	for _, m := range history {
		hist = append(hist, ai.Message{Role: m.Role, Content: m.Content})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := orch.RunAgent(ctx, ai.AgentRole(role), userInput, hist)
	if err != nil {
		return AIAgentResult{
			Role:    role,
			Error:   err.Error(),
			Success: false,
		}
	}
	return AIAgentResult{
		Role:      string(result.Role),
		Output:    result.Output,
		ToolCalls: result.ToolCalls,
		Success:   result.Success,
		Error:     result.Error,
	}
}

// RunAgentPipeline 串行执行多个 Agent（P2-9 接入）
// pipeline 是角色 ID 数组，如 ["planner", "coder", "reviewer"]
func (s *IDEService) RunAgentPipeline(endpoint, apiKey, model string, pipeline []string, userInput string, history []AIMessage) AIAgentResult {
	client := ai.NewClient(endpoint, apiKey, model)
	orch := ai.NewOrchestrator(client)
	orch.SetSink(func(ev ai.PipelineEvent) {
		if s.app != nil {
			s.app.Event.Emit("ide:ai-agent-event", map[string]any{
				"stage": ev.Stage,
				"role":  string(ev.Role),
				"index": ev.Index,
				"total": ev.Total,
			})
		}
	})

	var roles []ai.AgentRole
	for _, r := range pipeline {
		roles = append(roles, ai.AgentRole(r))
	}

	var hist []ai.Message
	for _, m := range history {
		hist = append(hist, ai.Message{Role: m.Role, Content: m.Content})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	result, err := orch.RunPipeline(ctx, roles, userInput, hist)
	if err != nil {
		return AIAgentResult{
			Role:    string(result.Role),
			Error:   err.Error(),
			Success: false,
		}
	}
	return AIAgentResult{
		Role:      string(result.Role),
		Output:    result.Output,
		ToolCalls: result.ToolCalls,
		Success:   result.Success,
		Error:     result.Error,
	}
}

// ListAgentRoles 列出所有可用的 Agent 角色配置（P2-9 接入）
func (s *IDEService) ListAgentRoles() []map[string]any {
	configs := ai.AllAgentConfigs()
	out := make([]map[string]any, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, map[string]any{
			"role":          string(cfg.Role),
			"name":          cfg.Name,
			"description":   cfg.Description,
			"tools":         cfg.Tools,
			"maxIterations": cfg.MaxIterations,
		})
	}
	return out
}

// ConfirmToolCall 前端回写工具确认结果（P2-11 接入）
// approved=true 表示用户同意执行，false 表示拒绝
func (s *IDEService) ConfirmToolCall(requestID string, approved bool) bool {
	if s.toolManager == nil {
		return false
	}
	return s.toolManager.ConfirmToolCall(requestID, approved)
}

// ListPendingToolCalls 列出所有待确认的工具调用请求（P2-11 接入）
func (s *IDEService) ListPendingToolCalls() []map[string]any {
	if s.toolManager == nil {
		return []map[string]any{}
	}
	pending := s.toolManager.ListPending()
	out := make([]map[string]any, 0, len(pending))
	for _, req := range pending {
		out = append(out, map[string]any{
			"id":        req.ID,
			"tool":      req.Tool,
			"summary":   req.Summary,
			"params":    req.Params,
			"risk":      string(req.Risk),
			"createdAt": req.CreatedAt.Unix(),
		})
	}
	return out
}

// BuildAndFix 编译失败后自动调用 Fixer Agent 修复（P2-12 接入）
// 通过事件 ide:ai-fix-status 回传修复进度
// maxRounds=0 时使用默认值 3
// projectPath 必须非空：修复过程中会重新编译，产物必须输出到项目目录，
// 避免污染 IDE 工作目录，也保证多开 IDE 与多项目同时运行时互不干扰。
func (s *IDEService) BuildAndFix(endpoint, apiKey, model string, source string, projectPath string, maxRounds int) {
	if projectPath == "" {
		if s.app != nil {
			s.app.Event.Emit("ide:ai-fix-status", map[string]any{
				"stage": "done",
				"error": "未打开项目，无法编译修复（请先新建或打开项目）",
			})
		}
		return
	}
	go func() {
		client := ai.NewClient(endpoint, apiKey, model)

		buildFn := func(src, projPath string) (string, error) {
			var combined string
			sink := func(ev runner.Event) {
				s.emitEvent(ev)
				combined += ev.Output + "\n"
			}
			_, err := runner.BuildSource(src, projPath, sink)
			return combined, err
		}

		fixSink := func(ev ai.FixEvent) {
			if s.app != nil {
				s.app.Event.Emit("ide:ai-fix-status", map[string]any{
					"stage":     ev.Stage,
					"round":     ev.Round,
					"errors":    ev.Errors,
					"fixOutput": ev.FixOutput,
					"source":    ev.Source,
				})
			}
		}

		finalSrc, output, success, err := ai.BuildAndFix(source, projectPath, buildFn, client, maxRounds, fixSink)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		if s.app != nil {
			s.app.Event.Emit("ide:ai-fix-status", map[string]any{
				"stage":   "done",
				"source":  finalSrc,
				"output":  output,
				"success": success,
				"error":   errMsg,
			})
		}
	}()
}

// AIChat 流式调用 OpenAI 兼容 API，通过事件 ide:ai-chunk 回传内容
// event data: { done: bool, content: string, error: string }
func (s *IDEService) AIChat(endpoint, apiKey, model string, messages []AIMessage, temperature float64, maxTokens int, systemPrompt string, projectMemory string) {
	go func() {
		client := ai.NewClient(endpoint, apiKey, model)

		allMsgs := make([]ai.Message, 0, len(messages)+2)
		sysParts := []string{"你是 EGOU 中文编程语言的 AI 助手。"}
		if systemPrompt != "" {
			sysParts = append(sysParts, systemPrompt)
		}
		if projectMemory != "" {
			sysParts = append(sysParts, "\n【项目记忆】\n"+projectMemory)
		}
		allMsgs = append(allMsgs, ai.Message{Role: "system", Content: strings.Join(sysParts, "\n")})

		for _, m := range messages {
			allMsgs = append(allMsgs, ai.Message{Role: m.Role, Content: m.Content})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		if temperature <= 0 {
			temperature = 0.7
		}
		if maxTokens <= 0 {
			maxTokens = 4096
		}

		_, err := client.ChatStream(ctx, allMsgs, temperature, maxTokens, func(chunk string, done bool, err error) {
			data := map[string]any{"done": done}
			if err != nil {
				data["error"] = err.Error()
			} else {
				data["content"] = chunk
			}
			s.app.Event.Emit("ide:ai-chunk", data)
		})
		if err != nil {
			s.app.Event.Emit("ide:ai-chunk", map[string]any{"done": true, "error": err.Error()})
		}
	}()
}

// ReadProjectMemory 读取项目级 AI 记忆（<project>/.eg/memory/memory.md）。
// 文件不存在时返回空字符串（不视为错误，表示新项目无记忆）。
// projectPath 为空或路径无效返回空串。
func (s *IDEService) ReadProjectMemory(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	memPath := filepath.Join(projectPath, ".eg", "memory", "memory.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		return ""
	}
	return string(data)
}

// SaveProjectMemory 写入项目级 AI 记忆。
// 自动创建 .eg/memory/ 目录。返回空字符串表示成功，非空表示错误信息。
func (s *IDEService) SaveProjectMemory(projectPath string, content string) string {
	if projectPath == "" {
		return "项目路径为空"
	}
	memDir := filepath.Join(projectPath, ".eg", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return "创建记忆目录失败: " + err.Error()
	}
	memPath := filepath.Join(memDir, "memory.md")
	if err := os.WriteFile(memPath, []byte(content), 0644); err != nil {
		return "写入记忆失败: " + err.Error()
	}
	return ""
}
