package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// runtimeUIService 是运行时全局 UI 服务，供用户注入的 Go 代码调用。
var runtimeUIService *UIService

// Version 由构建时通过 -ldflags "-X main.Version=xxx" 注入，默认 dev。
var Version = "dev"

func init() {
	// GUI 子系统下标准输出不会立即显示到 IDE，将运行时日志写入文件便于排查。
	logPath := os.Getenv("EG_LOG_PATH")
	if logPath == "" {
		if exe, err := os.Executable(); err == nil {
			logPath = filepath.Join(filepath.Dir(exe), "egruntime.log")
		} else {
			logPath = "egruntime.log"
		}
	}
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
		log.SetOutput(f)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("[runtime] 版本: %s", Version)
}

func main() {
	// Wails v3 需要在主 OS 线程上运行事件循环与窗口创建。
	runtime.LockOSThread()

	log.Println("[runtime] main() started")
	runtimeUIService = NewUIService()

	// WebView2 默认尝试写入 AppData\Roaming，在受限沙盒中可能失败。
	// 将用户数据目录设置到可执行文件所在目录，确保有写入权限。
	userDataDir := "webview-data"
	if exe, err := os.Executable(); err == nil {
		userDataDir = filepath.Join(filepath.Dir(exe), "webview-data")
	}
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		log.Printf("[runtime] 创建 WebView2 用户数据目录失败: %v", err)
	}
	log.Printf("[runtime] WebView2 用户数据目录: %s", userDataDir)

	app := application.New(application.Options{
		Name:        "EGOU Runtime",
		Description: "EGOU 运行时",
		Windows: application.WindowsOptions{
			WebviewUserDataPath: userDataDir,
		},
		Services: []application.Service{
			application.NewService(runtimeUIService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		ErrorHandler: func(err error) {
			log.Printf("[runtime] Wails Error: %v", err)
		},
		WarningHandler: func(msg string) {
			log.Printf("[runtime] Wails Warning: %s", msg)
		},
	})
	runtimeUIService.app = app
	log.Println("[runtime] Wails app created")

	// 用户注入的 .eg 转译代码定义 mainImpl() 与 registerHandlersImpl()。
	// 通过 ServiceStartup 在 Wails 事件循环启动后执行，这样主函数中的
	// 对话框、窗口创建、剪贴板等需要主消息循环的 API 才能正常工作。
	runtimeUIService.SetMainFuncs(registerHandlersImpl, mainImpl)

	log.Println("[runtime] 启动 Wails 事件循环...")
	err := app.Run()
	if err != nil {
		log.Printf("[runtime] app.Run() 返回错误: %v", err)
	}
	log.Println("[runtime] 程序退出")
}
