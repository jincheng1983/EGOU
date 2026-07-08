// Package ai — Agent 编排（P2-9，吸取 NxEGO3）
//
// 设计目标：
//   - 定义 5 种 Agent 角色（Planner/Coder/Reviewer/UIBuilder/Fixer）
//   - 每个 Agent 配置独立 SystemPrompt + 可用 Tools 列表 + MaxIterations 上限
//   - 支持单 Agent 调用（RunAgent）和多 Agent 串行编排（RunPipeline）
//   - 编排过程通过 EventSink 回调状态，便于前端展示进度
//
// 使用示例：
//   orch := ai.NewOrchestrator(client)
//   result, err := orch.RunAgent(ctx, ai.RoleCoder, "写一个排序函数", nil)
//   pipeline := []ai.AgentRole{ai.RolePlanner, ai.RoleCoder, ai.RoleReviewer}
//   final, err := orch.RunPipeline(ctx, pipeline, "实现用户登录功能", nil)
package ai

import (
	"context"
	"fmt"
	"strings"
)

// AgentRole 标识 Agent 角色
type AgentRole string

const (
	RolePlanner   AgentRole = "planner"   // 架构规划：分析需求、设计模块、拆分任务
	RoleCoder     AgentRole = "coder"     // 代码生成：根据规划编写 EGOU 代码
	RoleReviewer  AgentRole = "reviewer"  // 代码审查：检查代码质量、风格、潜在问题
	RoleUIBuilder AgentRole = "ui_builder" // UI 设计：窗口布局、控件配置、事件绑定
	RoleFixer     AgentRole = "fixer"     // 错误修复：根据编译错误诊断并修复
)

// AgentConfig 描述一个 Agent 角色的配置
type AgentConfig struct {
	Role          AgentRole
	Name          string   // 中文名称
	Description   string   // 角色描述
	SystemPrompt  string   // 独立系统提示词
	Tools         []string // 可用工具列表（如 read_file/write_file/list_dir）
	MaxIterations int      // 单 Agent 最大迭代次数（防止无限循环）
}

// ToolCall 表示一次工具调用请求
type ToolCall struct {
	Tool    string            `json:"tool"`
	Params  map[string]string `json:"params"`
}

// AgentResult 是 Agent 一次执行的输出
type AgentResult struct {
	Role      AgentRole
	Output    string     // 最终输出文本（含代码块）
	ToolCalls []ToolCall // 触发的工具调用（供前端确认/执行）
	Iterations int       // 实际迭代次数
	Success   bool
	Error     string
}

// PipelineEvent 是编排过程中的状态事件
type PipelineEvent struct {
	Stage     string      // "agent-start" / "agent-end" / "pipeline-done"
	Role      AgentRole
	Index     int         // 在 pipeline 中的位置
	Total     int         // pipeline 总长度
	Result    *AgentResult
}

// EventSinkFn 接收编排事件；为 nil 时静默
type EventSinkFn func(PipelineEvent)

// agentConfigs 是内置角色配置（SystemPrompt 简化，与前端 aiAgents.js 互补）
var agentConfigs = map[AgentRole]AgentConfig{
	RolePlanner: {
		Role:        RolePlanner,
		Name:        "架构规划师",
		Description: "分析需求、设计模块结构、拆分实现任务",
		SystemPrompt: `你是 EGOU 项目的架构规划师。
任务：
1. 分析用户需求，明确功能边界
2. 设计模块/类划分（参考 EGOU 项目结构：源码/模块/类/窗口）
3. 拆分实现步骤，给出任务清单
4. 指出潜在风险和注意事项
输出格式：先给架构方案，再给任务清单（带优先级）。`,
		Tools:         []string{"list_files", "read_file"},
		MaxIterations: 3,
	},
	RoleCoder: {
		Role:        RoleCoder,
		Name:        "代码工程师",
		Description: "根据规划编写 EGOU 代码",
		SystemPrompt: "你是 EGOU 代码工程师。\n任务：\n" +
			"1. 根据规划任务编写具体的 EGOU 代码\n" +
			"2. 严格遵循 EGOU 语法规范（// 注释、英文符号、4 空格缩进）\n" +
			"3. 使用内置支持库命令（信息框、调试输出、到文本等）\n" +
			"4. 给出可直接运行的完整代码块\n" +
			"输出格式：先简述实现思路，再给 ```egou 代码块。",
		Tools:         []string{"read_file", "write_file"},
		MaxIterations: 5,
	},
	RoleReviewer: {
		Role:        RoleReviewer,
		Name:        "代码审查员",
		Description: "检查代码质量、风格、潜在问题",
		SystemPrompt: `你是 EGOU 代码审查员。
任务：
1. 检查代码是否符合 EGOU 语法规范
2. 发现潜在 bug（类型不匹配、括号不匹配、变量未定义等）
3. 评估代码可读性和可维护性
4. 给出改进建议（如果有）
输出格式：先给审查结论（通过/需修改），再列具体问题。`,
		Tools:         []string{"read_file"},
		MaxIterations: 2,
	},
	RoleUIBuilder: {
		Role:        RoleUIBuilder,
		Name:        "UI 设计师",
		Description: "窗口布局、控件配置、事件绑定",
		SystemPrompt: `你是 EGOU UI 设计师。
任务：
1. 设计窗口布局（控件清单：名称/类型/位置/大小）
2. 给出 .ew 窗口设计数据建议
3. 给出 .eg 事件处理代码框架
4. 遵循 8px 网格规范，控件对齐整齐
输出格式：先给控件清单表格，再给事件代码。`,
		Tools:         []string{"read_file", "write_file"},
		MaxIterations: 4,
	},
	RoleFixer: {
		Role:        RoleFixer,
		Name:        "错误修复师",
		Description: "根据编译错误诊断并修复",
		SystemPrompt: `你是 EGOU 错误修复师。
任务：
1. 分析编译错误（格式：文件:行:列: 信息）
2. 定位错误根因
3. 给出修复后的代码片段
4. 翻译英文错误为中文解释
输出格式：先给错误原因，再给修复代码，最后给预防建议。`,
		Tools:         []string{"read_file", "write_file", "run_build"},
		MaxIterations: 5,
	},
}

// GetAgentConfig 返回角色的内置配置
func GetAgentConfig(role AgentRole) (AgentConfig, bool) {
	cfg, ok := agentConfigs[role]
	return cfg, ok
}

// AllAgentConfigs 返回所有内置角色配置
func AllAgentConfigs() []AgentConfig {
	roles := []AgentRole{RolePlanner, RoleCoder, RoleReviewer, RoleUIBuilder, RoleFixer}
	out := make([]AgentConfig, 0, len(roles))
	for _, r := range roles {
		if cfg, ok := agentConfigs[r]; ok {
			out = append(out, cfg)
		}
	}
	return out
}

// Orchestrator 编排器，依赖 Client 实际调用模型
type Orchestrator struct {
	client         *Client
	sink           EventSinkFn
	projectContext string // 项目记忆（结构化 markdown），注入到每个 Agent 的系统提示词
}

// NewOrchestrator 创建编排器
func NewOrchestrator(client *Client) *Orchestrator {
	return &Orchestrator{client: client}
}

// SetProjectContext 设置项目记忆上下文（来自 .eg/memory/ 的 summary + decisions + 用户备注）。
// 设置后，RunAgent / RunPipeline 会在每个 Agent 的系统提示词后追加此上下文，
// 让 Fixer / Reviewer 等 Agent 也能感知项目背景。
func (o *Orchestrator) SetProjectContext(ctx string) {
	o.projectContext = ctx
}

// SetSink 设置事件回调（可选）
func (o *Orchestrator) SetSink(sink EventSinkFn) {
	o.sink = sink
}

// emit 触发事件
func (o *Orchestrator) emit(stage string, role AgentRole, idx, total int, result *AgentResult) {
	if o.sink == nil {
		return
	}
	o.sink(PipelineEvent{
		Stage:  stage,
		Role:   role,
		Index:  idx,
		Total:  total,
		Result: result,
	})
}

// RunAgent 执行单个 Agent 调用
func (o *Orchestrator) RunAgent(ctx context.Context, role AgentRole, userInput string, history []Message) (AgentResult, error) {
	cfg, ok := agentConfigs[role]
	if !ok {
		return AgentResult{Role: role, Error: "未知角色"}, fmt.Errorf("未知 Agent 角色: %s", role)
	}

	sysPrompt := cfg.SystemPrompt
	if o.projectContext != "" {
		sysPrompt = sysPrompt + "\n\n【项目记忆】\n" + o.projectContext
	}
	msgs := []Message{{Role: "system", Content: sysPrompt}}
	msgs = append(msgs, history...)
	msgs = append(msgs, Message{Role: "user", Content: userInput})

	o.emit("agent-start", role, 0, 1, nil)

	output, err := o.client.Chat(ctx, msgs, 0.7, 4096)
	result := AgentResult{
		Role:       role,
		Output:     output,
		Iterations: 1,
		Success:    err == nil,
	}
	if err != nil {
		result.Error = err.Error()
		o.emit("agent-end", role, 0, 1, &result)
		return result, err
	}

	// 解析工具调用（简单格式：[TOOL:tool_name params=key:val,key:val]）
	result.ToolCalls = parseToolCalls(output)

	o.emit("agent-end", role, 0, 1, &result)
	return result, nil
}

// RunPipeline 串行执行多个 Agent，前一个的输出作为后一个的输入
func (o *Orchestrator) RunPipeline(ctx context.Context, pipeline []AgentRole, userInput string, history []Message) (AgentResult, error) {
	total := len(pipeline)
	if total == 0 {
		return AgentResult{}, fmt.Errorf("空 pipeline")
	}

	currentInput := userInput
	currentHistory := history
	var lastResult AgentResult

	for i, role := range pipeline {
		cfg, ok := agentConfigs[role]
		if !ok {
			return lastResult, fmt.Errorf("pipeline 第 %d 步未知角色: %s", i, role)
		}

		// 前一个 Agent 的输出作为当前 Agent 的输入
		if i > 0 {
			currentInput = fmt.Sprintf("【上一步：%s 的输出】\n%s\n\n请基于上述内容继续。",
				lastResult.Role, truncate(lastResult.Output, 4000))
		}

		o.emit("agent-start", role, i, total, nil)

		sysPrompt := cfg.SystemPrompt
		if o.projectContext != "" {
			sysPrompt = sysPrompt + "\n\n【项目记忆】\n" + o.projectContext
		}
		msgs := []Message{{Role: "system", Content: sysPrompt}}
		msgs = append(msgs, currentHistory...)
		msgs = append(msgs, Message{Role: "user", Content: currentInput})

		output, err := o.client.Chat(ctx, msgs, 0.7, 4096)
		lastResult = AgentResult{
			Role:       role,
			Output:     output,
			Iterations: 1,
			Success:    err == nil,
		}
		if err != nil {
			lastResult.Error = err.Error()
			o.emit("agent-end", role, i, total, &lastResult)
			return lastResult, err
		}

		lastResult.ToolCalls = parseToolCalls(output)
		o.emit("agent-end", role, i, total, &lastResult)

		// 把当前输出加入历史，供下一个 Agent 参考
		currentHistory = append(currentHistory,
			Message{Role: "user", Content: currentInput},
			Message{Role: "assistant", Content: output},
		)
	}

	o.emit("pipeline-done", lastResult.Role, total-1, total, &lastResult)
	return lastResult, nil
}

// parseToolCalls 从输出中解析简单的工具调用标记
// 格式：[TOOL:tool_name params=key:val,key:val]
func parseToolCalls(output string) []ToolCall {
	var calls []ToolCall
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[TOOL:") {
			continue
		}
		line = strings.TrimPrefix(line, "[TOOL:")
		end := strings.Index(line, "]")
		if end < 0 {
			continue
		}
		body := strings.TrimSpace(line[:end])
		parts := strings.SplitN(body, " ", 2)
		tool := parts[0]
		call := ToolCall{Tool: tool, Params: map[string]string{}}
		if len(parts) > 1 {
			for _, kv := range strings.Split(parts[1], ",") {
				kvParts := strings.SplitN(kv, ":", 2)
				if len(kvParts) == 2 {
					call.Params[strings.TrimSpace(kvParts[0])] = strings.TrimSpace(kvParts[1])
				}
			}
		}
		calls = append(calls, call)
	}
	return calls
}

// truncate 截断字符串到指定长度
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...(内容过长，已截断)"
}
