// Package runner 提供 .eg 源码的运行与构建能力，供 HTTP server 和 Wails IDE 共用。
package runner

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"egou/internal/transpiler"
	"egou/internal/types"
)

// wails3Cli 指向 Wails v3 CLI 可执行文件。
var wails3Cli = os.Getenv("WAILS3")

// goBinary 指向 Go 编译器路径。
// 初始化时自动检测 exe 同级 go/bin/go.exe（内置 Go SDK），找不到则回退 "go"（依赖 PATH）。
// 通过 SetGoBinary 可在运行时切换到用户指定的 Go SDK 路径。
var goBinary = detectBundledGo()

// goRoot 记录内置 Go SDK 的 GOROOT 路径（exe 同级 go/），编译时注入环境变量。
var goRoot string

// templateDir 指向外置 wails-template 目录的绝对路径（exe 同级 wails-template/）。
// 由 IDE 启动时通过 SetTemplateDir 注入，未注入则从 exe 同级目录查找。
var templateDir string

// detectBundledGo 检测 exe 同级 go/bin/go.exe（内置 Go SDK）。
// 找到则返回完整路径并记录 GOROOT；找不到返回 "go"（回退 PATH）。
func detectBundledGo() string {
	exePath, err := os.Executable()
	if err != nil {
		return "go"
	}
	exeDir := filepath.Dir(exePath)
	if runtime.GOOS == "windows" {
		bundled := filepath.Join(exeDir, "go", "bin", "go.exe")
		if _, err := os.Stat(bundled); err == nil {
			goRoot = filepath.Join(exeDir, "go")
			return bundled
		}
	} else {
		bundled := filepath.Join(exeDir, "go", "bin", "go")
		if _, err := os.Stat(bundled); err == nil {
			goRoot = filepath.Join(exeDir, "go")
			return bundled
		}
	}
	return "go"
}

// buildGoEnv 构造 go 子进程的环境变量。
// 关键修复（v0.11.23）：强制使用内置 Go SDK 的环境，彻底隔离用户系统 Go。
//
// 历史问题：
//   - v0.11.8 内置 Go SDK 后，只 append GOROOT 导致子进程收到两个 GOROOT
//   - v0.11.10 剔除 os.Environ() 中的 GOROOT 解决了双 GOROOT 问题
//   - 但 PATH 中仍可能包含用户系统的旧 Go bin 目录（如 C:\Program Files\Go\bin），
//     go.exe 执行 `go tool compile` 时通过 PATH 查找 compile.exe，会用旧版工具，
//     导致 "version go1.25.12 does not match go tool version go1.26.4" 错误
//
// 最终方案：当使用内置 SDK 时，剔除 PATH 中的其他 Go 相关路径，并把内置 SDK 的
// bin 目录前置到 PATH，确保 go tool 命令只找内置 SDK 的工具。
//
// v0.11.27 修复：同时将 exe 同级 tools/ 目录加入 PATH（wails3 等工具查找），
// 确保在无 Go 系统安装的环境中也能正常运行。
func buildGoEnv() []string {
	env := os.Environ()
	if goRoot == "" {
		return env
	}
	bundledBin := filepath.Join(goRoot, "bin")
	toolsDir := ""
	if exePath, err := os.Executable(); err == nil {
		toolsDir = filepath.Join(filepath.Dir(exePath), "tools")
	}
	filtered := make([]string, 0, len(env)+4)
	foundPath := false
	for _, e := range env {
		if strings.HasPrefix(e, "GOROOT=") {
			continue
		}
		if strings.HasPrefix(e, "GOBIN=") ||
			strings.HasPrefix(e, "GOFLAGS=") ||
			strings.HasPrefix(e, "GOPROXY=") ||
			strings.HasPrefix(e, "GOSUMDB=") ||
			strings.HasPrefix(e, "GONOSUMCHECK=") ||
			strings.HasPrefix(e, "GONOSUMDB=") ||
			strings.HasPrefix(e, "GOWORK=") ||
			strings.HasPrefix(e, "GOTOOLCHAIN=") ||
			strings.HasPrefix(e, "CGO_ENABLED=") {
			continue
		}
		if strings.HasPrefix(e, "PATH=") {
			foundPath = true
			pathVal := strings.TrimPrefix(e, "PATH=")
			paths := filepath.SplitList(pathVal)
			cleanPaths := make([]string, 0, len(paths)+2)
			for _, p := range paths {
				lower := strings.ToLower(p)
				if isSystemGoBin(lower) {
					continue
				}
				cleanPaths = append(cleanPaths, p)
			}
			// 前置顺序：go/bin（go 命令最优先）→ tools/（wails3）→ 系统 PATH
			prepend := []string{bundledBin}
			if toolsDir != "" {
				prepend = append(prepend, toolsDir)
			}
			cleanPaths = append(prepend, cleanPaths...)
			e = "PATH=" + strings.Join(cleanPaths, string(filepath.ListSeparator))
		}
		filtered = append(filtered, e)
	}
	// 如果环境中没有 PATH（极罕见），手动构造
	if !foundPath {
		pathParts := []string{}
		pathParts = append(pathParts, bundledBin)
		if toolsDir != "" {
			pathParts = append(pathParts, toolsDir)
		}
		pathParts = append(pathParts, `C:\Windows\System32`, `C:\Windows`)
		filtered = append(filtered, "PATH="+strings.Join(pathParts, string(filepath.ListSeparator)))
	}
	filtered = append(filtered, "GOROOT="+goRoot)
	// GOTOOLCHAIN=local：禁止自动下载新版工具链（离线环境必须），也避免 go.env 中 CRLF 导致的 "auto\r" 错误
	filtered = append(filtered, "GOTOOLCHAIN=local")
	return filtered
}

// isSystemGoBin 判断一个路径是否是用户系统安装的 Go 的 bin 目录。
// 用于从 PATH 中剔除用户系统的 Go，避免与内置 SDK 冲突。
func isSystemGoBin(lowerPath string) bool {
	// 常见安装路径
	systemGoBins := []string{
		`c:\program files\go\bin`,
		`c:\program files (x86)\go\bin`,
		`c:\go\bin`,
		`c:\go\1.25.12\bin`, // 用户的具体路径
	}
	for _, sgb := range systemGoBins {
		if lowerPath == sgb {
			return true
		}
	}
	// 通配匹配：任何包含 \go\bin 或 /go/bin 的路径（排除内置 SDK 路径，内置 SDK 由 detectBundledGo 处理）
	// 注意：内置 SDK 的路径是 <exeDir>/go/bin，不应被剔除。但 buildGoEnv 调用前 goRoot 已设置，
	// 内置 SDK bin 会作为前置加入，这里剔除的是用户系统的 Go bin。
	if strings.Contains(lowerPath, `\go\bin`) || strings.Contains(lowerPath, `/go/bin`) {
		// 但要排除内置 SDK 自己的路径（通过 goRoot 判断）
		if goRoot != "" {
			bundledBinLower := strings.ToLower(filepath.Join(goRoot, "bin"))
			if lowerPath == bundledBinLower {
				return false
			}
		}
		return true
	}
	return false
}

// SetGoBinary 设置 Go 编译器路径（如 C:\Program Files\Go\bin\go.exe）。
// 传入空字符串恢复为内置 Go SDK 或 "go"（从 PATH 查找）。
// 自动推断 GOROOT（go.exe 的上级目录的上级目录）。
//
// v0.11.23 修订：按用户要求，优先使用内置 Go SDK，用户指定的路径仍支持，
// 但内置 SDK 存在时默认用内置 SDK（detectBundledGo 已在包初始化时调用）。
func SetGoBinary(path string) {
	if path != "" {
		goBinary = path
		// 推断 GOROOT：goBinary = <GOROOT>/bin/go.exe
		goRoot = filepath.Dir(filepath.Dir(path))
	} else {
		// 恢复为内置 Go SDK 或 PATH
		goBinary = detectBundledGo()
	}
}

// GetGoBinary 返回当前配置的 Go 编译器路径。
func GetGoBinary() string {
	return goBinary
}

// SetTemplateDir 设置外置 wails-template 目录路径（exe 同级 wails-template/）。
// 编译用户程序时从该目录复制模板到临时构建目录，替代旧的 go:embed 嵌入方式。
func SetTemplateDir(dir string) {
	templateDir = dir
}

// transpileCache 缓存源码哈希 → 转译后的 Go 代码，实现增量编译。
// 当合并后的 .eg 源码哈希与缓存匹配时，跳过 transpiler.Transpile 直接使用缓存的 Go 代码。
// go build 本身有缓存，所以跳过 transpile 后整个编译流程会显著加速。
// H4：使用 LRU 淘汰策略（容量 256），避免长会话下不同历史版本源码缓存无限增长导致内存膨胀。
var transpileCache = newLRUCache(256)

// hashSource 计算源码的 SHA256 哈希，返回十六进制字符串。
func hashSource(src string) string {
	h := sha256.Sum256([]byte(src))
	return hex.EncodeToString(h[:])
}

// P2-19：跨层共享类型集中到 egou/internal/types，runner 包内通过类型别名引用，
// 包内代码无需改动即可继续使用 Event/EventSink/CompileError/HealthReport。
type Event = types.Event
type EventSink = types.EventSink
type CompileError = types.CompileError

// RunSource 把 .eg 源码转译后运行，返回标准输出。
// projectPath 用于加载窗口设计文件（.ew），为空时不影响普通代码运行。
// sink 在转译、构建、运行各阶段被调用，sink 为 nil 时静默忽略。
func RunSource(src string, projectPath string, sink EventSink) (string, error) {
	tmpDir, err := prepareRuntimeBuild(src, projectPath, sink)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	outFile := filepath.Join(tmpDir, "egruntime.exe")
	// RunSource 用 debug 模式构建（保留符号表，便于调试）
	if err := buildRuntime(tmpDir, outFile, sink, false, readProjectVersion(projectPath)); err != nil {
		return "", err
	}

	return runRuntime(tmpDir, outFile, projectPath, sink)
}

// BuildSource 把 .eg 源码转译后构建成可执行文件，返回结果文本。
// projectPath 用于加载窗口设计文件（.ew），为空时不影响普通代码运行。
// sink 在转译、构建各阶段被调用，sink 为 nil 时静默忽略。
// 默认 debug 模式；如需 release（-s -w 去除调试信息），用 BuildSourceRelease。
func BuildSource(src string, projectPath string, sink EventSink) (string, error) {
	return buildSourceEx(src, projectPath, sink, false)
}

// BuildSourceRelease 与 BuildSource 相同，但启用 release 模式（-ldflags "-s -w"），
// 去除调试符号和 DWARF 信息，生成的可执行文件体积更小。
func BuildSourceRelease(src string, projectPath string, sink EventSink) (string, error) {
	return buildSourceEx(src, projectPath, sink, true)
}

// BuildForDebug 编译源码到临时目录的二进制文件（debug 模式，保留 DWARF 调试符号），
// 返回二进制路径和临时目录路径。调用方负责清理临时目录（defer os.RemoveAll(tmpDir)）。
// 用于调试器：编译完成后启动 dlv 附加到返回的二进制。
//
// 与 BuildSource/RunSource 的区别：
//   - 不运行二进制（由 dlv 启动）
//   - 不清理临时目录（dlv 需要读取二进制，调试结束后才清理）
//   - 强制 debug 模式（release=false），保留 DWARF 供 dlv 使用
func BuildForDebug(src, projectPath string, sink EventSink) (binaryPath, tmpDir string, err error) {
	tmpDir, err = prepareRuntimeBuild(src, projectPath, sink)
	if err != nil {
		return "", "", err
	}
	outFile := filepath.Join(tmpDir, "egruntime-debug.exe")
	if err := buildRuntime(tmpDir, outFile, sink, false, readProjectVersion(projectPath)); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", err
	}
	return outFile, tmpDir, nil
}

func buildSourceEx(src string, projectPath string, sink EventSink, release bool) (string, error) {
	if projectPath == "" {
		return "", fmt.Errorf("未打开项目，无法编译（产物必须输出到项目目录，避免污染 IDE 目录）")
	}
	tmpDir, err := prepareRuntimeBuild(src, projectPath, sink)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	outFile := filepath.Join(tmpDir, "egruntime-build.exe")
	version := readProjectVersion(projectPath)
	if err := buildRuntime(tmpDir, outFile, sink, release, version); err != nil {
		return "", err
	}

	// P2：产物输出目录优先用 project.eg.json 的 output 字段（相对项目根），
	// 字段缺失时默认 "bin"。这样产物不会污染项目根目录，且用户可自定义输出位置。
	// projectPath 为空已在函数入口拦截，不会回退到 os.Getwd()（避免污染 IDE 目录）。
	destDir := projectPath
	if outRel := readProjectOutput(projectPath); outRel != "" && outRel != "." {
		destDir = filepath.Join(destDir, outRel)
	}
	// 文件名格式：egruntime[-release][-v版本号].exe
	// release 模式且有版本号时：egruntime-v1.0.1-release.exe
	// release 模式无版本号时：egruntime-release.exe
	// debug 模式：egruntime.exe（避免开发时文件名频繁变化）
	nameParts := []string{"egruntime"}
	if release {
		if version != "" {
			nameParts = append(nameParts, "-v"+version)
		}
		nameParts = append(nameParts, "-release")
	}
	destFile := filepath.Join(destDir, strings.Join(nameParts, "")+".exe")
	// P2-14：二次校验可执行文件路径必须在项目目录内，
	// 防止用户配置 output 字段指向系统目录（如 C:\Windows）导致编译产物覆盖系统文件。
	if err := validateOutputInProject(destFile, projectPath); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}
	input, err := os.ReadFile(outFile)
	if err != nil {
		return "", fmt.Errorf("读取生成文件失败: %w", err)
	}
	if err := os.WriteFile(destFile, input, 0755); err != nil {
		return "", fmt.Errorf("复制可执行文件失败: %w", err)
	}
	mode := "debug"
	if release {
		mode = "release"
	}
	if release {
		// release 构建成功后递增 patch 版本号，写回 project.eg.json
		// 失败不阻断，仅 emit 警告。让下次构建版本号自动 +1。
		if newVer, err := incrementProjectVersion(projectPath); err == nil && newVer != "" {
			emit(sink, "build", "版本号已自增至 "+newVer, false)
		} else if err != nil {
			emit(sink, "build", "版本号自增跳过: "+err.Error(), false)
		}
	}
	// 复制原生库到产物目录（让 release exe 能加载 DLL，G3+P3）
	// 来源与运行时一致：全局 native/ + 项目 native/（项目级覆盖全局同名）
	if n, err := copyNativeLibsToDest(destDir, projectPath); err != nil {
		emit(sink, "build", "复制原生库到产物目录跳过: "+err.Error(), false)
	} else if n > 0 {
		emit(sink, "build", fmt.Sprintf("已复制 %d 个原生库到产物目录", n), false)
	}
	// 计算构建产物 SHA256 校验和，便于用户验证文件完整性
	if hash, err := fileSHA256(destFile); err == nil {
		emit(sink, "build", "SHA256: "+hash, false)
	}
	// emit 产物路径，让前端能调用签名检查等后端服务
	emit(sink, "artifact", destFile, false)
	emitProgress(sink, "done", 100)
	return fmt.Sprintf("构建成功（%s）: %s", mode, destFile), nil
}

// fileSHA256 计算指定文件的 SHA256 哈希，返回十六进制字符串。
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// incrementProjectVersion 读取 project.eg.json 的 version 字段，把 patch 段 +1 后写回。
// 版本号格式：major.minor.patch（语义化版本），缺失的段补 0。
// 例如：1.0.0 → 1.0.1，1.2 → 1.2.1，无 version 字段时不处理。
func incrementProjectVersion(projectPath string) (string, error) {
	if projectPath == "" {
		return "", fmt.Errorf("项目路径为空")
	}
	configPath := filepath.Join(projectPath, "project.eg.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", err
	}
	ver, _ := raw["version"].(string)
	if ver == "" {
		return "", fmt.Errorf("项目无 version 字段")
	}
	newVer, err := bumpPatch(ver)
	if err != nil {
		return "", err
	}
	raw["version"] = newVer
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return "", err
	}
	// 保留 JSON 末尾换行
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return "", err
	}
	return newVer, nil
}

// bumpPatch 把 "major.minor.patch" 的 patch 段 +1，返回新版本号字符串。
// 非法段视为 0 处理。例如：1.2.3 → 1.2.4，1.2 → 1.2.1，1.2.0-dev → 1.2.1（忽略后缀）。
func bumpPatch(ver string) (string, error) {
	// 去除可能的后缀（如 1.0.0-dev → 1.0.0）
	main := ver
	if idx := strings.IndexAny(ver, "-+"); idx >= 0 {
		main = ver[:idx]
	}
	parts := strings.Split(main, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("版本号格式无效: %s", ver)
	}
	// 补齐到 3 段
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	patch, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		// 非数字段视为 0
		if err1 != nil {
			major = 0
		}
		if err2 != nil {
			minor = 0
		}
		if err3 != nil {
			patch = 0
		}
	}
	patch++
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

func emit(sink EventSink, stage, output string, isOutput bool) {
	if sink == nil {
		return
	}
	sink(Event{Stage: stage, Output: output, IsOutput: isOutput})
}

// emitProgress 发送编译进度事件（stage="progress"），output 格式为 "step:percent"。
// step 取值：prepare/transpile/ready/build/link/run/done，前端据此驱动步骤式进度条。
func emitProgress(sink EventSink, step string, percent int) {
	if sink == nil {
		return
	}
	sink(Event{Stage: "progress", Output: fmt.Sprintf("%s:%d", step, percent), IsOutput: false})
}

// stripLibEntryDeclarations 剥离附加 .eg 中的入口声明，避免与主源码冲突：
//   - `# 程序集 ...` 头部（避免出现两个 package）
//   - 顶层 `导入 ()` 整段（由主入口统一提供）
//   - 顶层 `函数 主函数() … 结束函数`（避免两个 mainImpl）
//
// 普通函数（事件处理函数 / 辅助函数）会原样保留。
func stripLibEntryDeclarations(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	// 状态机：0=正常  -1=正在跳过 导入 块  1=正在跳过 主函数 块
	skip := 0
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if skip == 0 {
			if strings.HasPrefix(trim, "#") && strings.Contains(trim, "程序集") {
				continue
			}
			if trim == "导入 (" {
				skip = -1
				continue
			}
			if trim == "函数 主函数()" || trim == "主函数()" {
				skip = 1
				continue
			}
			out = append(out, line)
			continue
		}
		if skip == -1 {
			if trim == ")" {
				skip = 0
			}
			continue
		}
		if skip == 1 {
			if trim == "结束函数" {
				skip = 0
			}
			continue
		}
	}
	return strings.Join(out, "\n")
}

// mergeProjectLibs 合并项目级和全局 .elib 扩展包到主源码前面。
// 顺序：全局 libs（exe 同级）→ 项目 libs（<项目>/libs/）→ 主源码。
// 项目级优先级更高（后注册的别名覆盖全局同名别名）。
// 每个 .elib 的入口声明（# 程序集、导入、主函数）会被自动剥离，避免与主源码冲突。
// 同时解析 commands.json 提取中文别名 → 英文键映射，注册到 transpiler。
func mergeProjectLibs(src, projectPath string) (string, error) {
	var b strings.Builder
	extraAliases := map[string]string{}
	b.WriteString("// ===== 扩展包自动合并（EGOU 编译器生成，不要手改）=====\n")

	// 1. 全局 libs（exe 同级 libs/，所有项目共享）
	if globalDir, err := globalLibsDir(); err == nil && globalDir != "" {
		if err := mergeLibsFromDir(&b, extraAliases, globalDir, "global"); err != nil {
			return "", err
		}
	}

	// 2. 项目 libs（<项目>/libs/）
	if projectPath != "" {
		projLibsDir := filepath.Join(projectPath, "libs")
		if err := mergeLibsFromDir(&b, extraAliases, projLibsDir, "project"); err != nil {
			return "", err
		}
	}

	// 没有任何扩展包时直接返回原源码
	if extraAliases == nil || len(extraAliases) == 0 {
		// 检查 b 是否只写了头部注释
		header := "// ===== 扩展包自动合并（EGOU 编译器生成，不要手改）=====\n"
		if b.String() == header {
			return src, nil
		}
	}

	b.WriteString("\n")
	b.WriteString(src)
	// 注册 .elib 命令别名到 transpiler，让中文别名调用能被替换为英文键
	if len(extraAliases) > 0 {
		transpiler.RegisterExtraAliases(extraAliases)
	}
	return b.String(), nil
}

// globalLibsDir 返回 exe 同级的 libs 目录路径（IDE 全局生态目录）。
// 目录不存在时返回空字符串。
func globalLibsDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(exePath), "libs")
	if _, err := os.Stat(dir); err != nil {
		return "", nil
	}
	return dir, nil
}

// mergeLibsFromDir 扫描 libsDir 下的 .elib 扩展包，合并 source.eg 到 b，
// 解析 commands.json 的中文别名 → 英文键映射到 extraAliases。
// originTag 用于 #@eg-file 标记（"global" 或 "project"），便于调试定位。
func mergeLibsFromDir(b *strings.Builder, extraAliases map[string]string, libsDir, originTag string) error {
	entries, err := os.ReadDir(libsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取 %s libs 目录失败: %w", originTag, err)
	}
	var pkgDirs []string
	for _, e := range entries {
		if e.IsDir() {
			pkgDirs = append(pkgDirs, e.Name())
		}
	}
	if len(pkgDirs) == 0 {
		return nil
	}
	sort.Strings(pkgDirs)
	for _, pkg := range pkgDirs {
		cmdsFile := filepath.Join(libsDir, pkg, "commands.json")
		cmdsData, err := os.ReadFile(cmdsFile)
		if err != nil {
			// 没 commands.json 视为非 .elib 目录，跳过
			continue
		}
		// 解析 commands.json 提取 displayName → englishName 映射
		var cmdsParsed struct {
			Commands []struct {
				DisplayName string `json:"displayName"`
				EnglishName string `json:"englishName"`
			} `json:"commands"`
		}
		if err := json.Unmarshal(cmdsData, &cmdsParsed); err == nil {
			for _, c := range cmdsParsed.Commands {
				if c.DisplayName != "" && c.EnglishName != "" {
					extraAliases[c.DisplayName] = c.EnglishName
				}
			}
		}
		srcFile := filepath.Join(libsDir, pkg, "source.eg")
		data, err := os.ReadFile(srcFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("读取 %s 失败: %w", srcFile, err)
		}
		cleaned := stripLibEntryDeclarations(string(data))
		b.WriteString("\n#@eg-file ")
		b.WriteString(originTag)
		b.WriteString(":libs/")
		b.WriteString(pkg)
		b.WriteString("/source.eg\n")
		b.WriteString(cleaned)
		if len(cleaned) > 0 && cleaned[len(cleaned)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	return nil
}

// prepareRuntimeBuild 将运行时模板复制到临时目录，并注入转译后的用户代码。
// 并行优化：转译（路线A）与模板复制（路线B）并行执行；
// 用户代码写入、资源嵌入、前端资源准备三者也并行执行。
func prepareRuntimeBuild(src, projectPath string, sink EventSink) (string, error) {
	emitProgress(sink, "prepare", 5)
	emit(sink, "transpile", "开始转译 .eg 源码...", false)

	// ===== 第一阶段：路线A（转译）与路线B（模板复制）并行 =====

	// 路线A：mergeProjectLibs → Transpile（不依赖 tmpDir）
	type transpileResult struct {
		goSrc string
		err   error
	}
	transpileCh := make(chan transpileResult, 1)
	go func() {
		merged, err := mergeProjectLibs(src, projectPath)
		if err != nil {
			emit(sink, "transpile", "合并扩展包失败: "+err.Error(), false)
			transpileCh <- transpileResult{err: err}
			return
		}
		// 增量编译：计算源码哈希，命中缓存则跳过 transpile
		hash := hashSource(merged)
		cached, cacheHit := transpileCache.get(hash)
		if cacheHit {
			// 缓存命中：清理别名后直接返回缓存的 Go 代码
			transpiler.ClearExtraAliases()
			transpiler.ClearExternalCreates()
			transpiler.ClearExternalEventSuffixes()
			emit(sink, "transpile", "源码未变更，跳过转译（命中缓存）", false)
			transpileCh <- transpileResult{goSrc: cached}
			return
		}
		// 注册外置组件创建命令：扫描 IDE components/ 目录，把带 runtime 字段的组件
		// 注册为 "创建<label>" → CreateComponent("type", ...) 命令映射。
		registerExternalComponents()
		goSrc, err := transpiler.Transpile(merged)
		// 无论 Transpile 是否成功，都清理别名和外置命令，避免跨项目污染
		transpiler.ClearExtraAliases()
		transpiler.ClearExternalCreates()
		transpiler.ClearExternalEventSuffixes()
		if err != nil {
			emit(sink, "transpile", "转译失败: "+err.Error(), false)
			transpileCh <- transpileResult{err: err}
			return
		}
		// 更新缓存
		transpileCache.set(hash, goSrc)
		emit(sink, "transpile", "转译成功", false)
		transpileCh <- transpileResult{goSrc: goSrc}
	}()

	// 路线B：extractTemplate → MkdirTemp（从 go:embed 提取，不依赖 IDE 源码树）
	type templateResult struct {
		tmpDir string
		err    error
	}
	templateCh := make(chan templateResult, 1)
	go func() {
		emit(sink, "stage", "准备运行时模板...", false)
		tmpDir, err := os.MkdirTemp("", "egruntime-*")
		if err != nil {
			templateCh <- templateResult{err: fmt.Errorf("创建临时目录失败: %w", err)}
			return
		}
		if err := extractTemplate(tmpDir); err != nil {
			os.RemoveAll(tmpDir)
			templateCh <- templateResult{err: fmt.Errorf("提取运行时模板失败: %w", err)}
			return
		}
		// 生成 Windows .syso 资源文件（嵌入图标和清单），失败不阻断构建
		if err := generateSyso(tmpDir, projectPath, sink); err != nil {
			emit(sink, "stage", "图标嵌入跳过: "+err.Error(), false)
		}
		templateCh <- templateResult{tmpDir: tmpDir}
	}()

	// 等待路线A完成
	tRes := <-transpileCh
	if tRes.err != nil {
		// 路线A失败时，异步等待路线B并清理其 tmpDir
		go func() {
			if bRes := <-templateCh; bRes.tmpDir != "" {
				os.RemoveAll(bRes.tmpDir)
			}
		}()
		return "", tRes.err
	}
	goSrc := tRes.goSrc
	emitProgress(sink, "transpile", 25)
	emit(sink, "transpile", "转译成功", false)

	// 等待路线B完成
	bRes := <-templateCh
	if bRes.err != nil {
		return "", bRes.err
	}
	tmpDir := bRes.tmpDir

	// ===== 第二阶段：四个写入操作并行 =====
	// a. 写入用户代码（依赖 goSrc + tmpDir）
	// b. 嵌入 .ew 资源（依赖 tmpDir + projectPath）
	// c. 准备前端资源（依赖 tmpDir）
	// d. 复制原生库 DLL（G3 全局 + P3 项目级，依赖 tmpDir + projectPath）
	errCh := make(chan error, 4)

	go func() {
		userCodeFile := filepath.Join(tmpDir, "usercode.go")
		if err := os.WriteFile(userCodeFile, []byte(goSrc), 0644); err != nil {
			errCh <- fmt.Errorf("写入用户代码失败: %w", err)
			return
		}
		errCh <- nil
	}()

	go func() {
		if projectPath != "" {
			if err := writeEmbeddedAssets(tmpDir, projectPath, sink); err != nil {
				errCh <- fmt.Errorf("嵌入资源失败: %w", err)
				return
			}
		}
		errCh <- nil
	}()

	go func() {
		if err := ensureFrontendDist(tmpDir, sink); err != nil {
			errCh <- fmt.Errorf("准备前端资源失败: %w", err)
			return
		}
		errCh <- nil
	}()

	go func() {
		if err := copyNativeLibs(tmpDir, projectPath, sink); err != nil {
			errCh <- fmt.Errorf("复制原生库失败: %w", err)
			return
		}
		errCh <- nil
	}()

	// 等待四个操作全部完成
	for i := 0; i < 4; i++ {
		if err := <-errCh; err != nil {
			os.RemoveAll(tmpDir)
			return "", err
		}
	}

	// G4：扫描已复制的 .lib/.a 静态库，生成 cgo 链接文件。
	// 只有检测到静态库时才生成 cgo_link.go 并启用 CGO，否则保持纯 Go 编译不受影响。
	if err := writeCgoLinkFile(tmpDir, sink); err != nil {
		emit(sink, "stage", "静态库链接跳过: "+err.Error(), false)
	}

	emit(sink, "stage", "运行时准备就绪", false)
	emitProgress(sink, "ready", 40)
	return tmpDir, nil
}

// writeCgoLinkFile 扫描 tmpDir 下的 .lib/.a 静态库文件，生成 cgo_link.go。
// 生成的文件包含 // #cgo LDFLAGS: 指令，让 go build 链接这些库。
// 没有静态库时不生成文件，保持纯 Go 编译。
// 注意：用户需在 .elib 中通过 cgo 声明对应的 C 函数才能实际调用库中符号。
func writeCgoLinkFile(tmpDir string, sink EventSink) error {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	var staticLibs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".lib" || ext == ".a" {
			staticLibs = append(staticLibs, e.Name())
		}
	}
	if len(staticLibs) == 0 {
		return nil
	}
	// 生成 cgo_link.go：用文件名直接传递给链接器
	// #cgo LDFLAGS: foo.lib bar.a
	// 同时保留一个空 C 函数避免 cgo 报错"no C source files"
	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString("/*\n")
	b.WriteString("#cgo LDFLAGS: ")
	b.WriteString(strings.Join(staticLibs, " "))
	b.WriteString("\n*/\n")
	b.WriteString("import \"C\"\n\n")
	// 引用一个 C 符号避免链接器完全优化掉（无实际副作用）
	b.WriteString("// _nxg_keep_static_libs 引用占位，避免 cgo 报错\n")
	b.WriteString("var _ = C.int(0)\n")
	cgoFile := filepath.Join(tmpDir, "cgo_link.go")
	if err := os.WriteFile(cgoFile, []byte(b.String()), 0644); err != nil {
		return err
	}
	emit(sink, "stage", fmt.Sprintf("已生成静态库链接配置（%d 个库）", len(staticLibs)), false)
	return nil
}

// externalComponentConfig 是外置组件运行时配置的精简结构，仅保留运行时渲染所需字段。
// 完整 config.json 由 IDE 端 components.go 的 ComponentDef 解析，这里只提取运行时部分。
type externalComponentConfig struct {
	Type         string            `json:"type"`
	Label        string            `json:"label"`
	HTML         string            `json:"html"`
	Events       map[string]string `json:"events"`       // runtime.events: DOM 事件 → EGOU 事件名
	EventNames   []string          `json:"eventNames"`   // 顶层 events 声明：所有 EGOU 事件名
}

// scanExternalComponents 扫描 IDE components/ 目录下所有组件包，
// 返回带 runtime 字段的组件配置列表（用于嵌入用户程序 + 注册转译命令）。
func scanExternalComponents() []externalComponentConfig {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	componentsRoot := filepath.Join(filepath.Dir(exePath), "components")
	entries, err := os.ReadDir(componentsRoot)
	if err != nil {
		return nil
	}
	var out []externalComponentConfig
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkgComponentsDir := filepath.Join(componentsRoot, e.Name(), "components")
		compEntries, err := os.ReadDir(pkgComponentsDir)
		if err != nil {
			continue
		}
		for _, ce := range compEntries {
			if !ce.IsDir() {
				continue
			}
			configPath := filepath.Join(pkgComponentsDir, ce.Name(), "config.json")
			data, err := os.ReadFile(configPath)
			if err != nil {
				continue
			}
			var raw struct {
				Type    string            `json:"type"`
				Label   string            `json:"label"`
				Events  []string          `json:"events"`
				Runtime *struct {
					HTML   string            `json:"html"`
					Events map[string]string `json:"events"`
				} `json:"runtime"`
			}
			if err := json.Unmarshal(data, &raw); err != nil || raw.Type == "" || raw.Runtime == nil {
				continue
			}
			out = append(out, externalComponentConfig{
				Type:       raw.Type,
				Label:      raw.Label,
				HTML:       raw.Runtime.HTML,
				Events:     raw.Runtime.Events,
				EventNames: raw.Events,
			})
		}
	}
	return out
}

// registerExternalComponents 扫描 IDE 已安装的外置组件包，
// 把带 runtime 字段的组件注册为转译命令："创建<label>" → CreateComponent("type", ...)，
// 同时注册自定义事件后缀（如 "节点被点击"），使事件处理函数能自动注册。
func registerExternalComponents() {
	for _, c := range scanExternalComponents() {
		transpiler.RegisterExternalCreate("创建"+c.Label, c.Type)
		// 注册顶层 events 中不在已知后缀表里的事件名
		for _, evt := range c.EventNames {
			transpiler.RegisterExternalEventSuffix(evt)
		}
	}
}

// writeEmbeddedAssets 扫描项目下的 .ew 窗口设计文件和 assets/ 资源文件，
// 生成 embedded_assets.go，让运行时 LoadWindow 和 载入资源/读资源文本 优先从嵌入数据加载。
// 同时嵌入 IDE 已安装的外置组件运行时配置（runtime 字段），供运行时前端渲染外置组件。
// 这样导出的 exe 是单文件，资源文件无需随 exe 一起分发。
func writeEmbeddedAssets(tmpDir, projectPath string, sink EventSink) error {
	type windowAsset struct {
		name string
		data string
	}
	type fileAsset struct {
		relPath string // 相对 assets/ 的路径，用正斜杠
		data    []byte
	}
	var windows []windowAsset
	var fileAssets []fileAsset

	// L6：补充跳过常见无关目录（IDE 配置、临时目录、编译输出、缓存），
	// 避免 filepath.Walk 遍历大目录树造成不必要的 IO 开销。
	skipDirs := map[string]bool{
		"bin": true, "build": true, "dist": true,
		"node_modules": true, ".git": true, "libs": true,
		"runtime-frontend": true,
		".vscode":          true, ".idea": true, ".vs": true,
		"temp": true, "tmp": true, "cache": true, ".cache": true,
		"obj": true, "target": true, "out": true,
		"coverage": true, ".next": true, ".nuxt": true,
	}
	assetsRoot := filepath.Join(projectPath, "assets")

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[strings.ToLower(info.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		// .ew 窗口文件
		if filepath.Ext(path) == ".ew" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			windows = append(windows, windowAsset{name: name, data: string(data)})
			return nil
		}
		// assets/ 目录下的所有文件
		if strings.HasPrefix(strings.ToLower(path), strings.ToLower(assetsRoot)+string(filepath.Separator)) {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			rel, err := filepath.Rel(assetsRoot, path)
			if err != nil {
				return nil
			}
			// 统一用正斜杠，跨平台一致
			rel = filepath.ToSlash(rel)
			fileAssets = append(fileAssets, fileAsset{relPath: rel, data: data})
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(windows) > 0 {
		emit(sink, "stage", fmt.Sprintf("嵌入 %d 个窗口设计文件", len(windows)), false)
	}
	if len(fileAssets) > 0 {
		emit(sink, "stage", fmt.Sprintf("嵌入 %d 个资源文件", len(fileAssets)), false)
	}
	// 扫描 IDE 外置组件，嵌入带 runtime 字段的组件配置
	externalComps := scanExternalComponents()
	if len(externalComps) > 0 {
		emit(sink, "stage", fmt.Sprintf("嵌入 %d 个外置组件配置", len(externalComps)), false)
	}

	var b strings.Builder
	b.WriteString("// Code generated by EGOU runner. DO NOT EDIT.\n")
	b.WriteString("package main\n\n")
	// 窗口设计文件
	b.WriteString("// embeddedWindows 存储项目 .ew 窗口设计文件的原始 JSON 内容。\n")
	b.WriteString("// LoadWindow 优先从这里查找，找不到再回退到文件系统。\n")
	b.WriteString("var embeddedWindows = map[string]string{\n")
	for _, a := range windows {
		b.WriteString("\t" + strconv.Quote(a.name) + ": " + strconv.Quote(a.data) + ",\n")
	}
	b.WriteString("}\n\n")
	// 资源文件
	b.WriteString("// embeddedFiles 存储项目 assets/ 目录下所有文件的原始字节。\n")
	b.WriteString("// 载入资源/读资源文本 从这里查找，找不到返回错误。\n")
	b.WriteString("var embeddedFiles = map[string][]byte{\n")
	for _, a := range fileAssets {
		// 用 strconv.Quote 转义路径，用 Go 字面量格式嵌入字节
		b.WriteString("\t" + strconv.Quote(a.relPath) + ": " + goBytesLiteral(a.data) + ",\n")
	}
	b.WriteString("}\n\n")
	// 外置组件运行时配置：供运行时前端渲染外置组件（datepicker/treeview/colorpicker 等）
	b.WriteString("// embeddedComponents 存储外置组件的运行时渲染配置（HTML 模板 + 事件映射）。\n")
	b.WriteString("// 前端 App.vue 根据 type 查找此表，用 runtime.html 模板渲染组件并路由事件。\n")
	b.WriteString("var embeddedComponents = map[string]ComponentRuntimeConfig{\n")
	for _, c := range externalComps {
		// 事件 map 序列化为 Go 字面量
		eventsLiteral := "map[string]string{}"
		if len(c.Events) > 0 {
			var ev []string
			for domEvt, egEvt := range c.Events {
				ev = append(ev, strconv.Quote(domEvt)+": "+strconv.Quote(egEvt))
			}
			eventsLiteral = "map[string]string{" + strings.Join(ev, ", ") + "}"
		}
		b.WriteString("\t" + strconv.Quote(c.Type) + ": {HTML: " + strconv.Quote(c.HTML) + ", Events: " + eventsLiteral + "},\n")
	}
	b.WriteString("}\n")
	return os.WriteFile(filepath.Join(tmpDir, "embedded_assets.go"), []byte(b.String()), 0644)
}

// goBytesLiteral 把字节切片转为 Go 源码字面量（[]byte{...} 形式）。
// 用 strconv.Quote 处理字符串形式后转回字节，避免逐字节拼接过长。
func goBytesLiteral(data []byte) string {
	return "[]byte(" + strconv.Quote(string(data)) + ")"
}

// nativeLibExts 是被识别为原生库的文件扩展名（小写）。
var nativeLibExts = map[string]bool{
	".dll":   true, // Windows
	".so":    true, // Linux
	".dylib": true, // macOS
	".lib":   true, // Windows 静态库导入文件（部分场景需要）
	".a":     true, // GCC/MinGW 静态库
}

// copyNativeLibs 复制原生库到运行时临时目录，让运行时程序能加载。
// 来源（按优先级，项目级覆盖全局同名）：
//  1. exe 同级 native/（G3 全局原生库，所有项目共享）
//  2. <项目>/native/（P3 项目级原生库）
//
// 复制到 tmpDir 根目录（与 exe 同级，Windows DLL 搜索路径默认包含 exe 目录）。
func copyNativeLibs(tmpDir, projectPath string, sink EventSink) error {
	count := 0
	// 收集已复制文件名，项目级覆盖全局同名
	copied := map[string]bool{}

	// 1. 全局 native/
	if globalDir, err := globalNativeDir(); err == nil && globalDir != "" {
		n, err := copyNativeLibsFromDir(tmpDir, globalDir, copied, sink)
		if err != nil {
			return err
		}
		count += n
	}

	// 2. 项目级 native/（覆盖全局同名）
	if projectPath != "" {
		projNativeDir := filepath.Join(projectPath, "native")
		n, err := copyNativeLibsFromDir(tmpDir, projNativeDir, copied, sink)
		if err != nil {
			return err
		}
		count += n
	}

	if count > 0 {
		emit(sink, "stage", fmt.Sprintf("复制 %d 个原生库", count), false)
	}
	return nil
}

// globalNativeDir 返回 exe 同级的 native 目录路径（G3 全局原生库目录）。
// 目录不存在时返回空字符串。
func globalNativeDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(exePath), "native")
	if _, err := os.Stat(dir); err != nil {
		return "", nil
	}
	return dir, nil
}

// copyNativeLibsToDest 复制原生库到产物输出目录，让 release exe 能加载 DLL。
// 与 copyNativeLibs 来源一致：全局 native/ + 项目 native/（项目级覆盖全局同名）。
// 返回成功复制的文件数。
func copyNativeLibsToDest(destDir, projectPath string) (int, error) {
	copied := map[string]bool{}
	count := 0
	if globalDir, err := globalNativeDir(); err == nil && globalDir != "" {
		n, err := copyNativeLibsFromDir(destDir, globalDir, copied, nil)
		if err != nil {
			return count, err
		}
		count += n
	}
	if projectPath != "" {
		projNativeDir := filepath.Join(projectPath, "native")
		n, err := copyNativeLibsFromDir(destDir, projNativeDir, copied, nil)
		if err != nil {
			return count, err
		}
		count += n
	}
	return count, nil
}

// copyNativeLibsFromDir 把 srcDir 下的原生库文件复制到 tmpDir，跳过 copied 中已存在的文件名。
// 返回成功复制的文件数。
// L7：改为并行复制，用 WaitGroup + Mutex 保护共享状态（copied map 和 count），
// 多个原生库文件 IO 等待可重叠，减少总复制时间。
func copyNativeLibsFromDir(tmpDir, srcDir string, copied map[string]bool, sink EventSink) (int, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("读取 %s 失败: %w", srcDir, err)
	}
	type copyTask struct{ src, dst, name string }
	var tasks []copyTask
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !nativeLibExts[ext] {
			continue
		}
		tasks = append(tasks, copyTask{
			src:  filepath.Join(srcDir, e.Name()),
			dst:  filepath.Join(tmpDir, e.Name()),
			name: e.Name(),
		})
	}
	if len(tasks) == 0 {
		return 0, nil
	}
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		count    int
		firstErr error
	)
	for _, t := range tasks {
		wg.Add(1)
		go func(task copyTask) {
			defer wg.Done()
			mu.Lock()
			if copied[task.name] {
				mu.Unlock()
				return
			}
			copied[task.name] = true
			mu.Unlock()
			data, err := os.ReadFile(task.src)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("读取 %s 失败: %w", task.src, err)
				}
				mu.Unlock()
				return
			}
			if err := os.WriteFile(task.dst, data, 0644); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("写入 %s 失败: %w", task.dst, err)
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			count++
			mu.Unlock()
		}(t)
	}
	wg.Wait()
	return count, firstErr
}

// buildRuntime 在指定目录编译运行时 Go 后端。
//
// 编译模式差异（v0.8.6 调试器支持）：
//   - release=false（debug）：保留 DWARF 调试符号 + 不用 -trimpath，供 dlv 源码级调试
//   - release=true（release）：-s -w -trimpath 去除符号和路径，减小产物体积
//
// version 非空时通过 -X main.Version=xxx 注入到运行时二进制。
//
// 离线编译支持：检测到 dir/vendor/ 目录时自动添加 -mod=vendor 标志，
// 让 Go 直接从本地 vendor 读取依赖源码，无需联网下载。
func buildRuntime(dir, outFile string, sink EventSink, release bool, version string) error {
	emitProgress(sink, "build", 45)
	emit(sink, "build", "开始编译运行时...", false)
	// debug 模式保留 DWARF 调试符号（供 dlv 调试器使用），不用 -trimpath 保留源码路径映射。
	// release 模式用 -s -w -trimpath 去除符号和路径，减小产物体积。
	ldflags := "-H=windowsgui"
	if release {
		ldflags = "-s -w -H=windowsgui"
	}
	if version != "" {
		ldflags += " -X main.Version=" + version
	}
	var args []string
	args = append(args, "build")
	if release {
		args = append(args, "-trimpath")
	}
	args = append(args, "-buildvcs=false", "-tags", "netgo,osusergo", "-ldflags", ldflags)
	// 检测到 vendor/ 目录时启用 -mod=vendor 离线编译
	if _, err := os.Stat(filepath.Join(dir, "vendor")); err == nil {
		args = append(args, "-mod=vendor")
		emit(sink, "build", "检测到 vendor 目录，启用离线编译模式", false)
	}
	args = append(args, "-o", outFile, ".")
	// G4：检测到 cgo_link.go 时启用 CGO（默认禁用 CGO 以减小体积和加速编译）
	hasCgo := false
	if _, err := os.Stat(filepath.Join(dir, "cgo_link.go")); err == nil {
		hasCgo = true
		emit(sink, "build", "检测到静态库，启用 CGO 编译...", false)
	}

	cmd := exec.Command(goBinary, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	env := buildGoEnv()
	if hasCgo {
		env = append(env, "CGO_ENABLED=1")
	} else {
		env = append(env, "CGO_ENABLED=0")
	}
	if _, err := os.Stat(filepath.Join(dir, "vendor")); err == nil {
		env = append(env, "GOPROXY=off")
	}
	cmd.Env = env
	start := time.Now()
	output, err := cmd.CombinedOutput()
	if err != nil {
		parsed := parseGoCompileErrors(string(output))
		if len(parsed) > 0 {
			formatted := formatCompileErrors(parsed)
			emit(sink, "build", "编译失败："+formatted, false)
			return fmt.Errorf("编译失败:\n%s", formatted)
		}
		emit(sink, "build", "编译失败: "+string(output), false)
		return fmt.Errorf("编译失败: %s", string(output))
	}
	emit(sink, "build", fmt.Sprintf("编译完成（耗时 %s）", time.Since(start).Round(time.Millisecond)), false)
	emitProgress(sink, "link", 85)
	// 输出产物体积
	if fi, err := os.Stat(outFile); err == nil {
		emit(sink, "build", fmt.Sprintf("产物体积: %.2f MB", float64(fi.Size())/1024/1024), false)
	}
	return nil
}

// ===== P1-4：Go 编译错误结构化解析 =====
// CompileError 类型定义已迁移到 egou/internal/types（P2-19）。

// validateOutputInProject 校验可执行文件路径必须在项目目录内，
// 防止用户配置 output 字段指向系统目录（如 C:\Windows）导致编译产物覆盖系统文件。
// projectPath 为空时跳过校验（由上层调用方保证路径安全）。
func validateOutputInProject(exePath, projectPath string) error {
	if exePath == "" || projectPath == "" {
		return nil
	}
	absExe, err := filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("无法解析可执行文件路径: %w", err)
	}
	absProj, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("无法解析项目路径: %w", err)
	}
	rel, err := filepath.Rel(absProj, absExe)
	if err != nil {
		return fmt.Errorf("路径校验失败: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("可执行文件必须在项目目录内，当前: %s", absExe)
	}
	return nil
}

// goErrorRe 匹配 Go 编译器输出格式：file:line:col: message
var goErrorRe = regexp.MustCompile(`^(.+?):(\d+):(\d+):\s*(.+)$`)

// parseGoCompileErrors 用正则解析 Go 编译器的 CombinedOutput，提取结构化错误列表。
func parseGoCompileErrors(output string) []CompileError {
	var errs []CompileError
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := goErrorRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file := m[1]
		lineNum, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		msg := strings.TrimSpace(m[4])
		severity := "error"
		if strings.HasPrefix(msg, "warning:") || strings.HasPrefix(msg, "note:") {
			severity = "warning"
		}
		errs = append(errs, CompileError{
			File:     file,
			Line:     lineNum,
			Col:      col,
			Message:  msg,
			Severity: severity,
		})
	}
	return errs
}

// formatCompileErrors 将结构化错误列表格式化为中文友好的输出。
func formatCompileErrors(errs []CompileError) string {
	if len(errs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, e := range errs {
		icon := "❌"
		if e.Severity == "warning" {
			icon = "⚠️"
		}
		fmt.Fprintf(&b, "%s %s:%d:%d — %s\n", icon, e.File, e.Line, e.Col, translateGoError(e.Message))
	}
	return b.String()
}

// translateGoError 将常见 Go 编译错误英文提示翻译为中文。
// 翻译策略：用正则单词边界 \b 匹配英文单词，避免 "expected" 误匹配 "unexpected"。
// 长短语优先（先替换多词短语，再换单词），保留原始格式（如行号、引号内容）。
func translateGoError(msg string) string {
	// 多词短语优先（无需 \b，整串匹配）
	phrases := []struct{ from, to string }{
		{"no new variables on left side of :=", ":= 左侧没有新变量"},
		{"declared and not used", "已声明但未使用"},
		{"imported and not used", "已导入但未使用"},
		{"redeclared in this block", "在此代码块中重复声明"},
		{"not enough arguments", "参数不足"},
		{"too many arguments", "参数过多"},
		{"mismatched types", "类型不匹配"},
		{"in assignment to", "在赋值给"},
		{"does not match interface", "与接口不匹配"},
		{"cannot refer to unexported name", "无法引用未导出的名称"},
		{"method has no receiver", "方法没有接收者"},
		{"unexpected token", "意外的符号"},
		{"expected token", "期望的符号"},
		{"missing return", "缺少 return 语句"},
		{"missing argument", "缺少参数"},
		{"not enough return values", "返回值不足"},
		{"too many return values", "返回值过多"},
		{"cannot range over", "无法 range 遍历"},
		{"cannot take address", "无法取地址"},
		{"cannot refer to", "无法引用"},
		{"cannot assign", "无法赋值"},
		{"cannot convert", "无法转换"},
		{"cannot call", "无法调用"},
		{"cannot index", "无法索引"},
		{"cannot slice", "无法切片"},
		{"cannot range", "无法遍历"},
		{"cannot compare", "无法比较"},
		{"cannot be used", "无法使用"},
		{"cannot use", "无法使用"},
		{"cannot find", "找不到"},
		{"as type", "作为类型"},
		{"unused variable", "未使用的变量"},
		{"unused import", "未使用的导入"},
		{"syntax error", "语法错误"},
		{"undefined:", "未定义:"},
		{"not defined", "未定义"},
		{"does not exist", "不存在"},
		{"already declared", "已声明"},
		{"not enough parameters", "参数不足"},
		{"too many parameters", "参数过多"},
		{"unknown field", "未知字段"},
		{"unknown identifier", "未知标识符"},
		{"not assignable", "不可赋值"},
		{"not comparable", "不可比较"},
		{"in call to", "在调用"},
		{" and ", " 和 "},
	}
	result := msg
	for _, r := range phrases {
		result = strings.ReplaceAll(result, r.from, r.to)
	}
	// 单词用 \b 边界匹配（避免 expected 误匹配 unexpected）
	words := []struct{ from, to string }{
		{"unexpected", "意外的"},
		{"expected", "期望"},
		{"missing", "缺少"},
		{"duplicate", "重复"},
		{"invalid", "无效"},
		{"illegal", "非法"},
		{"found", "但找到"},
	}
	for _, r := range words {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(r.from) + `\b`)
		result = re.ReplaceAllString(result, r.to)
	}
	return result
}

// runRuntime 启动已编译的运行时程序，捕获 stdout/stderr 并通过 sink 实时回调。
//
// 死循环保护策略（P3-7）：
//   - EGOU 采用「转译成 Go 原生代码」架构，没有解释器节点概念
//   - GUI 程序死循环不会卡住 IDE：用户关闭窗口即可终止进程（runCmd.Wait 会返回）
//   - 全局步数上限 MaxLoopIterations（types.go）供未来调试器/解释器模式使用
//   - 当前架构下，进程隔离是主要的死循环保护机制
func runRuntime(tmpDir, outFile, projectPath string, sink EventSink) (string, error) {
	emitProgress(sink, "run", 92)
	emit(sink, "run", "启动运行时窗口...", false)
	runCmd := exec.Command(outFile)
	runCmd.Dir = tmpDir
	runCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	env := os.Environ()
	if projectPath != "" {
		env = append(env, "EG_PROJECT_PATH="+projectPath)
		// 日志输出到项目目录下的 egruntime.log，避免污染 IDE 工作目录
		env = append(env, "EG_LOG_PATH="+filepath.Join(projectPath, "egruntime.log"))
	}
	// projectPath 为空时不设置 EG_LOG_PATH，让运行时模板自行决定日志位置（默认临时目录）
	runCmd.Env = env

	stdout, err := runCmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("获取 stdout 失败: %w", err)
	}
	stderr, err := runCmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("获取 stderr 失败: %w", err)
	}
	if err := runCmd.Start(); err != nil {
		emit(sink, "run", "启动失败: "+err.Error(), false)
		return "", fmt.Errorf("启动失败: %w", err)
	}
	emit(sink, "run", "运行时已启动，关闭窗口或 Ctrl+C 结束", false)
	emitProgress(sink, "done", 100)

	go pumpLines(stdout, sink, false)
	go pumpLines(stderr, sink, true)

	if err := runCmd.Wait(); err != nil {
		emit(sink, "run", "运行时退出，返回错误: "+err.Error(), false)
		return "", fmt.Errorf("运行失败: %w", err)
	}
	emit(sink, "run", "运行时已退出", false)
	return "", nil
}

// pumpLines 从 r 中逐行读取并通过 sink 转发，isOutput=true 标记为运行时输出。
func pumpLines(r io.Reader, sink EventSink, isOutput bool) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		emit(sink, "run", line, isOutput)
	}
}

// ensureFrontendDist 确保模板前端资源已构建，且 Wails bindings 已生成。
// 优先级：exe 同级预打包缓存 > 用户缓存目录 > 在线构建。
// 这样分发时把 EGOU.exe + runtime-frontend/ 一起打包，
// 无 Node.js 环境的机器也能直接运行用户项目。
func ensureFrontendDist(dir string, sink EventSink) error {
	// 1. 优先检查 exe 同级 runtime-frontend/（分发时预打包）
	if binCache, err := binCacheDir(); err == nil && hasFrontendCache(binCache) {
		emit(sink, "frontend", "复用 exe 同级预打包的前端资源", false)
		return copyFrontendCache(dir, binCache)
	}
	// 2. 再检查用户缓存目录（IDE 启动时 PrepareRuntimeCache 填充）
	cacheDir, err := runtimeCacheDir()
	if err != nil {
		return err
	}
	if hasFrontendCache(cacheDir) {
		emit(sink, "frontend", "复用已缓存的前端资源", false)
		return copyFrontendCache(dir, cacheDir)
	}
	// 3. 都没有时在线构建（需要 npm）
	emit(sink, "frontend", "首次运行，正在构建前端资源（耗时较久）...", false)
	if err := buildFrontendDist(dir, sink); err != nil {
		return err
	}
	return saveFrontendCache(dir, cacheDir)
}

// buildFrontendDist 在指定目录执行准备前端资源的步骤。
// 若目录下存在 vendor/（离线依赖），则跳过 go mod tidy 直接使用 vendor，
// 无需联网；否则执行 go mod tidy 下载依赖（开发环境首次构建）。
func buildFrontendDist(dir string, sink EventSink) error {
	hasVendor := false
	if _, err := os.Stat(filepath.Join(dir, "vendor")); err == nil {
		hasVendor = true
	}
	if !hasVendor {
		emit(sink, "frontend", "[1/3] go mod tidy", false)
		tidyCmd := exec.Command(goBinary, "mod", "tidy")
		tidyCmd.Dir = dir
		tidyCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		tidyCmd.Env = buildGoEnv()
		if output, err := tidyCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go mod tidy 失败: %s", string(output))
		}
	} else {
		emit(sink, "frontend", "[1/3] 检测到 vendor 目录，跳过 go mod tidy（离线模式）", false)
	}

	emit(sink, "frontend", "[2/3] 生成 Wails bindings", false)
	wailsBin := findWails3Cli()
	if wailsBin == "" {
		return fmt.Errorf("未找到 Wails3 CLI，请设置 WAILS3 环境变量")
	}
	genCmd := exec.Command(wailsBin, "generate", "bindings", "-ts")
	genCmd.Dir = dir
	genCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	genCmd.Env = buildGoEnv()
	if hasVendor {
		genCmd.Env = append(genCmd.Env, "GOFLAGS=-mod=vendor", "GOPROXY=off")
	}
	if output, err := genCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("生成 Wails bindings 失败: %s", string(output))
	}

	emit(sink, "frontend", "[3/3] npm install + vite build", false)
	frontendDir := filepath.Join(dir, "frontend")
	npm := findNPM()
	if npm == "" {
		return fmt.Errorf("未找到 npm")
	}
	installCmd := exec.Command(npm, "install")
	installCmd.Dir = frontendDir
	installCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm install 失败: %s", string(output))
	}
	buildCmd := exec.Command(npm, "run", "build")
	buildCmd.Dir = frontendDir
	buildCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm run build 失败: %s", string(output))
	}
	emit(sink, "frontend", "前端资源构建完成", false)
	return nil
}

// runtimeCacheDir 返回运行时前端资源的本地缓存目录。
func runtimeCacheDir() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("获取用户缓存目录失败: %w", err)
	}
	return filepath.Join(cacheRoot, "egou", "runtime-frontend"), nil
}

// binCacheDir 返回 exe 同级的 runtime-frontend 目录（分发时预打包）。
// 开发时（go run）exe 在临时目录，binCacheDir 可能不存在，调用方需检查 hasFrontendCache。
func binCacheDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取 exe 路径失败: %w", err)
	}
	return filepath.Join(filepath.Dir(exePath), "runtime-frontend"), nil
}

// hasFrontendCache 判断缓存目录是否包含有效的前端构建产物。
func hasFrontendCache(cacheDir string) bool {
	if _, err := os.Stat(filepath.Join(cacheDir, "frontend", "dist", "index.html")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "frontend", "bindings")); err != nil {
		return false
	}
	return true
}

// copyFrontendCache 将缓存的前端资源复制到临时构建目录。
func copyFrontendCache(dir, cacheDir string) error {
	src := filepath.Join(cacheDir, "frontend")
	dst := filepath.Join(dir, "frontend")
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("创建前端目录失败: %w", err)
	}
	return copyDirWithSkip(src, dst, []string{"node_modules"})
}

// saveFrontendCache 将临时构建目录中的前端资源保存到缓存。
func saveFrontendCache(dir, cacheDir string) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}
	src := filepath.Join(dir, "frontend")
	dst := filepath.Join(cacheDir, "frontend")
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("清理旧缓存失败: %w", err)
	}
	return copyDirWithSkip(src, dst, []string{"node_modules"})
}

// PrepareRuntimeCache 预构建运行时前端资源并缓存。
// 在编译 IDE 或首次安装时调用，可显著缩短后续运行/构建的耗时。
func PrepareRuntimeCache() error {
	tmpDir, err := os.MkdirTemp("", "eg-runtime-cache-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTemplate(tmpDir); err != nil {
		return fmt.Errorf("提取运行时模板失败: %w", err)
	}

	userCodeFile := filepath.Join(tmpDir, "usercode.go")
	placeholder := "package main\n\nfunc mainImpl() {}\nfunc registerHandlersImpl() {}\n"
	if err := os.WriteFile(userCodeFile, []byte(placeholder), 0644); err != nil {
		return fmt.Errorf("写入占位用户代码失败: %w", err)
	}

	if err := buildFrontendDist(tmpDir, nil); err != nil {
		return fmt.Errorf("构建运行时前端资源失败: %w", err)
	}

	cacheDir, err := runtimeCacheDir()
	if err != nil {
		return err
	}
	return saveFrontendCache(tmpDir, cacheDir)
}

// PrepareRuntimeCacheTo 预构建运行时前端资源并缓存到指定目录（如 bin/runtime-frontend）。
// 用于 IDE 构建脚本预打包缓存到 exe 同级目录，让无 Node.js 环境的机器也能运行用户项目。
// dst 是缓存根目录（内部会创建 frontend/ 子目录）。
func PrepareRuntimeCacheTo(dst string) error {
	tmpDir, err := os.MkdirTemp("", "eg-runtime-cache-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTemplate(tmpDir); err != nil {
		return fmt.Errorf("提取运行时模板失败: %w", err)
	}

	userCodeFile := filepath.Join(tmpDir, "usercode.go")
	placeholder := "package main\n\nfunc mainImpl() {}\nfunc registerHandlersImpl() {}\n"
	if err := os.WriteFile(userCodeFile, []byte(placeholder), 0644); err != nil {
		return fmt.Errorf("写入占位用户代码失败: %w", err)
	}

	if err := buildFrontendDist(tmpDir, nil); err != nil {
		return fmt.Errorf("构建运行时前端资源失败: %w", err)
	}

	return saveFrontendCache(tmpDir, dst)
}

// extractTemplate 将外置 wails-template 目录复制到目标路径。
// 第八版采用完整外置化：模板与 exe 同级，编译用户程序时复制到临时目录。
// 模板中的 go.mod 在源码中以 go.mod.tmpl 命名（避免被识别为外部 module），
// 复制后还原为 go.mod。
func extractTemplate(dst string) error {
	src := templateDir
	if src == "" {
		// 未注入模板目录，从 exe 同级目录查找
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("获取 exe 路径失败: %w", err)
		}
		src = filepath.Join(filepath.Dir(exePath), "wails-template")
	}

	// 检查源目录存在
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("外置 wails-template 目录不存在: %s", src)
	}

	// 复制目录
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		return err
	}

	// 还原 go.mod.tmpl → go.mod
	tmplPath := filepath.Join(dst, "go.mod.tmpl")
	if _, statErr := os.Stat(tmplPath); statErr == nil {
		if renameErr := os.Rename(tmplPath, filepath.Join(dst, "go.mod")); renameErr != nil {
			return fmt.Errorf("还原 go.mod 失败: %w", renameErr)
		}
	}
	return nil
}

// copyDirWithSkip 递归复制目录，可自定义需要跳过的目录名。
func copyDirWithSkip(src, dst string, skip []string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, info.Mode())
		}
		if info.IsDir() {
			for _, name := range skip {
				if info.Name() == name {
					return filepath.SkipDir
				}
			}
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// findNPM 查找 npm 可执行文件路径。
func findNPM() string {
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("npm.cmd"); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("npm"); err == nil {
		return p
	}
	return ""
}

// findWails3Cli 查找 wails3 可执行文件路径。
// 查找顺序：1. WAILS3 环境变量 → 2. exe 同级 tools/ → 3. PATH 查找 → 4. GOPATH/bin 回退。
func findWails3Cli() string {
	if wails3Cli != "" {
		if _, err := os.Stat(wails3Cli); err == nil {
			return wails3Cli
		}
	}
	// 1.5 优先从 exe 同级 tools/ 目录查找（随发布包自带，无需用户安装）
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates := []string{
			filepath.Join(exeDir, "tools", "wails3.exe"),
			filepath.Join(exeDir, "tools", "wails3"),
			filepath.Join(exeDir, "wails3.exe"),
			filepath.Join(exeDir, "wails3"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	candidates := []string{"wails3.exe", "wails3"}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		exeName := "wails3"
		if runtime.GOOS == "windows" {
			exeName = "wails3.exe"
		}
		defaultPath := filepath.Join(gopath, "bin", exeName)
		if _, err := os.Stat(defaultPath); err == nil {
			return defaultPath
		}
	}
	return ""
}

// readProjectVersion 读取项目根目录下 project.eg.json 的 version 字段。
// 文件不存在或解析失败时返回空字符串，不影响构建流程。
func readProjectVersion(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	configPath := filepath.Join(projectPath, "project.eg.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	ver, _ := raw["version"].(string)
	return ver
}

// readProjectOutput 读取项目根目录下 project.eg.json 的 output 字段，
// 返回产物输出目录（相对项目根）。空字符串或字段缺失时返回默认 "bin"。
// 用于 P2：让构建产物写到 Output 指定目录而非项目根目录。
func readProjectOutput(projectPath string) string {
	if projectPath == "" {
		return "bin"
	}
	configPath := filepath.Join(projectPath, "project.eg.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "bin"
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "bin"
	}
	out, _ := raw["output"].(string)
	if out == "" {
		return "bin"
	}
	return out
}

// generateSyso 生成 Windows 资源文件（.syso），嵌入图标和版本信息到编译产物。
// 优先用 wails3 generate syso（同时嵌入图标 + manifest + 版本/版权信息），
// 其次 rsrc（仅图标，rsrc 嵌入 manifest 会导致 Wails v3 Frameless 冲突），
// 最后 windres 回退。
// 如果 projectPath 下的 project.eg.json 配置了自定义图标路径（iconPath），则覆盖模板默认图标。
// 同时根据项目配置（version/companyName/fileDescription/legalCopyright/productName/comments）
// 动态生成 info.json，让产物 exe 的文件属性信息正确。
// 生成的 windows_amd64.syso 放在 tmpDir 根目录，go build 按文件名自动链接。
// 失败时返回错误，调用方可选择跳过（不阻断构建）。
func generateSyso(tmpDir string, projectPath string, sink EventSink) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("非 Windows 平台跳过 syso 生成")
	}
	buildDir := filepath.Join(tmpDir, "build")
	iconPath := filepath.Join(buildDir, "windows", "icon.ico")
	manifestPath := filepath.Join(buildDir, "windows", "wails.exe.manifest")
	infoPath := filepath.Join(buildDir, "windows", "info.json")

	// 读取项目配置（图标路径 + 版本/版权信息）
	type projCfg struct {
		IconPath        string `json:"iconPath"`
		Version         string `json:"version"`
		CompanyName     string `json:"companyName"`
		FileDescription string `json:"fileDescription"`
		LegalCopyright  string `json:"legalCopyright"`
		ProductName     string `json:"productName"`
		Comments        string `json:"comments"`
		Name            string `json:"name"`
		Description     string `json:"description"`
		Author          string `json:"author"`
	}
	var cfg projCfg
	if projectPath != "" {
		if cfgPath := filepath.Join(projectPath, "project.eg.json"); fileExists(cfgPath) {
			if data, err := os.ReadFile(cfgPath); err == nil {
				_ = json.Unmarshal(data, &cfg)
			}
		}
	}

	// 自定义图标覆盖模板默认图标
	if cfg.IconPath != "" {
		customIcon := cfg.IconPath
		if !filepath.IsAbs(customIcon) {
			customIcon = filepath.Join(projectPath, customIcon)
		}
		if fileExists(customIcon) {
			if data, err := os.ReadFile(customIcon); err == nil {
				_ = os.MkdirAll(filepath.Dir(iconPath), 0755)
				_ = os.WriteFile(iconPath, data, 0644)
				emit(sink, "stage", "使用自定义图标: "+cfg.IconPath, false)
			}
		}
	}

	if _, err := os.Stat(iconPath); err != nil {
		return fmt.Errorf("图标文件不存在: %s", iconPath)
	}

	// 动态生成 info.json（版本/版权信息），让产物 exe 文件属性正确
	// 字段缺失时用合理默认值，避免空值
	version := cfg.Version
	if version == "" {
		version = "1.0.0"
	}
	productName := cfg.ProductName
	if productName == "" {
		productName = cfg.Name
	}
	if productName == "" {
		productName = "EGOU Application"
	}
	fileDesc := cfg.FileDescription
	if fileDesc == "" {
		fileDesc = cfg.Description
	}
	if fileDesc == "" {
		fileDesc = productName
	}
	companyName := cfg.CompanyName
	if companyName == "" {
		companyName = cfg.Author
	}
	if companyName == "" {
		companyName = "EGOU"
	}
	legalCopyright := cfg.LegalCopyright
	if legalCopyright == "" {
		legalCopyright = "© 2026, " + companyName
	}
	infoContent := fmt.Sprintf(`{
	"fixed": {
		"file_version": "%s"
	},
	"info": {
		"0000": {
			"ProductVersion": "%s",
			"CompanyName": "%s",
			"FileDescription": "%s",
			"LegalCopyright": "%s",
			"ProductName": "%s",
			"Comments": "%s"
		}
	}
}`, version, version, companyName, fileDesc, legalCopyright, productName, cfg.Comments)
	if err := os.WriteFile(infoPath, []byte(infoContent), 0644); err != nil {
		emit(sink, "stage", "写入 info.json 失败: "+err.Error(), false)
	}

	arch := runtime.GOARCH
	// 文件名必须是 {os}_{arch}.syso，go build 才会自动链接
	outFile := filepath.Join(tmpDir, fmt.Sprintf("windows_%s.syso", arch))

	// 方案1：优先用 wails3 generate syso（同时嵌入图标 + manifest + 版本信息）
	if wails3 := findWails3Cli(); wails3 != "" {
		emit(sink, "stage", "生成 Windows 资源文件（wails3：图标+清单+版本信息）...", false)
		cmd := exec.Command(wails3, "generate", "syso",
			"-arch", arch,
			"-icon", iconPath,
			"-manifest", manifestPath,
			"-info", infoPath,
			"-out", outFile,
		)
		cmd.Dir = buildDir
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		output, err := cmd.CombinedOutput()
		if err == nil {
			emit(sink, "stage", "Windows 资源文件已生成（wails3）", false)
			return nil
		}
		emit(sink, "stage", "wails3 生成失败，尝试 rsrc 回退: "+string(output), false)
	}

	// 方案2：rsrc 回退（仅图标，不嵌入 manifest 避免 Wails v3 Frameless 冲突）
	if rsrc := findRsrcCli(); rsrc != "" {
		emit(sink, "stage", "生成 Windows 资源文件（rsrc：仅图标）...", false)
		args := []string{"-ico", iconPath, "-arch", arch, "-o", outFile}
		cmd := exec.Command(rsrc, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		output, err := cmd.CombinedOutput()
		if err == nil {
			emit(sink, "stage", "Windows 资源文件已生成（rsrc，无版本信息）", false)
			return nil
		}
		emit(sink, "stage", "rsrc 生成失败，尝试 windres 回退: "+string(output), false)
	}

	// 方案3：windres 回退（嵌入图标 + manifest + 版本信息，不依赖 wails3 CLI）
	windres := findWindres()
	if windres == "" {
		return fmt.Errorf("未找到 wails3 CLI、rsrc 和 windres，无法生成图标资源")
	}
	emit(sink, "stage", "生成 Windows 资源文件（windres：图标+清单+版本信息）...", false)

	// 将 icon.ico 和 manifest 复制到 tmpDir 根目录，windres 用相对路径引用避免转义问题
	rcIcon := filepath.Join(tmpDir, "icon.ico")
	rcManifest := filepath.Join(tmpDir, "wails.exe.manifest")
	if err := copyFile(iconPath, rcIcon); err != nil {
		return fmt.Errorf("复制图标文件失败: %w", err)
	}
	if err := copyFile(manifestPath, rcManifest); err != nil {
		return fmt.Errorf("复制manifest失败: %w", err)
	}
	defer os.Remove(rcIcon)
	defer os.Remove(rcManifest)

	// 解析版本号为 (major, minor, patch, build) 用于 VERSIONINFO
	verParts := strings.Split(version, ".")
	for len(verParts) < 4 {
		verParts = append(verParts, "0")
	}
	verNums := make([]uint16, 4)
	for i := 0; i < 4; i++ {
		n, _ := strconv.Atoi(verParts[i])
		verNums[i] = uint16(n)
	}
	verHex := fmt.Sprintf("%d,%d,%d,%d", verNums[0], verNums[1], verNums[2], verNums[3])

	// rcEscape 转义 RC 字符串中的特殊字符（双引号需要双写）
	rcEscape := func(s string) string {
		s = strings.ReplaceAll(s, "\x00", "")
		s = strings.ReplaceAll(s, "\"", "\"\"")
		return s
	}

	// 生成 .rc 资源脚本：图标 + manifest + VS_VERSION_INFO
	rcContent := fmt.Sprintf(`1 ICON "icon.ico"
1 24 "wails.exe.manifest"

VS_VERSION_INFO VERSIONINFO
 FILEVERSION %s
 PRODUCTVERSION %s
 FILEFLAGSMASK 0x3fL
 FILEFLAGS 0x0L
 FILEOS 0x40004L
 FILETYPE 0x1L
 FILESUBTYPE 0x0L
BEGIN
    BLOCK "StringFileInfo"
    BEGIN
        BLOCK "080404b0"
        BEGIN
            VALUE "CompanyName", "%s"
            VALUE "FileDescription", "%s"
            VALUE "FileVersion", "%s"
            VALUE "LegalCopyright", "%s"
            VALUE "ProductName", "%s"
            VALUE "ProductVersion", "%s"
            VALUE "Comments", "%s"
            VALUE "OriginalFilename", "%s"
        END
    END
    BLOCK "VarFileInfo"
    BEGIN
        VALUE "Translation", 0x0804, 1200
    END
END
`, verHex, verHex,
		rcEscape(companyName), rcEscape(fileDesc), version, rcEscape(legalCopyright),
		rcEscape(productName), version, rcEscape(cfg.Comments), filepath.Base(outFile))

	rcFile := filepath.Join(tmpDir, "resources.rc")
	if err := os.WriteFile(rcFile, []byte(rcContent), 0644); err != nil {
		return fmt.Errorf("写入 rc 文件失败: %w", err)
	}
	defer os.Remove(rcFile)

	coffFile := filepath.Join(tmpDir, "resources-coff.o")
	cmd := exec.Command(windres, "-O", "coff", "-i", rcFile, "-o", coffFile)
	cmd.Dir = tmpDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("windres 编译失败: %s\n%s", err, string(output))
	}
	// 将 COFF 目标文件重命名为 .syso（go build 自动链接）
	if err := os.Rename(coffFile, outFile); err != nil {
		return fmt.Errorf("重命名 syso 文件失败: %w", err)
	}
	emit(sink, "stage", "Windows 资源文件已生成（windres：图标+清单+版本信息）", false)
	return nil
}

// findRsrcCli 查找 rsrc 工具路径（GOPATH/bin 或 PATH）。
func findRsrcCli() string {
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		candidate := filepath.Join(gopath, "bin", "rsrc.exe")
		if fileExists(candidate) {
			return candidate
		}
	}
	if p, err := exec.LookPath("rsrc"); err == nil {
		return p
	}
	return ""
}

// fileExists 检查文件是否存在。
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// copyFile 复制单个文件，保留文件权限为 0644。
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
