// health.go 提供后端健康检查能力，供 IDE 启动时探活各关键依赖。
package runner

import (
	"runtime"
	"strings"

	"egou/internal/types"
)

// P2-19：HealthReport 类型定义已迁移到 egou/internal/types，runner 包内通过别名引用。
type HealthReport = types.HealthReport

// HealthCheck 收集后端关键依赖状态并返回汇总报告。
// 该函数不应返回 error——任何单项失败都通过字段反映，便于前端逐项展示。
func HealthCheck() HealthReport {
	rpt := HealthReport{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// P2-13：统一调用 DetectToolchains，避免 HealthCheck 与 findWails3Cli 检测路径不一致
	tc := DetectToolchains()
	rpt.GoCompiler = tc.Go.Path
	rpt.GoVersion = tc.Go.Version
	rpt.NPM = tc.NPM.Path
	rpt.Wails3CLI = tc.Wails3.Path
	rpt.CCompiler = tc.CGO.Path
	rpt.CGOVersion = tc.CGO.Version
	rpt.Windres = tc.Windres.Path

	// 2. 运行时模板已通过 go:embed 嵌入到 exe 中，无需检查文件系统
	rpt.TemplateDir = "(embedded)"
	rpt.TemplateOK = true

	// 3. 运行时前端缓存：优先报告 exe 同级预打包缓存，其次用户缓存目录
	if binCache, err := binCacheDir(); err == nil && hasFrontendCache(binCache) {
		rpt.CacheDir = binCache + " (exe 同级预打包)"
		rpt.CacheReady = true
	} else if cacheDir, err := runtimeCacheDir(); err == nil {
		rpt.CacheDir = cacheDir
		rpt.CacheReady = hasFrontendCache(cacheDir)
	}

	// 总体判定：Go 编译器是 IDE 正常运行的最小要求（模板已嵌入 exe，总是可用）
	// 缓存未就绪不算致命（首次启动会自动 PrepareRuntimeCache），npm/wails3 仅构建时需要
	rpt.OK = rpt.GoCompiler != "" && rpt.TemplateOK
	if !rpt.OK {
		missing := []string{}
		if rpt.GoCompiler == "" {
			missing = append(missing, "Go 编译器未找到（请确认 go 在 PATH 中，或通过 SetGoBinary 配置路径）")
		}
		rpt.Message = strings.Join(missing, "；")
	} else {
		rpt.Message = "后端服务正常"
	}

	return rpt
}
