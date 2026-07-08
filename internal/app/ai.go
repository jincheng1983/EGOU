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
// projectPath 非空时，自动读取 .eg/memory/ 注入到 Agent 系统提示词。
func (s *IDEService) RunAgent(endpoint, apiKey, model string, role string, userInput string, history []AIMessage, projectPath string) AIAgentResult {
	client := ai.NewClient(endpoint, apiKey, model)
	orch := ai.NewOrchestrator(client)
	// 注入项目记忆（v0.11.0）
	if projectPath != "" {
		if mem := s.ReadProjectMemory(projectPath); mem != "" {
			orch.SetProjectContext(mem)
		}
	}
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
// projectPath 非空时，自动读取 .eg/memory/ 注入到每个 Agent 系统提示词。
func (s *IDEService) RunAgentPipeline(endpoint, apiKey, model string, pipeline []string, userInput string, history []AIMessage, projectPath string) AIAgentResult {
	client := ai.NewClient(endpoint, apiKey, model)
	orch := ai.NewOrchestrator(client)
	// 注入项目记忆（v0.11.0）
	if projectPath != "" {
		if mem := s.ReadProjectMemory(projectPath); mem != "" {
			orch.SetProjectContext(mem)
		}
	}
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

		// 注入项目记忆到 Fixer Agent（v0.11.0）
		var projectCtx string
		if mem := s.ReadProjectMemory(projectPath); mem != "" {
			projectCtx = mem
		}
		finalSrc, output, success, err := ai.BuildAndFixWithContext(source, projectPath, buildFn, client, maxRounds, fixSink, projectCtx)
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
// projectPath 参数：传入项目路径时，AI 完整响应 done 后自动提取关键决策追加到 .eg/memory/decisions.md。
//                  传空串则跳过决策提取（向后兼容旧调用）。
func (s *IDEService) AIChat(endpoint, apiKey, model string, messages []AIMessage, temperature float64, maxTokens int, systemPrompt string, projectMemory string, projectPath string) {
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

		// 收集完整响应，done 时用于决策提取
		var fullResponse strings.Builder

		_, err := client.ChatStream(ctx, allMsgs, temperature, maxTokens, func(chunk string, done bool, err error) {
			data := map[string]any{"done": done}
			if err != nil {
				data["error"] = err.Error()
			} else {
				data["content"] = chunk
				if !done {
					fullResponse.WriteString(chunk)
				}
			}
			s.app.Event.Emit("ide:ai-chunk", data)
		})
		if err != nil {
			s.app.Event.Emit("ide:ai-chunk", map[string]any{"done": true, "error": err.Error()})
			return
		}
		// AI 响应结束后，启发式提取关键决策并追加到 decisions.md
		if projectPath != "" {
			resp := fullResponse.String()
			if decisions := extractDecisions(resp); len(decisions) > 0 {
				s.AppendProjectMemorySection(projectPath, "decisions", strings.Join(decisions, "\n"))
			}
		}
	}()
}

// ReadProjectMemory 读取项目级 AI 记忆。
// 优先读取结构化记忆（summary.md + decisions.md + 用户备注 memory.md 拼接），
// 向后兼容：若结构化文件不存在但旧 memory.md 存在，直接返回旧文件内容。
// projectPath 为空或路径无效返回空串。
func (s *IDEService) ReadProjectMemory(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	memDir := filepath.Join(projectPath, ".eg", "memory")
	summaryPath := filepath.Join(memDir, "summary.md")
	decisionsPath := filepath.Join(memDir, "decisions.md")
	userNotesPath := filepath.Join(memDir, "memory.md")

	var parts []string
	// 1. 滚动摘要（AI 自动压缩生成）
	if data, err := os.ReadFile(summaryPath); err == nil {
		summary := strings.TrimSpace(string(data))
		if summary != "" {
			parts = append(parts, "## 滚动摘要\n"+summary)
		}
	}
	// 2. 关键决策（AI 自动提取追加）
	if data, err := os.ReadFile(decisionsPath); err == nil {
		decisions := strings.TrimSpace(string(data))
		if decisions != "" {
			parts = append(parts, "## 关键决策\n" + decisions)
		}
	}
	// 3. 用户备注（手写，向后兼容旧 memory.md）
	if data, err := os.ReadFile(userNotesPath); err == nil {
		notes := strings.TrimSpace(string(data))
		if notes != "" {
			parts = append(parts, "## 用户备注\n" + notes)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// SaveProjectMemory 写入项目级 AI 记忆（用户备注部分，memory.md）。
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

// AppendProjectMemorySection 追加式写入项目记忆的某个分段（summary 或 decisions）。
// section 为 "summary" 或 "decisions"，content 为要追加的文本（可含多行）。
// 自动在内容前加时间戳标记。返回空字符串表示成功。
func (s *IDEService) AppendProjectMemorySection(projectPath, section, content string) string {
	if projectPath == "" {
		return "项目路径为空"
	}
	if section != "summary" && section != "decisions" {
		return "无效的记忆分段: " + section
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	memDir := filepath.Join(projectPath, ".eg", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return "创建记忆目录失败: " + err.Error()
	}
	filePath := filepath.Join(memDir, section+".md")
	// 读取现有内容（追加模式，不覆盖）
	existing := ""
	if data, err := os.ReadFile(filePath); err == nil {
		existing = string(data)
	}
	// 追加时间戳标记 + 新内容
	stamp := time.Now().Format("2006-01-02 15:04")
	var b strings.Builder
	if existing != "" {
		b.WriteString(existing)
		if !strings.HasSuffix(existing, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("### @ " + stamp + "\n")
	b.WriteString(content)
	b.WriteString("\n")
	if err := os.WriteFile(filePath, []byte(b.String()), 0644); err != nil {
		return "写入记忆失败: " + err.Error()
	}
	return ""
}

// CompactConversation 调用 AI 把会话历史压缩成摘要，持久化到 .eg/memory/summary.md。
// messages 为待压缩的对话历史（通常是超过阈值后被截断的旧消息）。
// 通过事件 ide:ai-memory-compacted 回传结果：{ success, summary, error }。
// 压缩失败时不修改已有记忆文件，仅回传错误。
func (s *IDEService) CompactConversation(endpoint, apiKey, model string, messages []AIMessage, projectPath string) {
	go func() {
		if projectPath == "" || len(messages) == 0 {
			s.app.Event.Emit("ide:ai-memory-compacted", map[string]any{
				"success": false,
				"error":   "项目路径或消息为空",
			})
			return
		}
		client := ai.NewClient(endpoint, apiKey, model)

		// 拼接待压缩的对话为纯文本
		var b strings.Builder
		b.WriteString("以下是 EGOU 项目中一段 AI 对话历史，请压缩成不超过 300 字的摘要，保留：\n")
		b.WriteString("1. 用户的核心需求\n2. 已做出的关键决策\n3. 未解决的问题\n4. 涉及的主要文件/模块\n\n")
		for _, m := range messages {
			role := "用户"
			if m.Role == "assistant" {
				role = "助手"
			} else if m.Role == "system" {
				role = "系统"
			}
			// 单条消息截断到 800 字避免过长
			content := m.Content
			if len([]rune(content)) > 800 {
				content = string([]rune(content)[:800]) + "...(截断)"
			}
			b.WriteString("【" + role + "】" + content + "\n\n")
		}

		compactMsgs := []ai.Message{
			{Role: "system", Content: "你是 EGOU 中文编程语言的 AI 助手，擅长把长对话压缩成结构化摘要。"},
			{Role: "user", Content: b.String()},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		summary, err := client.Chat(ctx, compactMsgs, 0.3, 800)
		if err != nil {
			if s.app != nil {
				s.app.Event.Emit("ide:ai-memory-compacted", map[string]any{
					"success": false,
					"error":   err.Error(),
				})
			}
			return
		}
		// 追加到 summary.md（不覆盖已有摘要）
		errMsg := s.AppendProjectMemorySection(projectPath, "summary", summary)
		if errMsg != "" {
			if s.app != nil {
				s.app.Event.Emit("ide:ai-memory-compacted", map[string]any{
					"success": false,
					"error":   errMsg,
				})
			}
			return
		}
		if s.app != nil {
			s.app.Event.Emit("ide:ai-memory-compacted", map[string]any{
				"success": true,
				"summary": summary,
			})
		}
	}()
}

// extractDecisions 从 AI 输出中启发式提取关键决策（含"决定/采用/选择/使用"等关键词的行）。
// 返回每行一条决策，最多 5 条。无匹配返回空 slice。
func extractDecisions(output string) []string {
	keywords := []string{"决定", "采用", "选择", "使用", "确定", "敲定", "敲定为", "最终"}
	var decisions []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 4 || len(line) > 200 {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				// 去掉 markdown 前缀
				line = strings.TrimPrefix(line, "- ")
				line = strings.TrimPrefix(line, "* ")
				line = strings.TrimPrefix(line, "1. ")
				decisions = append(decisions, line)
				if len(decisions) >= 5 {
					return decisions
				}
				break
			}
		}
	}
	return decisions
}
