// Package debugger 封装 Delve（dlv）调试器，提供 .eg 源码级调试能力。
//
// 设计：
//   - 通过子进程启动 `dlv exec <binary> --headless --api-version=2`
//   - 用 Go 标准库 net/rpc/jsonrpc 连接 dlv 的 JSON-RPC 服务
//   - 不导入 delve 包作为依赖（避免引入 LLVM 等重型依赖），仅定义用到的最小类型集
//   - 支持 .eg 源码级断点（依赖转译器生成的 //line 指令）
//
// 事件流：
//   - Continue/Next/Step/StepOut 都是阻塞调用，返回 DebuggerState
//   - 程序暂停时通过 onHalt 回调通知 IDE（文件/行号/停止原因）
//   - 程序退出时通过 onExit 回调通知 IDE
package debugger

// Breakpoint 表示一个断点。
type Breakpoint struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Addr         uint64 `json:"addr"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	FunctionName string `json:"functionName"`
	Cond         string `json:"cond"`
	HitCount     map[int]int `json:"hitCount"`
	TotalHitCount int    `json:"totalHitCount"`
	Disabled     bool   `json:"disabled"`
}

// Location 表示代码位置（文件:行:PC）。
type Location struct {
	PC       uint64 `json:"pc"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function *Func  `json:"function"`
}

// Func 表示函数信息。
type Func struct {
	Name   string `json:"name"`
	Value  uint64 `json:"value"`
	Type   byte   `json:"type"`
	GoType uint64 `json:"goType"`
}

// Thread 表示一个线程。
// GoroutineID 是该线程当前绑定的 goroutine ID（v0.9.4 添加），
// 断点命中时 CurrentThread.GoroutineID 就是用户代码 goroutine ID，
// 比 ListGoroutines 的 UserLoc 更可靠（dlv 1.25.2 下 UserLoc 经常为空）。
type Thread struct {
	ID          int         `json:"id"`
	PC          uint64      `json:"pc"`
	File        string      `json:"file"`
	Line        int         `json:"line"`
	Function    *Func       `json:"function"`
	GoroutineID int         `json:"goroutineID"`
	Breakpoint  *Breakpoint `json:"breakPoint"`
}

// Goroutine 表示一个 goroutine。
// 注意：dlv 的 Status 字段是 GoroutineStatus（int64 枚举），不是字符串。
//   0=Running 1=Waiting 2=Blocked 3=Syscall 4=WaitingForGc 5=Select 6=TimeSleep 7=Dead
type Goroutine struct {
	ID            int      `json:"id"`
	CurrentLoc    Location `json:"currentLoc"`
	UserLoc       Location `json:"userLoc"`
	GoLoc         Location `json:"goLoc"`
	StartLoc      Location `json:"startLoc"`
	Status        int      `json:"status"`
}

// DebuggerState 表示调试器当前状态（Continue/Step 返回值）。
type DebuggerState struct {
	Pid              int          `json:"pid"`
	Running          bool         `json:"running"`
	Exited           bool         `json:"exited"`
	ExitStatus       int          `json:"exitStatus"`
	StopReason       string       `json:"stopReason"`
	CurrentThread    *Thread      `json:"currentThread"`
	SelectedGoroutine *Goroutine  `json:"selectedGoroutine"`
	Threads          []*Thread    `json:"threads"`
	NextInProgress   bool         `json:"nextInProgress"`
}

// Stackframe 表示调用栈中的一帧。
type Stackframe struct {
	Location
	Locals   []Variable `json:"locals"`
	Arguments []Variable `json:"arguments"`
}

// Variable 表示一个变量/表达式的值。
// Kind 是 reflect.Kind 的数值形式（dlv 返回 uint，不是字符串）。
type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
	Addr  uint64 `json:"addr"`
	Kind  int    `json:"kind"`
}

// LoadConfig 控制变量值的加载深度（dlv 的 LoadConfig）。
type LoadConfig struct {
	FollowPointers    bool `json:"followPointers"`
	MaxVariableRecurse int  `json:"maxVariableRecurse"`
	MaxStringLen      int  `json:"maxStringLen"`
	MaxArrayValues    int  `json:"maxArrayValues"`
	MaxStructFields   int  `json:"maxStructFields"`
}

// DefaultLoadConfig 返回适合 IDE 显示的默认 LoadConfig。
func DefaultLoadConfig() LoadConfig {
	return LoadConfig{
		FollowPointers:    true,
		MaxVariableRecurse: 1,
		MaxStringLen:      128,
		MaxArrayValues:    64,
		MaxStructFields:   -1,
	}
}

// StopReason 常量（dlv 返回的 StopReason 字符串）。
const (
	StopBreakpoint       = "breakpoint"
	StopNext             = "next"
	StopStep             = "step"
	StopManual           = "manual"
	StopRuntimeError     = "runtimeError"
	StopHardcodedBreak   = "hardcodedBreakpoint"
	StopSharedLibLoaded  = "shared library loaded"
)
