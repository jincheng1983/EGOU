// Package ai — BuildAndFix 自动修复（P2-12，吸取 NxEGO3）
//
// 设计目标：
//   - 编译失败后自动调用 Fixer Agent 诊断并修复
//   - 最多 MaxRounds 轮重试，每轮重编
//   - 修复期间通过 FixEvent 回调状态（前端可展示进度）
//   - 修复完成后返回最终源码和编译结果
//
// 使用示例（在 IDEService 中）：
//   finalSrc, result, err := ai.BuildAndFix(src, projectPath, client, sink, 3)
//   if err == nil { 写回 finalSrc 到文件 }
package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// MaxRounds 默认最大修复轮数
const DefaultMaxRounds = 3

// FixEvent 是修复过程中的状态事件
type FixEvent struct {
	Stage    string // "build-start" / "build-failed" / "fix-start" / "fix-applied" / "build-success" / "max-rounds-exceeded"
	Round    int    // 当前轮次（1-based）
	Errors   []string // 当前编译错误
	FixOutput string // Fixer Agent 的输出
	Source   string // 当前源码（仅 fix-applied 时有意义）
}

// FixSink 接收修复事件
type FixSink func(FixEvent)

// BuildFn 是编译函数的抽象（避免循环依赖 runner 包）
// 返回：编译输出 + 错误（错误中包含失败信息）
type BuildFn func(source string, projectPath string) (output string, err error)

// parseCompileErrors 从编译输出中解析错误行
// 格式：file:line:col: message
var compileErrorRe = regexp.MustCompile(`(?m)^(?:\./)?([^\s:]+\.go):(\d+):(\d+):\s+(.+)$`)

func parseCompileErrors(output string) []string {
	var errors []string
	matches := compileErrorRe.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		if len(m) >= 5 {
			errors = append(errors, fmt.Sprintf("%s:%s:%s: %s", m[1], m[2], m[3], m[4]))
		}
	}
	return errors
}

// extractCodeBlock 从 AI 输出中提取 ```egou 代码块
func extractCodeBlock(output string) string {
	// 优先匹配 ```egou 标记的代码块
	re := regexp.MustCompile("(?s)```(?:egou|go)?\\s*\n(.*?)\n```")
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return ""
	}
	// 返回最后一个代码块（通常是修复后的完整代码）
	return strings.TrimSpace(matches[len(matches)-1][1])
}

// BuildAndFix 执行编译-修复循环
//
// 参数：
//   - initialSrc: 初始源码
//   - projectPath: 项目路径
//   - buildFn: 编译函数（实际调用 runner.BuildSource）
//   - client: AI Client（用于调用 Fixer Agent）
//   - maxRounds: 最大修复轮数（0 表示用默认值 DefaultMaxRounds）
//   - sink: 状态回调（可为 nil）
//
// 返回：
//   - finalSrc: 最终源码（可能被修复过）
//   - buildOutput: 最终编译输出
//   - success: 是否最终编译成功
//   - err: 不可恢复的错误
func BuildAndFix(initialSrc, projectPath string, buildFn BuildFn, client *Client, maxRounds int, sink FixSink) (finalSrc, buildOutput string, success bool, err error) {
	if maxRounds <= 0 {
		maxRounds = DefaultMaxRounds
	}

	currentSrc := initialSrc
	var lastErrors []string

	emit := func(stage string, round int, errs []string, fixOut, src string) {
		if sink == nil {
			return
		}
		sink(FixEvent{
			Stage:     stage,
			Round:     round,
			Errors:    errs,
			FixOutput: fixOut,
			Source:    src,
		})
	}

	for round := 1; round <= maxRounds; round++ {
		// 1. 编译
		emit("build-start", round, nil, "", currentSrc)
		output, buildErr := buildFn(currentSrc, projectPath)
		buildOutput = output

		if buildErr == nil {
			// 编译成功
			emit("build-success", round, nil, "", currentSrc)
			return currentSrc, buildOutput, true, nil
		}

		// 2. 解析错误
		lastErrors = parseCompileErrors(output)
		if len(lastErrors) == 0 {
			// 解析不到结构化错误，可能是其他错误，直接返回
			emit("build-failed", round, []string{buildErr.Error()}, "", currentSrc)
			return currentSrc, buildOutput, false, buildErr
		}

		emit("build-failed", round, lastErrors, "", currentSrc)

		// 3. 调用 Fixer Agent
		if client == nil {
			// 没有 AI Client，无法修复
			return currentSrc, buildOutput, false, fmt.Errorf("编译失败且未配置 AI Client: %w", buildErr)
		}

		emit("fix-start", round, lastErrors, "", currentSrc)

		fixerCfg, _ := GetAgentConfig(RoleFixer)
		errMsg := fmt.Sprintf("编译失败，错误如下：\n%s\n\n当前源码：\n```egou\n%s\n```\n\n请分析错误原因，给出修复后的完整代码。",
			strings.Join(lastErrors, "\n"), currentSrc)

		msgs := []Message{
			{Role: "system", Content: fixerCfg.SystemPrompt},
			{Role: "user", Content: errMsg},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		fixOutput, aiErr := client.Chat(ctx, msgs, 0.5, 4096)
		cancel()

		if aiErr != nil {
			return currentSrc, buildOutput, false, fmt.Errorf("Fixer Agent 调用失败: %w", aiErr)
		}

		// 4. 提取修复后的代码
		newSrc := extractCodeBlock(fixOutput)
		if newSrc == "" {
			// AI 没有给出代码块，可能是只给了修改建议
			emit("fix-applied", round, lastErrors, fixOutput, currentSrc)
			return currentSrc, buildOutput, false, fmt.Errorf("Fixer Agent 未返回可识别的代码块")
		}

		// 5. 应用修复，进入下一轮
		currentSrc = newSrc
		emit("fix-applied", round, lastErrors, fixOutput, currentSrc)
	}

	// 达到最大轮数仍未成功
	emit("max-rounds-exceeded", maxRounds, lastErrors, "", currentSrc)
	return currentSrc, buildOutput, false, fmt.Errorf("达到最大修复轮数 %d 仍未成功", maxRounds)
}
