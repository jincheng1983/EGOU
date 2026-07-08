// Package debugger 封装 Delve（dlv）调试器，提供 .eg 源码级调试能力。
//
// 本文件实现 Client：启动 dlv headless 子进程 + JSON-RPC 客户端。
package debugger

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"egou/internal/runner"
)

// delveBinary 指向 dlv 可执行文件路径，空则自动检测（内置 → PATH → GOPATH/bin）。
// 由 IDE 启动时通过 SetDelveBinary 注入（保留接口兼容，正常情况下无需配置）。
var delveBinary string

// SetDelveBinary 设置 dlv 可执行文件路径（留空则自动检测）。
func SetDelveBinary(path string) { delveBinary = path }

// findDelve 查找 dlv 可执行文件：优先用户配置 → 内置（exe同级go/bin/）→ PATH → GOPATH/bin。
func findDelve() (string, error) {
	if delveBinary != "" {
		if _, err := os.Stat(delveBinary); err == nil {
			return delveBinary, nil
		}
	}
	if p := runner.FindDelve(); p != "" {
		return p, nil
	}
	return "", errors.New("未找到 dlv（Delve 调试器），发布包缺失内置 dlv，请重新构建或手动将 dlv 放到 go/bin/ 目录")
}

// listenAddrRe 匹配 dlv 输出的 "API server listening at: ADDR"。
var listenAddrRe = regexp.MustCompile(`API server listening at: (.+)`)

// Client 封装一个 Delve 调试会话。
//
// 生命周期：
//   1. StartDebug() 启动 dlv 子进程 + JSON-RPC 连接
//   2. CreateBreakpoint() 设置断点
//   3. Continue()/Next()/Step() 阻塞执行（在 goroutine 中调用）
//   4. 程序暂停时通过 onHalt 回调通知 IDE
//   5. 程序退出时通过 onExit 回调通知 IDE
//   6. Close() 停止调试会话
type Client struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	conn   *rpc.Client
	addr   string
	closed bool

	// exitOnce 保证 onExit 只触发一次（dlv 进程退出监听器与 executeCommand 竞争触发）
	exitOnce sync.Once
	exitFired int32

	onHalt func(state *DebuggerState)
	onExit func(state *DebuggerState)
	onLog  func(line string)
}

// StartDebug 启动 dlv headless 子进程并连接其 JSON-RPC 服务。
//
//   - binaryPath: 被调试的可执行文件路径（必须带 DWARF 调试符号）
//   - args: 传给被调试程序的命令行参数
//   - onHalt: 程序暂停时回调（断点命中/单步完成/手动暂停），可空
//   - onExit: 被调试程序退出时回调，可空
//   - onLog: dlv/被调试程序输出日志时回调，可空
func StartDebug(binaryPath string, args []string, onHalt, onExit func(*DebuggerState), onLog func(string)) (*Client, error) {
	dlvPath, err := findDelve()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return nil, fmt.Errorf("被调试文件不存在: %w", err)
	}

	dlvArgs := []string{
		"exec", binaryPath,
		"--headless",
		"--api-version=2",
		"--listen=127.0.0.1:0", // 随机端口
		"--accept-multiclient",
		// 关闭 Go 版本检查：dlv 的版本支持列表通常滞后于 Go 发布，
		// 实际上 Go 1.x 之间的 DWARF 格式变化很小，强制关闭检查让调试器能用于新版 Go。
		// 如果新版 Go 确实有不兼容的 DWARF 变化，dlv 会在解析时返回错误，不会导致错误调试。
		"--check-go-version=false",
	}
	if len(args) > 0 {
		dlvArgs = append(dlvArgs, "--")
		dlvArgs = append(dlvArgs, args...)
	}

	cmd := exec.Command(dlvPath, dlvArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdout 管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 dlv 失败: %w", err)
	}

	c := &Client{
		cmd:    cmd,
		onHalt: onHalt,
		onExit: onExit,
		onLog:  onLog,
	}

	// dlv 把 "API server listening at: ADDR" 输出到 stderr，
	// 被调试程序的 stdout 通过 dlv 的 stdout 转发。
	addrCh := make(chan string, 2)
	go c.readStream(stderr, addrCh)
	go c.readStream(stdout, addrCh)

	select {
	case addr := <-addrCh:
		c.addr = addr
	case <-time.After(15 * time.Second):
		c.kill()
		return nil, errors.New("等待 dlv 启动超时（15s），请检查 dlv 是否正常安装")
	}

	conn, err := jsonrpc.Dial("tcp", c.addr)
	if err != nil {
		c.kill()
		return nil, fmt.Errorf("连接 dlv JSON-RPC 失败 (%s): %w", c.addr, err)
	}
	c.conn = conn

	// 启动 dlv 进程退出监听器：
	// dlv 在被调试程序退出后会关闭 RPC 连接（即使 --accept-multiclient），
	// 导致正在阻塞的 Continue/Next 的 RPC 调用返回 "connection closed" 错误，
	// 而非返回 Exited=true 的 state。这里监听 dlv 进程退出，触发 onExit 回调，
	// 让 IDE 能正确感知程序结束。
	go c.watchProcessExit()

	return c, nil
}

// watchProcessExit 监听 dlv 子进程退出，触发 onExit 回调。
// 仅在 executeCommand 未触发 onExit 且非主动 Close 时作为兜底（用 exitOnce 保证只触发一次）。
func (c *Client) watchProcessExit() {
	if c.cmd == nil || c.cmd.Process == nil {
		return
	}
	// 等待 dlv 进程退出（Close 中的 Kill 也会让 Wait 返回）
	_ = c.cmd.Wait()
	// 如果是主动 Close（c.closed=true），不触发 onExit（避免向 IDE 发送误导性的退出事件）
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return
	}
	// 进程退出后，如果 onExit 还没触发，触发一次
	c.fireExit(&DebuggerState{Exited: true, ExitStatus: -1, StopReason: "process exited"})
}

// fireExit 触发 onExit 回调（保证只触发一次）。
func (c *Client) fireExit(state *DebuggerState) {
	if !atomic.CompareAndSwapInt32(&c.exitFired, 0, 1) {
		return
	}
	if c.onExit != nil {
		c.onExit(state)
	}
}

// readStream 逐行读取 dlv 输出，解析监听地址或转发日志。
func (c *Client) readStream(r io.Reader, addrCh chan<- string) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if m := listenAddrRe.FindStringSubmatch(line); m != nil {
			select {
			case addrCh <- strings.TrimSpace(m[1]):
			default:
			}
			continue
		}
		if c.onLog != nil && line != "" {
			c.onLog(line)
		}
	}
}

// call 执行一个 JSON-RPC 调用到 dlv。
func (c *Client) call(method string, args, reply interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.conn == nil {
		return errors.New("调试器未连接")
	}
	return c.conn.Call(method, args, reply)
}

// Close 停止调试会话：先 Detach（杀被调试进程），再 Kill dlv 进程。
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	conn := c.conn
	c.mu.Unlock()

	if conn != nil {
		// 优雅分离：kill=true 终止被调试进程
		_ = conn.Call("RPCServer.Detach", struct {
			Kill bool `json:"kill"`
		}{Kill: true}, nil)
	}
	c.kill()
	return nil
}

// kill 终止 dlv 子进程。
func (c *Client) kill() {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
}

// ===== 断点管理 =====

// CreateBreakpoint 在指定文件:行设置断点，返回带 ID 的断点。
// cond 为条件表达式（Go 语法，如 "i == 10"），为空则无条件断点。
// dlv 的 CreateBreakpoint 响应格式为 {"Breakpoint": {...}}（包装在 Breakpoint 键中）。
func (c *Client) CreateBreakpoint(file string, line int, cond string) (*Breakpoint, error) {
	req := struct {
		Breakpoint Breakpoint `json:"breakpoint"`
	}{Breakpoint: Breakpoint{File: file, Line: line, Cond: cond}}
	var resp struct {
		Breakpoint Breakpoint `json:"Breakpoint"`
	}
	if err := c.call("RPCServer.CreateBreakpoint", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Breakpoint, nil
}

// AmendBreakpoint 修改已有断点的属性（条件、禁用状态等）。
// bp 必须包含有效的 ID 字段，其他字段为待修改的值。
// 用于运行时修改条件断点表达式，无需删除重建断点（保留断点 ID）。
func (c *Client) AmendBreakpoint(bp *Breakpoint) error {
	req := struct {
		Breakpoint Breakpoint `json:"breakpoint"`
	}{Breakpoint: *bp}
	var resp interface{}
	return c.call("RPCServer.AmendBreakpoint", req, &resp)
}

// CreateBreakpointAtFunction 在指定函数入口设置断点。
// functionName 是完整的 Go 函数名（如 "main.mainImpl"）。
// v0.9.9：用于自动设置入口断点，避免在不可执行行（如 # 程序集 注释）设置断点失败。
func (c *Client) CreateBreakpointAtFunction(functionName string) (*Breakpoint, error) {
	req := struct {
		Breakpoint Breakpoint `json:"breakpoint"`
	}{Breakpoint: Breakpoint{FunctionName: functionName}}
	var resp struct {
		Breakpoint Breakpoint `json:"Breakpoint"`
	}
	if err := c.call("RPCServer.CreateBreakpoint", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Breakpoint, nil
}

// ClearBreakpoint 按 ID 删除断点。
func (c *Client) ClearBreakpoint(id int) error {
	req := struct {
		Id int `json:"id"`
	}{Id: id}
	var resp interface{}
	return c.call("RPCServer.ClearBreakpoint", req, &resp)
}

// ListBreakpoints 返回所有断点。
// dlv 的 ListBreakpoints 响应格式为 {"Breakpoints": [...]}。
func (c *Client) ListBreakpoints() ([]Breakpoint, error) {
	var resp struct {
		Breakpoints []Breakpoint `json:"Breakpoints"`
	}
	if err := c.call("RPCServer.ListBreakpoints", struct{}{}, &resp); err != nil {
		return nil, err
	}
	return resp.Breakpoints, nil
}

// ListSources 返回调试器已知的所有源文件路径（包括 //line 指令生成的虚拟文件）。
// 用于诊断断点设置失败：确认 dlv 是否识别了 .eg 文件名。
func (c *Client) ListSources(filter string) ([]string, error) {
	req := struct {
		Filter string `json:"filter"`
	}{Filter: filter}
	// dlv 的 ListSources 返回 {"Sources": [...]} 结构
	var resp struct {
		Sources []string `json:"Sources"`
	}
	if err := c.call("RPCServer.ListSources", req, &resp); err != nil {
		return nil, err
	}
	return resp.Sources, nil
}

// FindLocation 查找代码位置（支持 "file:line" 格式）。
// 用于诊断断点设置：dlv 的 CreateBreakpoint 内部用 FindLocation 解析文件名，
// 如果 FindLocation 找不到，说明 //line 指令的虚拟文件名未被 dlv 识别。
func (c *Client) FindLocation(scopeGoroutineID, frame int, loc string) ([]Location, error) {
	req := struct {
		Scope struct {
			GoroutineID int `json:"goroutineID"`
			Frame       int `json:"frame"`
		} `json:"scope"`
		Loc string `json:"loc"`
	}{
		Loc: loc,
	}
	req.Scope.GoroutineID = scopeGoroutineID
	req.Scope.Frame = frame
	var resp struct {
		Locations []Location `json:"Locations"`
	}
	if err := c.call("RPCServer.FindLocation", req, &resp); err != nil {
		return nil, err
	}
	return resp.Locations, nil
}

// ===== 执行控制（阻塞调用，程序停止时才返回）=====

// Continue 继续执行被调试程序，直到命中断点或程序退出。
// 这是阻塞调用，应在 goroutine 中调用。
func (c *Client) Continue() (*DebuggerState, error) {
	return c.executeCommand("continue")
}

// Next 单步跳过（不进入函数体）。
func (c *Client) Next() (*DebuggerState, error) {
	return c.executeCommand("next")
}

// Step 单步进入（进入函数体）。
func (c *Client) Step() (*DebuggerState, error) {
	return c.executeCommand("step")
}

// StepOut 单步跳出（执行到当前函数返回）。
func (c *Client) StepOut() (*DebuggerState, error) {
	return c.executeCommand("stepOut")
}

// Halt 请求暂停正在运行的程序（中断 Continue 阻塞）。
func (c *Client) Halt() (*DebuggerState, error) {
	req := struct {
		Name string `json:"name"`
	}{Name: "halt"}
	var resp DebuggerState
	if err := c.call("RPCServer.Command", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// executeCommand 执行一个调试命令并触发回调。
func (c *Client) executeCommand(name string) (*DebuggerState, error) {
	req := struct {
		Name string `json:"name"`
	}{Name: name}
	var resp DebuggerState
	if err := c.call("RPCServer.Command", req, &resp); err != nil {
		// Continue/Next 在被调试程序退出时，dlv 会关闭 RPC 连接，
		// 导致 RPC 调用返回 "connection closed" / "wsarecv" 错误。
		// 此时 watchProcessExit 会触发 onExit，这里返回一个表示退出的 state。
		if isConnClosedErr(err) {
			c.fireExit(&DebuggerState{Exited: true, ExitStatus: -1, StopReason: "process exited"})
			return &DebuggerState{Exited: true, ExitStatus: -1, StopReason: "process exited"}, nil
		}
		return nil, err
	}
	if resp.Exited {
		c.fireExit(&resp)
	} else if !resp.Running {
		if c.onHalt != nil {
			c.onHalt(&resp)
		}
	}
	return &resp, nil
}

// isConnClosedErr 判断错误是否由 RPC 连接关闭引起。
// dlv 在被调试程序退出后关闭连接，Continue/Next 的阻塞 RPC 调用会返回此类错误。
func isConnClosedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "wsarecv") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection forcibly closed") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "closed network connection") ||
		strings.Contains(msg, "connection refused")
}

// ===== 状态查询 =====

// State 返回当前调试器状态（非阻塞）。
// dlv 的 State 请求接受 {"nonBlocking": bool}，响应为 DebuggerState 对象。
func (c *Client) State() (*DebuggerState, error) {
	req := struct {
		NonBlocking bool `json:"nonBlocking"`
	}{NonBlocking: true}
	var resp DebuggerState
	if err := c.call("RPCServer.State", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListGoroutines 返回所有 goroutine 列表。
// dlv 的 ListGoroutines 响应格式为 {"Goroutines": [...]}。
// 用于获取实际 goroutine ID（当 DebuggerState.SelectedGoroutine 为 nil 时作为 fallback）。
func (c *Client) ListGoroutines() ([]Goroutine, error) {
	req := struct {
		Count    int `json:"count"`
		Start    int `json:"start"`
	}{}
	var resp struct {
		Goroutines []Goroutine `json:"Goroutines"`
	}
	if err := c.call("RPCServer.ListGoroutines", req, &resp); err != nil {
		return nil, err
	}
	return resp.Goroutines, nil
}

// Stacktrace 返回指定 goroutine 的调用栈。
// goroutineID 为 -1 时使用当前选中的 goroutine。
// dlv 的 Stacktrace 响应格式为 {"Locations": [...]}（包装在 Locations 键中）。
func (c *Client) Stacktrace(goroutineID, depth int) ([]Stackframe, error) {
	req := struct {
		Id    int `json:"id"`
		Depth int `json:"depth"`
	}{Id: goroutineID, Depth: depth}
	var resp struct {
		Locations []Stackframe `json:"Locations"`
	}
	if err := c.call("RPCServer.Stacktrace", req, &resp); err != nil {
		return nil, err
	}
	return resp.Locations, nil
}

// ListLocalVars 返回指定栈帧的局部变量。
// dlv 的 ListLocalVars 响应格式为 {"Vars": [...]}。
func (c *Client) ListLocalVars(goroutineID, frame int) ([]Variable, error) {
	req := struct {
		GoroutineID int `json:"goroutineID"`
		Frame       int `json:"frame"`
	}{GoroutineID: goroutineID, Frame: frame}
	var resp struct {
		Vars []Variable `json:"Vars"`
	}
	if err := c.call("RPCServer.ListLocalVars", req, &resp); err != nil {
		return nil, err
	}
	return resp.Vars, nil
}

// ListFunctionArgs 返回指定栈帧的函数参数。
// dlv 的 ListFunctionArgs 响应格式为 {"Args": [...]}。
func (c *Client) ListFunctionArgs(goroutineID, frame int) ([]Variable, error) {
	req := struct {
		GoroutineID int `json:"goroutineID"`
		Frame       int `json:"frame"`
	}{GoroutineID: goroutineID, Frame: frame}
	var resp struct {
		Args []Variable `json:"Args"`
	}
	if err := c.call("RPCServer.ListFunctionArgs", req, &resp); err != nil {
		return nil, err
	}
	return resp.Args, nil
}

// Detach 分离调试器（kill=true 时终止被调试进程）。
func (c *Client) Detach(kill bool) error {
	req := struct {
		Kill bool `json:"kill"`
	}{Kill: kill}
	var resp interface{}
	return c.call("RPCServer.Detach", req, &resp)
}

// versionRe 匹配语义化版本号 major.minor[.patch]。
var versionRe = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// detectDelveVersion 执行 dlv version，返回版本号字符串（如 "1.25.2"）。
func detectDelveVersion(dlvPath string) (string, error) {
	out, err := exec.Command(dlvPath, "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("执行 dlv version 失败: %w", err)
	}
	m := versionRe.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("无法解析 dlv 版本号: %s", strings.TrimSpace(string(out)))
	}
	return m[0], nil
}

// parseMajorMinor 从版本号字符串提取 "major.minor"（如 "go1.26.4" → "1.26"，"1.25.2" → "1.25"）。
func parseMajorMinor(ver string) string {
	// 剥离 "go"/"v" 前缀
	ver = strings.TrimPrefix(ver, "go")
	ver = strings.TrimPrefix(ver, "v")
	m := versionRe.FindStringSubmatch(ver)
	if m == nil {
		return ""
	}
	return m[1] + "." + m[2]
}

// CheckVersionCompatibility 检测 dlv 与 Go 版本的兼容性。
//
// 规则：dlv 的 major.minor 必须 >= Go 的 major.minor。
// dlv 的运行时数据结构解析能力与 Go 大版本绑定，低版本 dlv 无法解析高版本 Go
// 的 DWARF/runtime 结构（如 dlv 1.25.2 + Go 1.26.4 下断点能命中但无法读取
// goroutine/变量，报 "unknown goroutine 0"）。
//
// 参数 goVersion 格式如 "go1.26.4"（detectGoVersion 返回值）。
// 返回 nil 表示兼容，error 包含明确的升级建议。
func CheckVersionCompatibility(goVersion string) error {
	dlvPath, err := findDelve()
	if err != nil {
		// dlv 未安装，让后续 StartDebug 的 findDelve 报详细错误
		return nil
	}
	dlvVer, err := detectDelveVersion(dlvPath)
	if err != nil {
		return nil // 版本检测失败不阻断，让 dlv 自己启动
	}
	goMM := parseMajorMinor(goVersion)
	dlvMM := parseMajorMinor(dlvVer)
	if goMM == "" || dlvMM == "" {
		return nil // 解析失败不阻断
	}
	if dlvMM < goMM {
		return fmt.Errorf("dlv 版本 %s 与 Go %s 不兼容：dlv 无法解析新版 Go 的运行时数据结构（断点能命中但无法读取 goroutine/变量）。请安装匹配版本的 dlv：go install github.com/go-delve/delve/cmd/dlv@v%s.0",
			dlvVer, goVersion, goMM)
	}
	return nil
}
