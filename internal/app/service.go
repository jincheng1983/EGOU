// Package app 实现 EGOU IDE 的 Wails Service 层。
//
// 第七版 EGOU 的 ide_service.go（2300+行/44 方法）按职责拆分到本包下多个文件：
//   - service.go  : IDEService 结构体 + 生命周期（ServiceStartup/ServiceShutdown）+ ServiceName
//   - project.go  : 项目管理（OpenProject/CreateProject/配置读写/模板）
//   - file.go     : 文件操作（OpenFile/SaveFile/QuickSave/ListProjectDir）
//   - compile.go  : 编译运行 + 符号索引（Transpile/Run/Build/BuildProject/RunProject/ListSymbols/...）
//   - ai.go       : AI 方法（RunAgent/RunAgentPipeline/AIChat/ConfirmToolCall）
//   - libs.go     : 支持库/扩展包（ScanGlobalLibs/CreateElib/RenameElib/DeleteElib）
//   - plugins.go  : 插件（ScanPlugins/ReadPluginFile）
//   - system.go   : 系统（HealthCheck/ToggleFullscreen/OpenInExplorer/CheckSignature）
//
// 拆分目的：单一职责、可读性、可维护性。每个文件 200-400 行。
package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"egou/internal/ai"
	"egou/internal/debugger"
	"egou/internal/runner"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// IDEService 是 EGOU IDE 暴露给前端（Wails binding）的核心服务。
//
// 字段说明：
//   - app         : Wails application 实例（事件触发、窗口管理用），由 main.go 回填
//   - win         : 主窗口实例（标题栏控制、全屏切换用），由 main.go 回填
//   - goBinary    : Go 编译器路径（可配置，runner.SetGoBinary 用）
//   - toolManager : AI 危险工具人机确认管理器（P2-11），通过 Wails 事件通知前端
//   - resourceDir : 外置资源根目录（exe 同级），存放 frontend/dist、libs、templates、wails-template
type IDEService struct {
	app         *application.App
	win         *application.WebviewWindow
	goBinary    string
	toolManager *ai.ToolManager
	resourceDir string

	// 调试器状态（P2 Delve 集成）
	debugMu     sync.Mutex
	debugClient *debugger.Client
	debugTmpDir string // 调试编译的临时目录，调试结束后清理
}

// NewIDEService 构造一个 IDEService 实例。
//
// app 参数允许为 nil（main.go 需要先有 service 实例才能创建 application），
// 创建 application 后通过 SetApp 回填。
//
// 同时初始化 AI 工具确认管理器，通过 Wails 事件 "ai-tool-confirm" 通知前端。
func NewIDEService(app *application.App) *IDEService {
	s := &IDEService{app: app}
	s.toolManager = ai.NewToolManager(func(req *ai.ToolConfirmRequest) {
		if s.app != nil {
			s.app.Event.Emit("ai-tool-confirm", map[string]any{
				"id":        req.ID,
				"tool":      req.Tool,
				"summary":   req.Summary,
				"params":    req.Params,
				"risk":      string(req.Risk),
				"createdAt": req.CreatedAt.Unix(),
			})
		}
	})
	return s
}

// SetApp 由 main.go 在 application.New 之后回填 app 引用。
func (s *IDEService) SetApp(app *application.App) {
	s.app = app
}

// SetWindow 由 main.go 在创建主窗口后回填窗口引用。
func (s *IDEService) SetWindow(win *application.WebviewWindow) {
	s.win = win
}

// SetResourceDir 由 main.go 注入外置资源根目录（exe 同级）。
// IDE 启动时从该目录加载 fonts/、examples/、wails-template/ 等外置资源。
func (s *IDEService) SetResourceDir(dir string) {
	s.resourceDir = dir
}

// SetBuildOptions 由前端同步"编译选项"设置到后端。
// garbleLevel 控制用户编译产物的 Garble 源码混淆强度：
//   - "off"   : 普通 go build，产物保留原始符号（便于调试自己的程序）
//   - "basic" : garble -tiny，仅混淆变量名/函数名/类型名 + 移除文件名/行号（默认，无杀软误杀）
//   - "full"  : garble -literals -tiny，再加字符串字面量运行时解密（最强，但可能触发杀软误报）
// 前端在 IDE 启动时和强度下拉变化时调用此方法。
// v0.8.0 起 UPX 完全移除（杀软误杀严重），Garble 成为唯一防逆向手段。
// v0.8.0 修订：原布尔开关改为三档强度，吸取 -literals 触发 TrojanSpy/Stealer.uj 误报的教训。
func (s *IDEService) SetBuildOptions(garbleLevel string) {
	runner.SetGarbleLevel(garbleLevel)
}

// ServiceName 实现 Wails Service 接口，返回服务唯一标识。
// Wails v3 要求每个 Service 必须实现此方法用于注册。
func (s *IDEService) ServiceName() string {
	return "egou.IDEService"
}

// ServiceStartup 在 Wails application 启动时由框架调用。
//
// 异步执行：
//   - 注入外置资源目录给 runner（wails-template 路径）
//   - 预热运行时缓存（runner.PrepareRuntimeCache）
//   - 工具链检测预热
func (s *IDEService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	// 同步注入模板目录（编译用户程序时立即可用）
	if s.resourceDir != "" {
		runner.SetTemplateDir(filepath.Join(s.resourceDir, "wails-template"))
	}
	go func() {
		if err := runner.PrepareRuntimeCache(); err != nil {
			fmt.Println("[egou] prepare runtime cache warning:", err)
		}
	}()
	return nil
}

// ServiceShutdown 在 Wails application 关闭时由框架调用。
//
// 阶段二为简化实现，未做持久化；后续阶段可在这里：
//   - 清理项目级库数据
//   - 保存未保存的编辑器内容
//   - 持久化会话状态
func (s *IDEService) ServiceShutdown() {
	// 阶段二：空实现
}
