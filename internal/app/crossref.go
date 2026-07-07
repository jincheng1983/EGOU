// crossref.go 实现跨文件符号索引 API。
//
// 设计：
//   - 一次性扫描项目所有 .eg + 项目 libs/*/source.eg + 全局 libs/*/source.eg
//   - 返回统一符号索引，前端缓存供 F12/Shift+F12 跨文件跳转使用
//   - 避免前端逐个 RPC 读取文件（原 onGotoDef 每次都磁盘 I/O）
//   - 项目路径作为参数传入（IDEService 不持有项目状态）
//
// 从 compile.go 拆分（v0.6.12 单文件不超过 500 行规约）。

package app

import (
	"os"
	"path/filepath"
	"strings"

	"egou/internal/transpiler"
)

// SymbolEntry 是跨文件符号索引条目
type SymbolEntry struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"` // function/method/type/const/var
	File       string      `json:"file"` // 符号所在文件的绝对路径
	Line       int         `json:"line"` // 1-based 行号
	Col        int         `json:"col"`  // 1-based 列号
	Params     []ParamInfo `json:"params,omitempty"`
	ReturnType string      `json:"returnType,omitempty"`
	Source     string      `json:"source"`           // "project" / "project-lib" / "global-lib"
	PkgName    string      `json:"pkgName,omitempty"` // .elib 包名（source=project-lib/global-lib 时）
}

// AllSymbolsResponse 是 ListAllSymbols 的响应
type AllSymbolsResponse struct {
	Symbols []SymbolEntry `json:"symbols,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// ListAllSymbols 扫描项目所有 .eg + 项目 libs + 全局 libs，返回统一符号索引。
// 用于编辑器 F12 跨文件跳转 / Shift+F12 跨文件查找引用。
// projectPath 为空时只扫描全局 libs。
func (s *IDEService) ListAllSymbols(projectPath string) AllSymbolsResponse {
	var symbols []SymbolEntry

	// 1. 项目内 .eg 文件（根目录 + modules/ + types/）
	if projectPath != "" {
		symbols = append(symbols, s.scanEgFiles(projectPath, "", "project")...)

		// 2. 项目 libs/*/source.eg
		projLibsDir := filepath.Join(projectPath, "libs")
		if entries, err := os.ReadDir(projLibsDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				pkgName := entry.Name()
				srcPath := filepath.Join(projLibsDir, pkgName, "source.eg")
				symbols = append(symbols, s.scanSingleEg(srcPath, pkgName, "project-lib")...)
			}
		}
	}

	// 3. 全局 libs/*/source.eg（exe 同级）
	globalLibsDir := s.GetGlobalLibsDir()
	if globalLibsDir != "" {
		if entries, err := os.ReadDir(globalLibsDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				pkgName := entry.Name()
				srcPath := filepath.Join(globalLibsDir, pkgName, "source.eg")
				symbols = append(symbols, s.scanSingleEg(srcPath, pkgName, "global-lib")...)
			}
		}
	}

	return AllSymbolsResponse{Symbols: symbols}
}

// scanEgFiles 扫描 dir 下的 .eg 文件（递归 modules/ types/ 子目录）
// pkgName 为空表示项目源码文件，否则为 .elib 包名
func (s *IDEService) scanEgFiles(dir, pkgName, source string) []SymbolEntry {
	var out []SymbolEntry
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		name := entry.Name()
		// 跳过无关目录
		if entry.IsDir() {
			if name == "bin" || name == ".git" || name == "node_modules" || name == "libs" || name == "assets" || name == "native" {
				continue
			}
			out = append(out, s.scanEgFiles(filepath.Join(dir, name), pkgName, source)...)
			continue
		}
		if !strings.HasSuffix(name, ".eg") {
			continue
		}
		// 跳过 source.eg（在 .elib 目录里单独处理）
		if name == "source.eg" {
			continue
		}
		out = append(out, s.scanSingleEg(filepath.Join(dir, name), pkgName, source)...)
	}
	return out
}

// scanSingleEg 解析单个 .eg 文件，提取顶层符号
func (s *IDEService) scanSingleEg(path, pkgName, source string) []SymbolEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	file, _ := transpiler.Parse(string(data))
	if file == nil {
		return nil
	}
	syms := transpiler.CollectSymbols(file)
	out := make([]SymbolEntry, 0, len(syms))
	for _, sym := range syms {
		entry := SymbolEntry{
			Name:       sym.Name,
			Kind:       transpiler.SymbolNameKind(sym.Kind),
			File:       path,
			Line:       sym.Pos.Line,
			Col:        sym.Pos.Col,
			ReturnType: sym.ReturnType,
			Source:     source,
			PkgName:    pkgName,
		}
		if sym.Params != nil {
			entry.Params = make([]ParamInfo, 0, len(sym.Params))
			for _, p := range sym.Params {
				entry.Params = append(entry.Params, ParamInfo{Name: p.Name, Type: p.Type})
			}
		}
		out = append(out, entry)
	}
	return out
}

// CrossDefResponse 是 FindDefCrossFile 的响应
type CrossDefResponse struct {
	Found   bool   `json:"found"`
	Name    string `json:"name,omitempty"`
	Kind    string `json:"kind,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
	Source  string `json:"source,omitempty"`
	PkgName string `json:"pkgName,omitempty"`
	Error   string `json:"error,omitempty"`
}

// FindDefCrossFile 在项目全局符号索引中查找给定名称的定义位置。
// 用于编辑器 F12 跨文件跳转（同文件找不到时调用）。
// 返回首个匹配（优先级：project > project-lib > global-lib）。
func (s *IDEService) FindDefCrossFile(projectPath, word string) CrossDefResponse {
	if word == "" {
		return CrossDefResponse{}
	}
	resp := s.ListAllSymbols(projectPath)
	if resp.Error != "" {
		return CrossDefResponse{Error: resp.Error}
	}
	// 优先级：project > project-lib > global-lib
	priority := map[string]int{"project": 0, "project-lib": 1, "global-lib": 2}
	best := -1
	var found SymbolEntry
	for _, sym := range resp.Symbols {
		if sym.Name != word {
			continue
		}
		// 只跳函数/方法/类型/常量/变量（不跳参数/局部变量）
		if sym.Kind != "function" && sym.Kind != "method" && sym.Kind != "type" && sym.Kind != "const" && sym.Kind != "var" {
			continue
		}
		p := priority[sym.Source]
		if best == -1 || p < best {
			best = p
			found = sym
		}
	}
	if best == -1 {
		return CrossDefResponse{Found: false}
	}
	return CrossDefResponse{
		Found:   true,
		Name:    found.Name,
		Kind:    found.Kind,
		File:    found.File,
		Line:    found.Line,
		Col:     found.Col,
		Source:  found.Source,
		PkgName: found.PkgName,
	}
}

// CrossRefEntry 是单个跨文件引用位置
type CrossRefEntry struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Length  int    `json:"length"`
	IsDef   bool   `json:"isDef"`   // 是否为定义处
	Source  string `json:"source"`  // "project" / "project-lib" / "global-lib"
	PkgName string `json:"pkgName,omitempty"`
}

// CrossRefsResponse 是 FindRefsCrossFile 的响应
type CrossRefsResponse struct {
	Refs  []CrossRefEntry `json:"refs,omitempty"`
	Error string          `json:"error,omitempty"`
}

// FindRefsCrossFile 在项目全局符号索引中查找给定名称的所有引用位置。
// 跨文件搜索：扫描所有 .eg 文件源码，返回包含该名称的所有位置（行号）。
func (s *IDEService) FindRefsCrossFile(projectPath, word string) CrossRefsResponse {
	if word == "" {
		return CrossRefsResponse{}
	}
	resp := s.ListAllSymbols(projectPath)
	if resp.Error != "" {
		return CrossRefsResponse{Error: resp.Error}
	}

	var out []CrossRefEntry

	// 1. 添加定义处（来自符号索引）
	for _, sym := range resp.Symbols {
		if sym.Name == word {
			out = append(out, CrossRefEntry{
				File:    sym.File,
				Line:    sym.Line,
				Col:     sym.Col,
				Length:  len(word),
				IsDef:   true,
				Source:  sym.Source,
				PkgName: sym.PkgName,
			})
		}
	}

	// 2. 扫描所有 .eg 文件源码，找引用位置（按行匹配，简单实现）
	// 收集所有文件路径（去重）
	fileSet := make(map[string]bool)
	for _, sym := range resp.Symbols {
		fileSet[sym.File] = true
	}
	// 加上项目内所有 .eg 文件（即使无符号也要扫描引用）
	if projectPath != "" {
		s.collectAllEgFiles(projectPath, fileSet)
	}

	for filePath := range fileSet {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			// 简单子串匹配 + 列定位（不区分定义/引用，已由步骤1标记定义处）
			col := 1
			for {
				idx := strings.Index(line[col-1:], word)
				if idx < 0 {
					break
				}
				actualCol := col + idx
				// 检查前后字符是否为单词边界（避免部分匹配）
				if isWordBoundary(line, actualCol-1, actualCol+len(word)-1) {
					out = append(out, CrossRefEntry{
						File:   filePath,
						Line:   i + 1,
						Col:    actualCol,
						Length: len(word),
						IsDef:  false,
						Source: fileSourceKind(filePath, projectPath, s.GetGlobalLibsDir()),
					})
				}
				col = actualCol + len(word)
			}
		}
	}

	return CrossRefsResponse{Refs: out}
}

// collectAllEgFiles 递归收集 dir 下所有 .eg 文件路径到 fileSet
func (s *IDEService) collectAllEgFiles(dir string, fileSet map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if name == "bin" || name == ".git" || name == "node_modules" || name == "libs" || name == "assets" || name == "native" {
				continue
			}
			s.collectAllEgFiles(filepath.Join(dir, name), fileSet)
			continue
		}
		if strings.HasSuffix(name, ".eg") {
			fileSet[filepath.Join(dir, name)] = true
		}
	}
}

// isWordBoundary 判断 line[start:end]（0-based，end exclusive）前后是否为单词边界
func isWordBoundary(line string, start, end int) bool {
	if start > 0 {
		prev := line[start-1]
		if isIdentByte(prev) {
			return false
		}
	}
	if end < len(line) {
		next := line[end]
		if isIdentByte(next) {
			return false
		}
	}
	return true
}

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b >= 0x80
}

// fileSourceKind 根据文件路径推断来源类型
func fileSourceKind(filePath, projectPath, globalLibsDir string) string {
	if projectPath != "" && strings.HasPrefix(filePath, projectPath) {
		if strings.Contains(filePath, string(filepath.Separator)+"libs"+string(filepath.Separator)) {
			return "project-lib"
		}
		return "project"
	}
	if globalLibsDir != "" && strings.HasPrefix(filePath, globalLibsDir) {
		return "global-lib"
	}
	return "unknown"
}
