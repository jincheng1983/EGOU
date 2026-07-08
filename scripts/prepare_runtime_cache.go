//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"egou/internal/runner"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "获取 CWD 失败:", err)
		os.Exit(1)
	}
	root := cwd
	binCache := filepath.Join(root, "bin", "runtime-frontend")
	if err := os.MkdirAll(binCache, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "创建缓存目录失败:", err)
		os.Exit(1)
	}

	// 设置模板目录为源码 runtime/wails-template（已含 vendor）
	// build.py 调用本脚本时在第4步，bin/wails-template/ 尚未复制完成
	templateDir := filepath.Join(root, "runtime", "wails-template")
	runner.SetTemplateDir(templateDir)

	// 配置 Go SDK 路径：优先使用本机 go 命令（build.py 在开发机上运行，有 Go 环境）
	// 不调用 runner.SetGoBinary，默认用 PATH 中的 go 即可

	fmt.Printf("[prepare_runtime_cache] 预构建运行时前端缓存到: %s\n", binCache)
	fmt.Printf("[prepare_runtime_cache] 使用模板目录: %s\n", templateDir)
	if err := runner.PrepareRuntimeCacheTo(binCache); err != nil {
		fmt.Fprintln(os.Stderr, "预构建运行时缓存失败:", err)
		os.Exit(1)
	}
	fmt.Println("[prepare_runtime_cache] 完成（包含 frontend/dist + frontend/bindings）")
}
