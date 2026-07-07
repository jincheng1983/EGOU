// Package main 是 EGOU IDE 的应用入口。
//
// EGOU 第八版采用完整外置化架构：所有资源以磁盘文件形式存放在 exe 同级目录。
// IDE 启动时从磁盘加载 frontend/dist（WebView 资源）、fonts/（字体）、
// examples/（示例 .elib）、wails-template/（用户程序编译模板）。
//
// Version 由 build.py 通过 -ldflags -X main.Version=xxx 注入。
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"egou/internal/app"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Version 由 build.py 通过 -ldflags -X main.Version=xxx 注入
var Version = "dev"

// resolveExeDir 返回 IDE 可执行文件所在目录（外置资源的根目录）。
// 开发模式下 os.Executable 指向 cmd/egou/，但外置资源在 bin/，
// 因此优先用 EGOU_DEV_DIR 环境变量指定资源目录（build.py 启动 IDE 时设置）。
func resolveExeDir() string {
	if dev := os.Getenv("EGOU_DEV_DIR"); dev != "" {
		return dev
	}
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

func main() {
	exeDir := resolveExeDir()

	// 先构造 IDEService（app 暂为 nil，toolManager 等子组件已初始化），
	// 再创建 application，最后回填 app 引用。
	ideService := app.NewIDEService(nil)
	ideService.SetResourceDir(exeDir) // 注入外置资源根目录

	// 从磁盘加载前端构建产物作为 WebView 资源
	distDir := filepath.Join(exeDir, "frontend", "dist")
	var handler http.Handler
	if _, err := os.Stat(distDir); err == nil {
		handler = application.AssetFileServerFS(os.DirFS(distDir))
	} else {
		// 开发模式回退：使用工作目录下的 frontend/dist
		handler = application.AssetFileServerFS(os.DirFS("frontend/dist"))
	}

	wailsApp := application.New(application.Options{
		Name:        "EGOU IDE",
		Description: "易狗 IDE - 中文 Go 编程环境",
		Services: []application.Service{
			application.NewService(ideService),
		},
		Assets: application.AssetOptions{
			Handler: handler,
		},
	})
	ideService.SetApp(wailsApp)

	win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "EGOU IDE",
		Width:            1280,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		Frameless:        true,
		BackgroundColour: application.NewRGB(22, 22, 42),
		URL:              "/",
	})
	ideService.SetWindow(win)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
