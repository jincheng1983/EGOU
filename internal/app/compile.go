// compile.go 实现编译运行 + 构建相关方法。
//
// 这是 IDEService 的核心职责之一：把 .eg 源码转译为 Go，编译为可执行文件，
// 运行用户程序。所有方法都通过 Wails 事件 "ide:run-event" 实时回传阶段进度。
//
// 单文件符号索引见 symbols.go，跨文件符号索引见 crossref.go
// （v0.6.12 拆分，遵守单文件不超过 500 行规约）。
//
// 第七版对应方法直接迁移，仅按第八版命名规约重命名：
//   - "nxg-parser" → "eg-parser"
//   - "nxruntime-standalone.exe" → "egruntime-standalone.exe"
//   - source.nxg → source.eg
//   - ".nxg" → ".eg"

package app

import (
	"os"

	"egou/internal/runner"
	"egou/internal/transpiler"
)

// CodeResponse 统一返回运行/转译/构建结果。
type CodeResponse struct {
	Output string `json:"output,omitempty"`
	Go     string `json:"go,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Transpile 将 .eg 源码转译为 Go 源码。
func (s *IDEService) Transpile(source string) CodeResponse {
	goSrc, err := transpiler.Transpile(source)
	if err != nil {
		return CodeResponse{Error: err.Error()}
	}
	return CodeResponse{Go: goSrc}
}

// Run 同步转译并运行 .eg 源码，输出和错误合并返回。
// 运行过程中会通过 Wails 事件 "ide:run-event" 实时发送阶段进度和用户输出。
// projectPath 用于加载窗口设计文件（.ew），为空时不影响普通代码。
func (s *IDEService) Run(source string, projectPath string) CodeResponse {
	var combined string
	sink := func(ev runner.Event) {
		s.emitEvent(ev)
		combined += ev.Output + "\n"
	}
	_, err := runner.RunSource(source, projectPath, sink)
	if err != nil {
		s.emitEvent(runner.Event{Stage: "error", Output: err.Error()})
		return CodeResponse{Error: err.Error()}
	}
	s.emitEvent(runner.Event{Stage: "done", Output: "完成"})
	return CodeResponse{Output: combined}
}

// emitEvent 把 runner 事件转发为 Wails 事件，供前端订阅实时显示进度。
func (s *IDEService) emitEvent(ev runner.Event) {
	if s.app == nil {
		return
	}
	s.app.Event.Emit("ide:run-event", map[string]any{
		"stage":    ev.Stage,
		"output":   ev.Output,
		"isOutput": ev.IsOutput,
	})
}

// Build 同步转译并构建 .eg 源码；进度同样通过 "ide:run-event" 事件实时回传。
// projectPath 必须非空：产物必须输出到项目目录，避免污染 IDE 工作目录，
// 也保证多开 IDE 与多项目同时运行时互不干扰。
func (s *IDEService) Build(source string, projectPath string) CodeResponse {
	if projectPath == "" {
		errMsg := "未打开项目，无法编译（请先新建或打开项目，产物必须输出到项目目录）"
		s.emitEvent(runner.Event{Stage: "error", Output: errMsg})
		return CodeResponse{Error: errMsg}
	}
	var combined string
	sink := func(ev runner.Event) {
		s.emitEvent(ev)
		combined += ev.Output + "\n"
	}
	_, err := runner.BuildSource(source, projectPath, sink)
	if err != nil {
		s.emitEvent(runner.Event{Stage: "error", Output: err.Error()})
		return CodeResponse{Error: err.Error()}
	}
	s.emitEvent(runner.Event{Stage: "done", Output: "完成"})
	return CodeResponse{Output: combined}
}

// BuildProject 转译并构建项目中的 main.eg，输出可执行文件到项目根目录。
// 进度通过 "ide:run-event" 事件实时回传。
func (s *IDEService) BuildProject(sourcePath string, projectPath string) CodeResponse {
	if projectPath == "" {
		errMsg := "未打开项目，无法编译（请先新建或打开项目，产物必须输出到项目目录）"
		s.emitEvent(runner.Event{Stage: "error", Output: errMsg})
		return CodeResponse{Error: errMsg}
	}
	if sourcePath == "" {
		return CodeResponse{Error: "未指定源码路径"}
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return CodeResponse{Error: "读取源码失败: " + err.Error()}
	}
	return s.Build(string(data), projectPath)
}

// BuildProjectRelease 与 BuildProject 相同，但启用 release 模式（-ldflags "-s -w"），
// 去除调试符号和 DWARF 信息，生成的可执行文件体积更小，适合分发。
func (s *IDEService) BuildProjectRelease(projectPath string) CodeResponse {
	if projectPath == "" {
		return CodeResponse{Error: "未打开项目"}
	}
	src, err := collectProjectSources(projectPath)
	if err != nil {
		return CodeResponse{Error: err.Error()}
	}
	var combined string
	sink := func(ev runner.Event) {
		s.emitEvent(ev)
		combined += ev.Output + "\n"
	}
	_, err = runner.BuildSourceRelease(src, projectPath, sink)
	if err != nil {
		s.emitEvent(runner.Event{Stage: "error", Output: err.Error()})
		return CodeResponse{Error: err.Error()}
	}
	// 版本号递增已由 runner.BuildSourceRelease 内部处理，无需重复调用
	s.emitEvent(runner.Event{Stage: "done", Output: "完成"})
	return CodeResponse{Output: combined}
}

// ExportStandaloneProject 导出独立可执行文件：release 模式编译 + 嵌入所有 .ew 资源。
// 生成的 exe 单文件即可分发运行，无需外部项目目录。
// 输出到项目根目录的 egruntime-standalone.exe。
func (s *IDEService) ExportStandaloneProject(projectPath string) CodeResponse {
	if projectPath == "" {
		return CodeResponse{Error: "未打开项目"}
	}
	src, err := collectProjectSources(projectPath)
	if err != nil {
		return CodeResponse{Error: err.Error()}
	}
	var combined string
	sink := func(ev runner.Event) {
		s.emitEvent(ev)
		combined += ev.Output + "\n"
	}
	// BuildSourceRelease 内部 prepareRuntimeBuild 会自动调用 writeEmbeddedAssets 嵌入 .ew
	result, err := runner.BuildSourceRelease(src, projectPath, sink)
	if err != nil {
		s.emitEvent(runner.Event{Stage: "error", Output: err.Error()})
		return CodeResponse{Error: err.Error()}
	}
	// 版本号递增已由 runner.BuildSourceRelease 内部处理，无需重复调用
	s.emitEvent(runner.Event{Stage: "done", Output: "导出完成（含嵌入资源）"})
	return CodeResponse{Output: combined + "\n" + result}
}

// RunProject 编译运行整个项目，自动读取 main.eg 作为入口，并把项目下所有 .eg
// 文件一起参与转译，确保窗口/模块/类的函数与事件处理函数都会被编译进去。
func (s *IDEService) RunProject(projectPath string) CodeResponse {
	if projectPath == "" {
		return CodeResponse{Error: "未打开项目"}
	}
	// P2-14：路径安全三件套 — 运行前校验
	if err := validateProjectPath(projectPath); err != nil {
		return CodeResponse{Error: err.Error()}
	}
	src, err := collectProjectSources(projectPath)
	if err != nil {
		return CodeResponse{Error: err.Error()}
	}
	return s.Run(src, projectPath)
}
