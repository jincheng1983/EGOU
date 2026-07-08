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

	// dlv 与 Go 版本兼容性检测（编译前检查，避免浪费编译时间）
	// 规则：dlv major.minor 必须 >= Go major.minor，否则 dlv 无法解析新版 Go 的 runtime 结构
	tc := runner.DetectToolchains()
	if tc.Go.Version != "" {
		if err := debugger.CheckVersionCompatibility(tc.Go.Version); err != nil {
			return err
		}
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
	hasUserBreakpoint := false
	for _, bp := range breakpoints {
		if bp.Line <= 0 {
			continue
		}
		file := filepath.Base(bp.File)
		if _, err := client.CreateBreakpoint(file, bp.Line); err != nil {
			s.emitDebugLog("设置断点失败 " + file + ":" + strconv.Itoa(bp.Line) + " " + err.Error())
		} else {
			hasUserBreakpoint = true
		}
	}

	// v0.9.9：如果用户没有设置断点，自动在主函数入口设置断点。
	// 转译器把 .eg 的"主函数"转译为 Go 的 main.mainImpl（见 transpiler 主函数处理）。
	// 用函数名断点避免在不可执行行（如 # 程序集 注释）设置断点失败。
	if !hasUserBreakpoint {
		if _, err := client.CreateBreakpointAtFunction("main.mainImpl"); err != nil {
			s.emitDebugLog("自动设置入口断点失败: " + err.Error())
		} else {
			hasUserBreakpoint = true
		}
	}

	// v0.9.8：自动 Continue 到第一个断点。
	// dlv exec 启动后程序停在 runtime 入口（非用户代码），
	// 如果用户此时点击单步，会在 runtime 中单步，导致 "no source for PC 0x..." 错误。
	// 自动 Continue 让程序运行到用户代码断点处停止。
	if hasUserBreakpoint {
		go func() {
			state, err := client.Continue()
			if err != nil {
				// Continue 返回错误（可能是连接关闭，程序已退出）
				s.emitDebugError(err)
				return
			}
			if state != nil && state.Exited {
				s.emitDebugExit(state)
			}
			// Continue 成功停止时 onHalt 回调已触发，这里不需要再处理
		}()
	} else {
		// 没有断点时获取初始状态（程序停在 runtime 入口）
		state, err := client.State()
		if err == nil && state != nil {
			s.emitDebugHalt(state)
		}
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
	s.currentGoroutineID = 0 // 重置 goroutine ID（v0.9.4）
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
	gid := s.getCurrentGoroutineID()
	return client.Stacktrace(gid, depth)
}

// DebugVariables 返回指定栈帧的局部变量和函数参数。frame=0 为当前帧。
func (s *IDEService) DebugVariables(frame int) (*DebugVars, error) {
	client := s.getDebugClient()
	if client == nil {
		return nil, errDebugNotRunning
	}
	gid := s.getCurrentGoroutineID()
	locals, err := client.ListLocalVars(gid, frame)
	if err != nil {
		return nil, err
	}
	args, err := client.ListFunctionArgs(gid, frame)
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
// 同时维护 currentGoroutineID（v0.9.4：解决 dlv 1.25.2 SelectedGoroutine 为空导致 ListLocalVars "unknown goroutine 0"）。
//
// goroutine ID 解析优先级：
//  1. state.SelectedGoroutine.ID（最准确，但 dlv 1.25.2 + Go 1.26.4 下经常为空）
//  2. state.CurrentThread.GoroutineID（断点命中的线程绑定的 goroutine，最可靠）
//  3. 主动调用 State() 获取完整状态再试 1/2（应对 Continue 返回不完整）
//  4. ListGoroutines 中 UserLoc 在 .eg 文件上的 goroutine（fallback）
//  5. ListGoroutines 第一个 goroutine（最后兜底）
//
// 注意：dlv 1.25.2 + Go 1.26.4 下 1-4 全部失效（State() 返回 threads=0），
// 此时 ListLocalVars 无法工作。根本解决：用户安装与 Go 版本匹配的 dlv（v0.9.3 已支持配置 dlv 路径）。
func (s *IDEService) emitDebugHalt(state *debugger.DebuggerState) {
	if s.app == nil || state == nil {
		return
	}
	// 维护当前 goroutine ID
	resolvedGid := 0
	if state.SelectedGoroutine != nil && state.SelectedGoroutine.ID > 0 {
		resolvedGid = state.SelectedGoroutine.ID
	} else if state.CurrentThread != nil && state.CurrentThread.GoroutineID > 0 {
		// 断点命中时 CurrentThread.GoroutineID 就是用户代码 goroutine（最可靠）
		resolvedGid = state.CurrentThread.GoroutineID
	}
	// 若 Continue 返回的 state 不完整，主动调用 State() 获取完整状态
	if resolvedGid == 0 && state.CurrentThread == nil {
		if client := s.getDebugClient(); client != nil {
			if fullState, err := client.State(); err == nil && fullState != nil {
				if fullState.SelectedGoroutine != nil && fullState.SelectedGoroutine.ID > 0 {
					resolvedGid = fullState.SelectedGoroutine.ID
				} else if fullState.CurrentThread != nil && fullState.CurrentThread.GoroutineID > 0 {
					resolvedGid = fullState.CurrentThread.GoroutineID
				}
				// 用完整状态的 CurrentThread 补全 file/line
				if fullState.CurrentThread != nil {
					state.CurrentThread = fullState.CurrentThread
				}
			}
		}
	}
	// 仍为空时通过 ListGoroutines fallback
	if resolvedGid == 0 {
		if client := s.getDebugClient(); client != nil {
			if gs, err := client.ListGoroutines(); err == nil {
				for _, g := range gs {
					if isEgFile(g.UserLoc.File) {
						resolvedGid = g.ID
						break
					}
				}
				if resolvedGid == 0 && len(gs) > 0 {
					resolvedGid = gs[0].ID
				}
			}
		}
	}
	if resolvedGid > 0 {
		s.setGoroutineID(resolvedGid)
	}

	file := ""
	line := 0
	if state.CurrentThread != nil {
		file = state.CurrentThread.File
		line = state.CurrentThread.Line
	}
	// v0.9.11：当 CurrentThread 为空或 file/line 为空时（dlv 版本不兼容），
	// 通过 Stacktrace 获取第一帧的 file/line，确保编辑器能高亮当前执行行。
	if (file == "" || line == 0) && resolvedGid > 0 {
		if client := s.getDebugClient(); client != nil {
			if stack, err := client.Stacktrace(resolvedGid, 5); err == nil && len(stack) > 0 {
				// 优先找 .eg 文件的栈帧（跳过 runtime 帧）
				for _, frame := range stack {
					if isEgFile(frame.File) {
						file = frame.File
						line = frame.Line
						break
					}
				}
				// 如果没有 .eg 帧，用第一帧
				if file == "" && stack[0].File != "" {
					file = stack[0].File
					line = stack[0].Line
				}
			}
		}
	}
	s.app.Event.Emit("debug:halt", map[string]any{
		"file":       file,
		"line":       line,
		"stopReason": state.StopReason,
		"running":    state.Running,
		"exited":     state.Exited,
	})
}

// isEgFile 判断路径是否是 .eg 源文件（dlv 返回的 file 可能是绝对路径或 basename）。
func isEgFile(path string) bool {
	if path == "" {
		return false
	}
	return filepath.Ext(path) == ".eg"
}

// getCurrentGoroutineID 返回当前记录的 goroutine ID（线程安全）。
// 返回 -1 表示未记录（让 dlv 自己选择当前 goroutine）。
func (s *IDEService) getCurrentGoroutineID() int {
	s.debugMu.Lock()
	defer s.debugMu.Unlock()
	if s.currentGoroutineID <= 0 {
		return -1
	}
	return s.currentGoroutineID
}

// setGoroutineID 设置当前 goroutine ID（线程安全）。
func (s *IDEService) setGoroutineID(id int) {
	s.debugMu.Lock()
	s.currentGoroutineID = id
	s.debugMu.Unlock()
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
