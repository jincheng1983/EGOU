// detector.go 集中放置工具链检测逻辑（P2-13，吸取 NxEGO3 backend/compiler/config.go）。
//
// 设计目标：
//   - 统一 Go/gcc/clang/windres/npm/upx/rsrc/wails3 的检测入口
//   - 不只依赖 PATH，还扫描常见安装路径（MinGW/LLVM/Program Files）
//   - 通过 --version 解析版本号，供 HealthCheck 上报
//   - DetectToolchains() 返回结构化报告，便于未来扩展（多平台/多编译器选择）
package runner

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
)

// ToolchainInfo 描述单个工具链的检测信息。
type ToolchainInfo struct {
	Name    string `json:"name"`    // 工具名（go/gcc/clang/windres/npm/garble/rsrc/wails3）
	Path    string `json:"path"`    // 工具路径，空表示未找到
	Version string `json:"version"` // 工具版本号，空表示未解析
}

// ToolchainReport 汇总所有工具链的检测结果。
type ToolchainReport struct {
	Go      ToolchainInfo `json:"go"`
	CGO     ToolchainInfo `json:"cgo"`     // C 编译器（gcc 或 clang，cgo 用）
	Windres ToolchainInfo `json:"windres"` // Windows 资源编译器（生成 .syso）
	NPM     ToolchainInfo `json:"npm"`     // Node 包管理器（前端构建）
	Garble  ToolchainInfo `json:"garble"`  // Go 源码混淆工具（v0.8.0 替代 UPX）
	Rsrc    ToolchainInfo `json:"rsrc"`    // Go 原生 syso 生成工具（回退方案）
	Wails3  ToolchainInfo `json:"wails3"`  // Wails v3 CLI
}

// commonGoPaths 是 Go 编译器在 Windows 上的常见安装路径。
var commonGoPaths = []string{
	`C:\Program Files\Go\bin\go.exe`,
	`C:\Program Files (x86)\Go\bin\go.exe`,
	`C:\Go\bin\go.exe`,
}

// commonMinGWPaths 是 MinGW/gcc/windres 在 Windows 上的常见安装路径前缀。
var commonMinGWPaths = []string{
	`C:\MinGW\bin`,
	`C:\mingw64\bin`,
	`C:\msys64\mingw64\bin`,
	`C:\msys64\ucrt64\bin`,
	`C:\TDM-GCC-64\bin`,
}

// commonLLVMPaths 是 LLVM/clang 在 Windows 上的常见安装路径前缀。
var commonLLVMPaths = []string{
	`C:\Program Files\LLVM\bin`,
	`C:\Program Files (x86)\LLVM\bin`,
}

// findGoInCommonPaths 在 Go 编译器不在 PATH 时，扫描常见安装路径。
// 仅 Windows 生效；非 Windows 直接返回空串（依赖 PATH）。
func findGoInCommonPaths() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, p := range commonGoPaths {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// findGCC 查找 GCC 编译器（PATH + MinGW 常见路径）。
func findGCC() string {
	// 1. PATH 查找
	candidates := []string{"gcc"}
	if runtime.GOOS == "windows" {
		candidates = []string{"gcc.exe", "gcc"}
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	// 2. MinGW 常见路径
	if runtime.GOOS == "windows" {
		for _, dir := range commonMinGWPaths {
			candidate := filepath.Join(dir, "gcc.exe")
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// findClang 查找 Clang 编译器（PATH + LLVM 常见路径）。
func findClang() string {
	candidates := []string{"clang"}
	if runtime.GOOS == "windows" {
		candidates = []string{"clang.exe", "clang"}
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if runtime.GOOS == "windows" {
		for _, dir := range commonLLVMPaths {
			candidate := filepath.Join(dir, "clang.exe")
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// findWindres 查找 windres 资源编译器（PATH + MinGW 常见路径）。
// 用于 generateSyso 的第三方案回退。
func findWindres() string {
	candidates := []string{"windres"}
	if runtime.GOOS == "windows" {
		candidates = []string{"windres.exe", "windres"}
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if runtime.GOOS == "windows" {
		for _, dir := range commonMinGWPaths {
			candidate := filepath.Join(dir, "windres.exe")
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// detectVersion 执行 `<path> <args...>` 并从输出中解析版本号。
// 版本号正则：匹配形如 "x.y.z" 的数字串（含可选前缀如 "gcc (..."）。
// 解析失败时返回空串（不影响主流程）。
func detectVersion(path string, args ...string) string {
	if path == "" {
		return ""
	}
	cmd := exec.Command(path, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	// 匹配版本号：major.minor[.patch][.build]，至少两段
	re := regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)
	if m := re.FindStringSubmatch(string(out)); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// detectGoVersion 执行 `go version` 并返回版本号（如 "go1.23.4"）。
func detectGoVersion(goPath string) string {
	if goPath == "" {
		return ""
	}
	cmd := exec.Command(goPath, "version")
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	// go version 输出格式：go version go1.23.4 windows/amd64
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) >= 3 {
		return fields[2]
	}
	return strings.TrimSpace(string(out))
}

// DetectToolchains 综合检测所有工具链，返回结构化报告。
// 该函数不返回 error——任何单项失败都通过 Path="" 反映，便于上层逐项展示。
// 调用方：HealthCheck（启动探活）、未来多编译器选择 UI。
func DetectToolchains() ToolchainReport {
	// Go 编译器：优先 goBinary（可能通过 SetGoBinary 配置），其次 PATH，最后常见安装路径
	goPath := goBinary
	if goPath == "go" {
		if p, err := exec.LookPath("go"); err == nil {
			goPath = p
		} else if p := findGoInCommonPaths(); p != "" {
			goPath = p
		}
	}
	goInfo := ToolchainInfo{Name: "go", Path: ""}
	if goPath != "go" {
		goInfo.Path = goPath
		goInfo.Version = detectGoVersion(goPath)
	}

	// C 编译器：gcc 优先，clang 回退（cgo 用）
	cgoPath := findGCC()
	cgoName := "gcc"
	if cgoPath == "" {
		cgoPath = findClang()
		cgoName = "clang"
	}
	cgoInfo := ToolchainInfo{Name: cgoName, Path: cgoPath}
	if cgoPath != "" {
		cgoInfo.Version = detectVersion(cgoPath, "--version")
	}

	// windres
	windresPath := findWindres()
	windresInfo := ToolchainInfo{Name: "windres", Path: windresPath}
	if windresPath != "" {
		windresInfo.Version = detectVersion(windresPath, "--version")
	}

	// npm
	npmPath := findNPM()
	npmInfo := ToolchainInfo{Name: "npm", Path: npmPath}
	if npmPath != "" {
		npmInfo.Version = detectVersion(npmPath, "--version")
	}

	// garble（Go 源码混淆工具，v0.8.0 替代 UPX）
	garblePath := findGarble()
	garbleInfo := ToolchainInfo{Name: "garble", Path: garblePath}
	if garblePath != "" {
		garbleInfo.Version = detectVersion(garblePath, "version")
	}

	// rsrc
	rsrcPath := findRsrcCli()
	rsrcInfo := ToolchainInfo{Name: "rsrc", Path: rsrcPath}
	if rsrcPath != "" {
		rsrcInfo.Version = detectVersion(rsrcPath, "-v")
	}

	// wails3
	wails3Path := findWails3Cli()
	wails3Info := ToolchainInfo{Name: "wails3", Path: wails3Path}
	if wails3Path != "" {
		wails3Info.Version = detectVersion(wails3Path, "version")
	}

	return ToolchainReport{
		Go:      goInfo,
		CGO:     cgoInfo,
		Windres: windresInfo,
		NPM:     npmInfo,
		Garble:  garbleInfo,
		Rsrc:    rsrcInfo,
		Wails3:  wails3Info,
	}
}
