// debug.go 实现 IDEService 的调试方法（P2 Delve 集成）。
//
// 调试流程：
//   1. StartDebug: 编译（debug 模式保留 DWARF）→ 启动 dlv headless → 设置初始断点
//   2. 前端通过 Wails 事件 "debug:halt" 接收暂停通知（断点命中/单步完成）
//   3. DebugContinue/Next/Step/StepOut: 异步执行控制（goroutine 内调用阻塞的 dlv 命令）
//   4. StopDebug: Detach + Kill dlv + 清理临时目录
//
// 事件：
//   - "debug:halt"  : 程序暂停（含 file/line/stopReason）
//   - "debug:exit"  : 程序退出（含 exitStatus）
//   - "debug:log"   : 调试输出（被调试程序 stdout/stderr）
//   - "debug:error" : 调试错误
package app

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"egou/internal/debugger"
	"egou/internal/runner"
)

var errDebugNotRunning = errors.New("调试器未启动")
var errDebugAlreadyRunning = errors.New("调试器已在运行，请先停止当前调试会话")

// BreakpointSpec 是前端传给 StartDebug 的初始断点规格。
type BreakpointSpec struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// DebugVars 是 DebugVariables 的返回值。
type DebugVars struct {
	Locals    []debugger.Variable `json:"locals"`
	Arguments []debugger.Variable `json:"arguments"`
}

// StartDebug 编译项目（debug 模式保留 DWARF）+ 启动 dlv + 设置初始断点。
// projectPath 是项目根目录（会自动收集所有 .eg 源码）。
// breakpoints 是初始断点列表（file 用完整路径或 basename 均可）。
// 编译进度通过 "ide:run-event" 事件回传，调试状态通过 "debug:*" 事件回传。
func (s *IDEService) StartDebug(projectPath string, breakpoints []BreakpointSpec) error {
	s.debugMu.Lock()
	if s.debugClient != nil {
		s.debugMu.Unlock()
		return errDebugAlreadyRunning
	}
	s.debugMu.Unlock()

	if projectPath == "" {
		return errors.New("未打开项目，无法调试")
	}

	// 收集项目源码（与 RunProject 一致：main.eg + 所有 .eg 文件）
	src, err := collectProjectSources(projectPath)
	if err != nil {
		return err
	}

	// 编译（debug 模式，保留 DWARF 调试符号）
	sink := func(ev runner.Event) { s.emitEvent(ev) }
	binaryPath, tmpDir, err := runner.BuildForDebug(src, projectPath, sink)
	if err != nil {
		return err
	}

	// 启动 dlv headless
	client, err := debugger.StartDebug(binaryPath, nil,
		func(state *debugger.DebuggerState) { s.emitDebugHalt(state) },
		func(state *debugger.DebuggerState) { s.emitDebugExit(state) },
		func(line string) { s.emitDebugLog(line) },
	)
	if err != nil {
		os.RemoveAll(tmpDir)
		return err
	}

	s.debugMu.Lock()
	s.debugClient = client
	s.debugTmpDir = tmpDir
	s.debugMu.Unlock()

	// 设置初始断点（//line 指令用 basename，所以这里转 basename）
	for _, bp := range breakpoints {
		if bp.Line <= 0 {
			continue
		}
		file := filepath.Base(bp.File)
		if _, err := client.CreateBreakpoint(file, bp.Line); err != nil {
			s.emitDebugLog("设置断点失败 " + file + ":" + strconv.Itoa(bp.Line) + " " + err.Error())
		}
	}

	// 获取初始状态（dlv exec 启动后程序停在入口）
	state, err := client.State()
	if err == nil && state != nil {
		s.emitDebugHalt(state)
	}
	return nil
}

// StopDebug 停止调试会话：Detach（杀被调试进程）+ Kill dlv + 清理临时目录。
func (s *IDEService) StopDebug() error {
	s.debugMu.Lock()
	client := s.debugClient
	tmpDir := s.debugTmpDir
	s.debugClient = nil
	s.debugTmpDir = ""
	s.debugMu.Unlock()

	if client == nil {
		return errDebugNotRunning
	}
	err := client.Close()
	if tmpDir != "" {
		os.RemoveAll(tmpDir)
	}
	return err
}

// ===== 执行控制（异步：goroutine 内调用阻塞的 dlv 命令）=====

// DebugContinue 继续执行，直到命中断点或程序退出。非阻塞，结果通过事件回传。
func (s *IDEService) DebugContinue() error {
	return s.asyncDebugCommand(func(c *debugger.Client) error {
		_, err := c.Continue()
		return err
	})
}

// DebugNext 单步跳过（不进入函数体）。非阻塞。
func (s *IDEService) DebugNext() error {
	return s.asyncDebugCommand(func(c *debugger.Client) error {
		_, err := c.Next()
		return err
	})
}

// DebugStep 单步进入（进入函数体）。非阻塞。
func (s *IDEService) DebugStep() error {
	return s.asyncDebugCommand(func(c *debugger.Client) error {
		_, err := c.Step()
		return err
	})
}

// DebugStepOut 单步跳出（执行到当前函数返回）。非阻塞。
func (s *IDEService) DebugStepOut() error {
	return s.asyncDebugCommand(func(c *debugger.Client) error {
		_, err := c.StepOut()
		return err
	})
}

// asyncDebugCommand 在 goroutine 中执行阻塞的调试命令。
func (s *IDEService) asyncDebugCommand(fn func(*debugger.Client) error) error {
	client := s.getDebugClient()
	if client == nil {
		return errDebugNotRunning
	}
	go func() {
		if err := fn(client); err != nil {
			s.emitDebugError(err)
		}
	}()
	return nil
}

// ===== 断点管理 =====

// DebugToggleBreakpoint 切换指定文件:行的断点（存在则删除，不存在则创建）。
func (s *IDEService) DebugToggleBreakpoint(file string, line int) error {
	client := s.getDebugClient()
	if client == nil {
		return errDebugNotRunning
	}
	bps, err := client.ListBreakpoints()
	if err != nil {
		return err
	}
	fileBase := filepath.Base(file)
	for _, bp := range bps {
		if bp.File == fileBase && bp.Line == line {
			return client.ClearBreakpoint(bp.ID)
		}
	}
	_, err = client.CreateBreakpoint(fileBase, line)
	return err
}

// DebugListBreakpoints 返回当前所有断点。
func (s *IDEService) DebugListBreakpoints() ([]debugger.Breakpoint, error) {
	client := s.getDebugClient()
	if client == nil {
		return nil, errDebugNotRunning
	}
	return client.ListBreakpoints()
}

// ===== 状态查询 =====

// DebugState 返回当前调试器状态。
func (s *IDEService) DebugState() (*debugger.DebuggerState, error) {
	client := s.getDebugClient()
	if client == nil {
		return nil, errDebugNotRunning
	}
	return client.State()
}

// DebugStacktrace 返回当前 goroutine 的调用栈。depth<=0 时默认 20 层。
func (s *IDEService) DebugStacktrace(depth int) ([]debugger.Stackframe, error) {
	client := s.getDebugClient()
	if client == nil {
		return nil, errDebugNotRunning
	}
	if depth <= 0 {
		depth = 20
	}
	return client.Stacktrace(-1, depth) // -1 = 当前选中 goroutine
}

// DebugVariables 返回指定栈帧的局部变量和函数参数。frame=0 为当前帧。
func (s *IDEService) DebugVariables(frame int) (*DebugVars, error) {
	client := s.getDebugClient()
	if client == nil {
		return nil, errDebugNotRunning
	}
	locals, err := client.ListLocalVars(-1, frame)
	if err != nil {
		return nil, err
	}
	args, err := client.ListFunctionArgs(-1, frame)
	if err != nil {
		return nil, err
	}
	return &DebugVars{Locals: locals, Arguments: args}, nil
}

// IsDebugging 返回调试器是否正在运行。
func (s *IDEService) IsDebugging() bool {
	return s.getDebugClient() != nil
}

// ===== 内部辅助 =====

func (s *IDEService) getDebugClient() *debugger.Client {
	s.debugMu.Lock()
	defer s.debugMu.Unlock()
	return s.debugClient
}

// emitDebugHalt 触发程序暂停事件。
func (s *IDEService) emitDebugHalt(state *debugger.DebuggerState) {
	if s.app == nil || state == nil {
		return
	}
	file := ""
	line := 0
	if state.CurrentThread != nil {
		file = state.CurrentThread.File
		line = state.CurrentThread.Line
	}
	s.app.Event.Emit("debug:halt", map[string]any{
		"file":       file,
		"line":       line,
		"stopReason": state.StopReason,
		"running":    state.Running,
		"exited":     state.Exited,
	})
}

// emitDebugExit 触发程序退出事件。
func (s *IDEService) emitDebugExit(state *debugger.DebuggerState) {
	if s.app == nil {
		return
	}
	s.app.Event.Emit("debug:exit", map[string]any{
		"exitStatus": state.ExitStatus,
	})
}

// emitDebugLog 触发调试输出事件（被调试程序的 stdout/stderr）。
func (s *IDEService) emitDebugLog(line string) {
	if s.app == nil {
		return
	}
	s.app.Event.Emit("debug:log", map[string]any{"line": line})
}

// emitDebugError 触发调试错误事件。
func (s *IDEService) emitDebugError(err error) {
	if s.app == nil || err == nil {
		return
	}
	s.app.Event.Emit("debug:error", map[string]any{"error": err.Error()})
}
