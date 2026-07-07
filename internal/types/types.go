// Package types 集中放置跨层共享的类型定义（Wails binding 契约 + runner/ide_service 共享 DTO）。
//
// 设计目标（P2-19，吸取 NxEGO6）：
//   - 避免 runner 包与 package main 之间循环依赖
//   - 统一 JSON tag 规范，让前端契约单一来源
//   - 为未来 AI/调试/扩展系统的新类型预留集中位置
package types

// Event 表示运行过程中产生的一条事件。
// Stage 标识当前阶段，Output 是面向用户的中文提示或运行时的 stdout/stderr 文本行。
// 当 IsOutput=true 时表示该行是运行时程序的标准输出（来自 用户代码中的 打印 等）。
type Event struct {
	Stage    string `json:"stage"`
	Output   string `json:"output"`
	IsOutput bool   `json:"isOutput"`
}

// 事件 Stage 命名空间常量（P3-6，吸取 NxEGO5 调试事件分类）。
// 用「命名空间:子类型」格式，便于前端按前缀过滤订阅，避免字符串散落。
//
// 现有编译事件保持向后兼容（runner.go 中 emit("build"/"transpile"/"stage"/"artifact")），
// 前端 App.vue 的 stage switch case 仍使用旧值。下方常量供未来调试器/测试器使用。
//
//   - engine:*   调试引擎事件（启动/连接/停止/断开）
//   - node:*     节点执行事件（进入/离开/异常，供未来流程图可视化）
//   - test:*     测试事件（套件/用例 开始-通过-失败）
//   - action:*   用户动作事件（单步/继续/暂停/切换断点）
//   - runtime:*  运行时输出（stdout/stderr，对应 IsOutput=true）
const (
	StageEngineStart    = "engine:start"     // 调试器启动
	StageEngineStop     = "engine:stop"      // 调试器停止
	StageEngineAttached = "engine:attached"  // 已附加到目标进程
	StageEngineDetached = "engine:detached"  // 已分离
	StageEngineError    = "engine:error"     // 引擎错误

	StageNodeEnter  = "node:enter"   // 进入节点
	StageNodeLeave  = "node:leave"   // 离开节点
	StageNodeError  = "node:error"   // 节点执行异常
	StageNodeOutput = "node:output"  // 节点产生的输出

	StageTestSuiteStart = "test:suite-start"
	StageTestSuiteEnd   = "test:suite-end"
	StageTestCaseStart  = "test:case-start"
	StageTestCasePass   = "test:case-pass"
	StageTestCaseFail   = "test:case-fail"

	StageActionStepOver  = "action:step-over"
	StageActionStepInto  = "action:step-into"
	StageActionStepOut   = "action:step-out"
	StageActionContinue  = "action:continue"
	StageActionPause     = "action:pause"
	StageActionToggleBP  = "action:toggle-bp"

	StageRuntimeStdout = "runtime:stdout"
	StageRuntimeStderr = "runtime:stderr"
)

// 循环防死循环双重保护常量（P3-7，吸取 NxEGO5）。
//
// EGOU 采用「转译成 Go 原生代码」架构，没有解释器节点概念，因此：
//   - MaxLoopIterations：判断循环（do-while 模式）的单体迭代上限，默认 0=不启用
//     通过环境变量 NXG_MAX_LOOP_ITERATIONS 控制（>0 时转译器注入计数器检查）
//   - MaxNodeVisits：保留供未来解释器/调试器节点访问上限使用，当前架构不生效
//
// 启用方式（仅对「判断循环」生效，不影响 for init;cond;post 和 range 循环）：
//   NXG_MAX_LOOP_ITERATIONS=10000 egruntime
//   或 IDE 在 runner.runRuntime 中通过 env 注入
//
// 触发上限时，运行时会 panic("判断循环迭代次数超过上限 N") 并退出，便于用户排查死循环。
const (
	MaxLoopIterations = 10000 // 默认上限（环境变量启用后生效）
	MaxNodeVisits     = 500   // 单节点访问上限（保留供未来解释器模式）
)

// EnvMaxLoopIterations 是控制循环保护开关的环境变量名。
const EnvMaxLoopIterations = "NXG_MAX_LOOP_ITERATIONS"

// EventSink 接收运行过程中产生的事件；为 nil 时静默忽略。
type EventSink func(Event)

// CompileError 表示一条结构化的 Go 编译错误。
// 前端可据此跳转到对应文件行。
type CompileError struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error" | "warning"
}

// HealthReport 汇总后端各关键依赖的运行状态。
// 前端通过 IDEService.HealthCheck() 拿到此结构后，可在状态栏展示整体健康度。
type HealthReport struct {
	OK          bool   `json:"ok"`          // 总体是否健康（所有关键项都通过）
	Message     string `json:"message"`     // 总体说明，失败时给出首要原因
	GoCompiler  string `json:"goCompiler"`  // Go 编译器路径，空表示未找到
	GoVersion   string `json:"goVersion"`   // Go 版本号，如 "go1.23.4"
	OS          string `json:"os"`          // 运行时操作系统（runtime.GOOS）
	Arch        string `json:"arch"`        // 运行时 CPU 架构（runtime.GOARCH）
	TemplateDir string `json:"templateDir"` // 运行时模板来源（已嵌入 exe，显示 "(embedded)"）
	TemplateOK  bool   `json:"templateOk"`  // 模板是否可用（嵌入后总是 true）
	CacheDir    string `json:"cacheDir"`    // 运行时前端缓存目录绝对路径
	CacheReady  bool   `json:"cacheReady"`  // 缓存是否包含有效前端构建产物
	NPM         string `json:"npm"`         // npm 可执行文件路径，空表示未找到
	Wails3CLI   string `json:"wails3Cli"`   // Wails v3 CLI 路径，空表示未配置
	// P2-13：C 编译器（cgo 用），gcc 优先，clang 回退
	CCompiler  string `json:"cCompiler"`  // C 编译器路径，空表示未找到
	CGOVersion string `json:"cgoVersion"` // C 编译器版本号
	Windres    string `json:"windres"`    // windres 路径（生成 .syso 资源用），空表示未找到
	// P2 调试器：Delve（dlv）路径，空表示未安装（调试功能不可用）
	Delve string `json:"delve"` // dlv 路径，空表示未找到
}
