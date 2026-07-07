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
	"syscall"
	"time"
)

// delveBinary 指向 dlv 可执行文件路径，空则从 PATH/GOPATH/bin 查找。
// 由 IDE 启动时通过 SetDelveBinary 注入。
var delveBinary string

// SetDelveBinary 设置 dlv 可执行文件路径。
func SetDelveBinary(path string) { delveBinary = path }

// findDelve 查找 dlv 可执行文件：优先 delveBinary，其次 PATH，最后 GOPATH/bin。
func findDelve() (string, error) {
	if delveBinary != "" {
		if _, err := os.Stat(delveBinary); err == nil {
			return delveBinary, nil
		}
	}
	if p, err := exec.LookPath("dlv"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("dlv.exe"); err == nil {
		return p, nil
	}
	gp := os.Getenv("GOPATH")
	if gp == "" {
		gp = os.Getenv("USERPROFILE") + "/go"
	}
	for _, c := range []string{gp + "/bin/dlv.exe", gp + "/bin/dlv"} {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", errors.New("未找到 dlv（Delve 调试器），请先执行: go install github.com/go-delve/delve/cmd/dlv@latest")
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

	return c, nil
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
func (c *Client) CreateBreakpoint(file string, line int) (*Breakpoint, error) {
	req := struct {
		Breakpoint Breakpoint `json:"breakpoint"`
	}{Breakpoint: Breakpoint{File: file, Line: line}}
	var resp Breakpoint
	if err := c.call("RPCServer.CreateBreakpoint", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
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
func (c *Client) ListBreakpoints() ([]Breakpoint, error) {
	var resp []Breakpoint
	if err := c.call("RPCServer.ListBreakpoints", struct{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
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
		return nil, err
	}
	if resp.Exited {
		if c.onExit != nil {
			c.onExit(&resp)
		}
	} else if !resp.Running {
		if c.onHalt != nil {
			c.onHalt(&resp)
		}
	}
	return &resp, nil
}

// ===== 状态查询 =====

// State 返回当前调试器状态（非阻塞）。
func (c *Client) State() (*DebuggerState, error) {
	var resp DebuggerState
	if err := c.call("RPCServer.State", struct{}{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Stacktrace 返回指定 goroutine 的调用栈。
// goroutineID 为 -1 时使用当前选中的 goroutine。
func (c *Client) Stacktrace(goroutineID, depth int) ([]Stackframe, error) {
	req := struct {
		Id    int `json:"id"`
		Depth int `json:"depth"`
	}{Id: goroutineID, Depth: depth}
	var resp []Stackframe
	if err := c.call("RPCServer.Stacktrace", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ListLocalVars 返回指定栈帧的局部变量。
func (c *Client) ListLocalVars(goroutineID, frame int) ([]Variable, error) {
	req := struct {
		GoroutineID int `json:"goroutineID"`
		Frame       int `json:"frame"`
	}{GoroutineID: goroutineID, Frame: frame}
	var resp []Variable
	if err := c.call("RPCServer.ListLocalVars", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ListFunctionArgs 返回指定栈帧的函数参数。
func (c *Client) ListFunctionArgs(goroutineID, frame int) ([]Variable, error) {
	req := struct {
		GoroutineID int `json:"goroutineID"`
		Frame       int `json:"frame"`
	}{GoroutineID: goroutineID, Frame: frame}
	var resp []Variable
	if err := c.call("RPCServer.ListFunctionArgs", req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Detach 分离调试器（kill=true 时终止被调试进程）。
func (c *Client) Detach(kill bool) error {
	req := struct {
		Kill bool `json:"kill"`
	}{Kill: kill}
	var resp interface{}
	return c.call("RPCServer.Detach", req, &resp)
}
