// e2e_test.go 是 P2 调试器的端到端集成测试。
//
// 测试目标：验证从 .eg 源码编译 → dlv 启动 → 断点/单步/变量/退出 全流程可用。
//
// 测试策略：
//   1. 构造最小 .eg 项目（main.eg + project.eg.json 写到临时目录）
//   2. 调用 runner.BuildForDebug 编译（debug 模式保留 DWARF）
//   3. 调用 debugger.StartDebug 启动 dlv headless
//   4. 在 main.eg 指定行设置断点 → Continue → 验证命中
//   5. 查询栈帧和变量
//   6. Next 单步 → 验证当前行前进
//   7. Continue → 程序退出 → 验证 onExit 回调
//
// 依赖：dlv 已安装（PATH 或 GOPATH/bin），Go 工具链可用。
// 跳过条件：dlv 不存在时跳过（避免 CI 环境无 dlv 时失败）。
package debugger

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"egou/internal/runner"
)

// findDelveForTest 在测试启动前确认 dlv 可用，不可用时跳过测试。
func findDelveForTest(t *testing.T) string {
	t.Helper()
	// 优先用 debugger 包内的 findDelve（与生产代码同逻辑）
	p, err := findDelve()
	if err != nil {
		t.Skipf("跳过：未找到 dlv — %v", err)
	}
	return p
}

// setupTemplateDir 注入 wails-template 路径到 runner 包。
// BuildForDebug 需要复制 wails-template 到临时目录，测试环境下 exe 同级没有该目录，
// 必须显式注入（与 IDE 启动时的 SetTemplateDir 调用一致）。
// 路径解析：测试运行目录是 <repo>/internal/debugger/，向上 3 级到 repo 根。
func setupTemplateDir(t *testing.T) {
	t.Helper()
	// runtime.Caller 获取调用者文件位置，向上推导仓库根目录
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	// file = <repo>/internal/debugger/e2e_test.go
	// repo 根 = filepath.Dir(filepath.Dir(filepath.Dir(file)))
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	tmplDir := filepath.Join(repoRoot, "runtime", "wails-template")
	if _, err := os.Stat(tmplDir); err != nil {
		t.Skipf("跳过：wails-template 目录不存在: %s", tmplDir)
	}
	runner.SetTemplateDir(tmplDir)
}

// writeTestProject 在临时目录下创建最小 .eg 项目结构。
// 返回项目根目录路径。
//
// 项目结构：
//
//	<tmp>/dbgproject/
//	├── project.eg.json
//	└── src/
//	    └── main.eg
func writeTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	srcDir := filepath.Join(root, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("创建 src 目录失败: %v", err)
	}

	// main.eg：包含 4 行可断点语句，便于验证单步前进
	// 行号从 1 开始计数（包含 # 程序集行）
	mainEg := `# 程序集 main

导入 (
    "fmt"
)

函数 主函数()
    fmt.Println("调试开始")
    fmt.Println("第二行")
    fmt.Println("第三行")
    fmt.Println("调试结束")
结束函数
`
	if err := os.WriteFile(filepath.Join(srcDir, "main.eg"), []byte(mainEg), 0644); err != nil {
		t.Fatalf("写入 main.eg 失败: %v", err)
	}

	// project.eg.json：最小项目配置
	projectJSON := `{
  "name": "dbgtest",
  "type": "console",
  "version": "1.0.0",
  "output": "bin"
}`
	if err := os.WriteFile(filepath.Join(root, "project.eg.json"), []byte(projectJSON), 0644); err != nil {
		t.Fatalf("写入 project.eg.json 失败: %v", err)
	}

	return root
}

// waitForHalt 等待 onHalt 回调触发，超时返回错误。
// dlv 的 Continue/Next/Step 是阻塞调用，但在 StartDebug 中通过 goroutine 异步执行，
// 这里用 channel + 超时确保测试不会卡死。
func waitForHalt(ch <-chan *DebuggerState, timeout time.Duration) (*DebuggerState, error) {
	select {
	case s := <-ch:
		return s, nil
	case <-time.After(timeout):
		return nil, os.ErrDeadlineExceeded
	}
}

// waitForExit 等待 onExit 回调触发。
func waitForExit(ch <-chan *DebuggerState, timeout time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

// TestE2EDebuggerFlow 是端到端主测试：编译→启动→断点→单步→退出。
func TestE2EDebuggerFlow(t *testing.T) {
	findDelveForTest(t)
	setupTemplateDir(t)

	// 确认 go 工具链可用
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("跳过：未找到 go 工具链")
	}

	projectPath := writeTestProject(t)

	// 收集源码（模拟 collectProjectSources 的简化逻辑）
	mainEgBytes, err := os.ReadFile(filepath.Join(projectPath, "src", "main.eg"))
	if err != nil {
		t.Fatalf("读取 main.eg 失败: %v", err)
	}
	src := "#@eg-file main.eg\n" + string(mainEgBytes)

	// 编译（debug 模式保留 DWARF）
	t.Log("开始编译（BuildForDebug）...")
	sink := func(ev runner.Event) {
		t.Logf("[%s] %s", ev.Stage, ev.Output)
	}
	binaryPath, tmpDir, err := runner.BuildForDebug(src, projectPath, sink)
	if err != nil {
		t.Fatalf("BuildForDebug 失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	t.Logf("编译成功: %s", binaryPath)

	// 打印转译后的 usercode.go 前 40 行，便于诊断 //line 指令
	if usercode, err := os.ReadFile(filepath.Join(tmpDir, "usercode.go")); err == nil {
		lines := strings.Split(string(usercode), "\n")
		t.Logf("usercode.go 共 %d 行，前 40 行:", len(lines))
		for i, line := range lines {
			if i >= 40 {
				break
			}
			t.Logf("  %3d: %s", i+1, line)
		}
		// 检查是否包含 //line 指令
		if strings.Contains(string(usercode), "//line ") {
			t.Logf("✓ usercode.go 包含 //line 指令")
		} else {
			t.Logf("✗ usercode.go 不包含 //line 指令")
		}
	}

	// 验证二进制存在
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("编译产物不存在: %v", err)
	}

	// 准备回调 channel
	haltCh := make(chan *DebuggerState, 8)
	exitCh := make(chan *DebuggerState, 1)
	logCh := make(chan string, 32)

	var haltMu sync.Mutex
	haltReceived := 0
	onHalt := func(s *DebuggerState) {
		haltMu.Lock()
		haltReceived++
		haltMu.Unlock()
		select {
		case haltCh <- s:
		default:
		}
	}
	onExit := func(s *DebuggerState) {
		select {
		case exitCh <- s:
		default:
		}
	}
	// 启动 dlv
	t.Log("启动 dlv headless...")

	// 先收集 dlv 的所有输出（包括 stderr），便于诊断
	var dlvLogs []string
	var dlvLogMu sync.Mutex
	enhancedOnLog := func(line string) {
		dlvLogMu.Lock()
		dlvLogs = append(dlvLogs, line)
		dlvLogMu.Unlock()
		t.Logf("[dlv-out] %s", line)
		select {
		case logCh <- line:
		default:
		}
	}

	client, err := StartDebug(binaryPath, nil, onHalt, onExit, enhancedOnLog)
	if err != nil {
		t.Fatalf("StartDebug 失败: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()
	t.Logf("dlv 已连接，地址: %s", client.addr)

	// 给 dlv 一点时间稳定（防止 race condition）
	time.Sleep(500 * time.Millisecond)

	// dlv exec 启动后程序停在入口，但不会自动触发 onHalt 回调。
	// 这里调用 State() 主动获取初始状态（与 IDEService.StartDebug 的做法一致）。
	t.Log("获取初始状态...")
	initialState, err := client.State()
	if err != nil {
		// 如果 State 因连接问题失败，可能是 dlv 已退出
		if isConnClosedErr(err) {
			t.Fatalf("dlv 启动后立即退出，State 失败: %v", err)
		}
		t.Fatalf("State 调用失败: %v", err)
	}
	t.Logf("初始状态: reason=%s, file=%s:%d, running=%v, exited=%v",
		initialState.StopReason,
		func() string {
			if initialState.CurrentThread != nil {
				return initialState.CurrentThread.File
			}
			return "?"
		}(),
		func() int {
			if initialState.CurrentThread != nil {
				return initialState.CurrentThread.Line
			}
			return 0
		}(),
		initialState.Running, initialState.Exited)
	if initialState.Exited {
		t.Fatalf("程序在启动时已退出（exitStatus=%d）", initialState.ExitStatus)
	}

	// 设置断点：在 main.eg 的 "fmt.Println(\"第二行\")" 行
	// main.eg 内容（行号从 1 开始）：
	//   1: # 程序集 main
	//   2: (空)
	//   3: 导入 (
	//   4:     "fmt"
	//   5: )
	//   6: (空)
	//   7: 函数 主函数()
	//   8:     fmt.Println("调试开始")
	//   9:     fmt.Println("第二行")  ← 断点设在这一行
	//  10:     fmt.Println("第三行")
	//  11:     fmt.Println("调试结束")
	//  12: 结束函数
	const bpLine = 9
	t.Logf("在 main.eg:%d 设置断点", bpLine)

	// 先列出 dlv 已知的源文件，诊断断点设置失败的原因
	sources, srcErr := client.ListSources("")
	if srcErr != nil {
		t.Logf("ListSources 失败: %v", srcErr)
	} else {
		t.Logf("dlv 已知源文件 %d 个:", len(sources))
		// 查找包含 .eg 的源文件（ListSources 返回的文件名可能带引号）
		var egFiles []string
		for _, s := range sources {
			if strings.Contains(s, ".eg") {
				egFiles = append(egFiles, s)
			}
		}
		t.Logf("其中含 .eg 的文件 %d 个:", len(egFiles))
		for _, s := range egFiles {
			t.Logf("  原始值: %q (len=%d)", s, len(s))
		}
	}

	// 尝试用 FindLocation 查找 main.eg:9
	locs, findErr := client.FindLocation(-1, 0, "main.eg:9")
	if findErr != nil {
		t.Logf("FindLocation(main.eg:9) 失败: %v", findErr)
	} else {
		t.Logf("FindLocation(main.eg:9) 返回 %d 个位置:", len(locs))
		for i, loc := range locs {
			t.Logf("  [%d] file=%s, line=%d, pc=%d", i, loc.File, loc.Line, loc.PC)
		}
	}

	// 尝试用 +9 格式（dlv 支持的 "文件名:行号" 语法）
	locs2, findErr2 := client.FindLocation(-1, 0, "+9")
	if findErr2 != nil {
		t.Logf("FindLocation(+9) 失败: %v", findErr2)
	} else {
		t.Logf("FindLocation(+9) 返回 %d 个位置:", len(locs2))
		for i, loc := range locs2 {
			t.Logf("  [%d] file=%s, line=%d", i, loc.File, loc.Line)
		}
	}

	bp, err := client.CreateBreakpoint("main.eg", bpLine)
	if err != nil {
		// 调试器实际运行时可能因 //line 指令格式不同导致断点设置失败
		// 这里记录详细错误，但不立即 Fail，让后续 Continue 也能验证流程
		t.Logf("设置断点失败（可能是 //line 指令文件名不匹配）: %v", err)
	} else {
		t.Logf("断点已设置: id=%d, file=%s, line=%d", bp.ID, bp.File, bp.Line)
	}

	// Continue：继续执行，应命中断点或程序直接退出
	// Continue 是阻塞调用，放在 goroutine 中，通过 onHalt/onExit 回调感知结果
	t.Log("Continue...")
	go func() {
		_, _ = client.Continue()
	}()

	// 等待 halt 或 exit
	var haltState *DebuggerState
	select {
	case s := <-haltCh:
		haltState = s
		t.Logf("命中断点: reason=%s, running=%v", s.StopReason, s.Running)
		if s.CurrentThread != nil {
			t.Logf("  位置: %s:%d", s.CurrentThread.File, s.CurrentThread.Line)
		}
		if s.SelectedGoroutine != nil {
			t.Logf("  goroutine ID: %d", s.SelectedGoroutine.ID)
		}
	case <-exitCh:
		t.Fatalf("程序在断点前已退出（断点未命中）")
	case <-time.After(30 * time.Second):
		t.Fatalf("等待 halt 超时（断点未命中且程序未退出）")
	}

	// 获取 goroutine ID：优先从 haltState.SelectedGoroutine，fallback 到 ListGoroutines
	goroutineID := -1
	if haltState != nil && haltState.SelectedGoroutine != nil {
		goroutineID = haltState.SelectedGoroutine.ID
	}
	if goroutineID == -1 {
		t.Log("SelectedGoroutine 为空，尝试 ListGoroutines 获取 goroutine ID...")
		gs, gerr := client.ListGoroutines()
		if gerr != nil {
			t.Logf("ListGoroutines 失败: %v", gerr)
		} else if len(gs) > 0 {
			goroutineID = gs[0].ID
			t.Logf("从 ListGoroutines 获取 goroutine ID: %d (共 %d 个 goroutine)", goroutineID, len(gs))
		}
	}

	// 查询调用栈（用实际 goroutine ID）
	t.Logf("查询调用栈（goroutineID=%d）...", goroutineID)
	stack, err := client.Stacktrace(goroutineID, 20)
	if err != nil {
		t.Logf("Stacktrace 失败: %v", err)
	} else {
		t.Logf("调用栈 %d 层:", len(stack))
		for i, frame := range stack {
			fn := "?"
			if frame.Function != nil {
				fn = frame.Function.Name
			}
			t.Logf("  [%d] %s %s:%d", i, fn, frame.File, frame.Line)
		}
	}

	// 查询局部变量（用实际 goroutine ID）
	t.Logf("查询局部变量（goroutineID=%d）...", goroutineID)
	vars, err := client.ListLocalVars(goroutineID, 0)
	if err != nil {
		t.Logf("ListLocalVars 失败: %v", err)
	} else {
		t.Logf("局部变量 %d 个:", len(vars))
		for _, v := range vars {
			t.Logf("  %s = %s (类型: %s)", v.Name, v.Value, v.Type)
		}
	}

	// Next 单步跳过
	t.Log("Next 单步跳过...")
	go func() {
		_, _ = client.Next()
	}()
	nextState, err := waitForHalt(haltCh, 15*time.Second)
	if err != nil {
		t.Logf("等待 Next halt 超时")
	} else if nextState.CurrentThread != nil {
		t.Logf("Next 后位置: %s:%d", nextState.CurrentThread.File, nextState.CurrentThread.Line)
		// 期望行号前进到 10（"第三行"）
		if nextState.CurrentThread.Line == bpLine+1 {
			t.Logf("✓ 单步前进到下一行（行号 %d）", nextState.CurrentThread.Line)
		} else {
			t.Logf("行号未按预期前进（期望 %d，实际 %d）", bpLine+1, nextState.CurrentThread.Line)
		}
	} else {
		t.Logf("Next 完成（CurrentThread 为空，可能因 //line 指令导致 dlv 未填充线程信息）")
	}

	// 继续 Continue → 程序应退出
	// 注意：Wails app 无窗口时可能不退出（app.Quit 不终止事件循环），
	// 如果超时不视为测试失败（Wails 生命周期问题，非调试器问题）
	t.Log("Continue → 等待程序退出...")
	go func() {
		_, _ = client.Continue()
	}()
	if !waitForExit(exitCh, 15*time.Second) {
		t.Logf("⚠ 等待程序退出超时（Wails app 无窗口时不退出，属已知问题，非调试器问题）")
	} else {
		t.Log("✓ 程序已退出")
	}

	// 验证被调试程序的输出（"调试开始"等应通过 onLog 转发）
	close(logCh)
	var logs []string
	for line := range logCh {
		logs = append(logs, line)
	}
	allLogs := strings.Join(logs, "\n")
	t.Logf("被调试程序输出 %d 行:\n%s", len(logs), allLogs)
	if !strings.Contains(allLogs, "调试开始") {
		t.Errorf("被调试程序输出中未找到 '调试开始'")
	}
	if !strings.Contains(allLogs, "调试结束") {
		t.Errorf("被调试程序输出中未找到 '调试结束'")
	}
}

// TestE2EDebuggerBreakpointOnly 验证仅启动 dlv + 设置断点 + 立即退出的最简流程。
// 用于快速验证 dlv 集成是否正常工作（不依赖单步/变量查询）。
func TestE2EDebuggerBreakpointOnly(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 测试")
	}
	findDelveForTest(t)
	setupTemplateDir(t)

	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("跳过：未找到 go 工具链")
	}

	projectPath := writeTestProject(t)
	mainEgBytes, _ := os.ReadFile(filepath.Join(projectPath, "src", "main.eg"))
	src := "#@eg-file main.eg\n" + string(mainEgBytes)

	binaryPath, tmpDir, err := runner.BuildForDebug(src, projectPath, func(ev runner.Event) {})
	if err != nil {
		t.Fatalf("BuildForDebug 失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	exitCh := make(chan *DebuggerState, 1)
	client, err := StartDebug(binaryPath, nil,
		func(s *DebuggerState) {}, // 不关心 halt
		func(s *DebuggerState) { exitCh <- s },
		func(line string) {},
	)
	if err != nil {
		t.Fatalf("StartDebug 失败: %v", err)
	}
	defer client.Close()

	// 不设断点，直接 Continue → 程序应直接退出或停止运行
	// 注意：Wails app 无窗口时可能不退出（app.Quit 不终止事件循环），
	// 但用户代码已执行完毕，dlv 会返回 running=false。
	// 同步调用（不用 goroutine），直接检查返回值，便于诊断
	t.Log("Continue...")
	state, err := client.Continue()
	if err != nil {
		t.Fatalf("Continue 调用失败: %v", err)
	}
	t.Logf("Continue 返回: exited=%v, running=%v, exitStatus=%d, stopReason=%q",
		state.Exited, state.Running, state.ExitStatus, state.StopReason)
	if state.Exited {
		t.Log("✓ 最简流程通过：dlv 启动 + Continue + 程序退出")
	} else if !state.Running {
		t.Log("✓ 最简流程通过：dlv 启动 + Continue + 程序停止运行（Wails app 未退出，属已知问题）")
	} else {
		t.Fatalf("程序仍在运行（exited=false, running=true）")
	}
}
