// symbols.go 实现单文件符号索引 API（基于 AST，供编辑器 F12/Shift+F12/F2/hover 使用）。
//
// 设计：
//   - 前端把当前文件源码传过来（无状态），后端用 AST 解析器分析后返回位置信息
//   - 所有位置都是 1-based 行号和列号，与 Monaco 编辑器一致
//   - 错误容忍：解析失败时返回空数组 + Error 字段，不抛异常
//
// 从 compile.go 拆分（v0.6.12 单文件不超过 500 行规约）。

package app

import (
	"fmt"
	"strings"

	"egou/internal/transpiler"
)

// SymbolInfo 是符号信息的 JSON 序列化形式（用于编辑器大纲/跳转列表）
type SymbolInfo struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"` // function/method/type/const/var
	Line       int         `json:"line"` // 定义起始行（1-based）
	Col        int         `json:"col"`  // 定义起始列（1-based）
	EndLine    int         `json:"endLine"`
	EndCol     int         `json:"endCol"`
	Params     []ParamInfo `json:"params,omitempty"`
	ReturnType string      `json:"returnType,omitempty"`
	Fields     []FieldInfo `json:"fields,omitempty"`
	Receiver   string      `json:"receiver,omitempty"` // 方法接收者（如 "(p *Point)"）
}

// ParamInfo 是函数参数信息
type ParamInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// FieldInfo 是类型字段信息
type FieldInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// SymbolListResponse 是 ListSymbols 的响应
type SymbolListResponse struct {
	Symbols []SymbolInfo `json:"symbols,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// DefResponse 是 FindDefinition 的响应
// Line=0 表示未找到定义
type DefResponse struct {
	Line  int    `json:"line,omitempty"`
	Col   int    `json:"col,omitempty"`
	Name  string `json:"name,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Error string `json:"error,omitempty"`
}

// RefInfo 是单个引用位置
type RefInfo struct {
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Length int    `json:"length"` // 标识符长度（字节，前端按字符截取）
	Kind   string `json:"kind"`   // "definition" / "reference"
}

// RefsResponse 是 FindReferences 的响应
type RefsResponse struct {
	Refs  []RefInfo `json:"refs,omitempty"`
	Error string    `json:"error,omitempty"`
}

// ListSymbols 解析源码，返回所有顶层符号（函数/方法/类型/常量/包级变量）。
// 用于编辑器右侧大纲面板。
func (s *IDEService) ListSymbols(source string) SymbolListResponse {
	file, errs := transpiler.Parse(source)
	if file == nil {
		return SymbolListResponse{Error: joinParseErrors(errs)}
	}
	syms := transpiler.CollectSymbols(file)
	out := make([]SymbolInfo, 0, len(syms))
	for _, sym := range syms {
		info := SymbolInfo{
			Name:    sym.Name,
			Kind:    transpiler.SymbolNameKind(sym.Kind),
			Line:    sym.Pos.Line,
			Col:     sym.Pos.Col,
			EndLine: sym.EndPos.Line,
			EndCol:  sym.EndPos.Col,
		}
		if sym.Params != nil {
			info.Params = make([]ParamInfo, 0, len(sym.Params))
			for _, p := range sym.Params {
				info.Params = append(info.Params, ParamInfo{Name: p.Name, Type: p.Type})
			}
		}
		info.ReturnType = sym.ReturnType
		if sym.Fields != nil {
			info.Fields = make([]FieldInfo, 0, len(sym.Fields))
			for _, f := range sym.Fields {
				info.Fields = append(info.Fields, FieldInfo{Name: f.Name, Type: f.Type})
			}
		}
		out = append(out, info)
	}
	return SymbolListResponse{Symbols: out}
}

// FindDefinition 查找给定名称的定义位置。
// 用于编辑器 F12 跳转定义。
// 查找顺序：函数/方法/类型/常量/包级变量 > 参数 > 局部变量
// 未找到时 Line=0，前端应回退到跨文件 .elib 搜索或显示提示。
func (s *IDEService) FindDefinition(source string, word string) DefResponse {
	if word == "" {
		return DefResponse{}
	}
	file, errs := transpiler.Parse(source)
	if file == nil {
		return DefResponse{Error: joinParseErrors(errs)}
	}
	pos, ok := transpiler.FindDefinition(file, word)
	if !ok {
		return DefResponse{}
	}
	// 推断符号类型
	kind := "var"
	for _, sym := range transpiler.CollectSymbols(file) {
		if sym.Pos == pos {
			kind = transpiler.SymbolNameKind(sym.Kind)
			break
		}
	}
	return DefResponse{Line: pos.Line, Col: pos.Col, Name: word, Kind: kind}
}

// FindReferences 查找给定名称的所有引用位置（含定义处）。
// 用于编辑器 Shift+F12 查找引用 + F2 重命名（多光标编辑）。
func (s *IDEService) FindReferences(source string, word string) RefsResponse {
	if word == "" {
		return RefsResponse{}
	}
	file, errs := transpiler.Parse(source)
	if file == nil {
		return RefsResponse{Error: joinParseErrors(errs)}
	}
	refs := transpiler.FindIdentRefs(file, word)
	out := make([]RefInfo, 0, len(refs))
	for _, r := range refs {
		out = append(out, RefInfo{
			Line:   r.Pos.Line,
			Col:    r.Pos.Col,
			Length: len(word),
			Kind:   r.Kind,
		})
	}
	return RefsResponse{Refs: out}
}

// joinParseErrors 把解析错误列表拼成单个字符串（前 5 条）
func joinParseErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	var sb strings.Builder
	limit := len(errs)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(errs[i].Error())
	}
	if len(errs) > 5 {
		fmt.Fprintf(&sb, "\n... 还有 %d 条错误", len(errs)-5)
	}
	return sb.String()
}

// DiagInfo 是单个诊断信息（语法错误/警告）
type DiagInfo struct {
	Line     int    `json:"line"`               // 1-based 行号
	Col      int    `json:"col"`                // 1-based 列号
	EndLine  int    `json:"endLine,omitempty"`  // 结束行（默认同 Line）
	EndCol   int    `json:"endCol,omitempty"`   // 结束列（默认 Col + 1）
	Severity string `json:"severity"`           // "error" / "warning" / "info"
	Message  string `json:"message"`            // 错误消息
	Source   string `json:"source,omitempty"`   // 来源标识（"eg-parser"）
}

// DiagResponse 是 GetDiagnostics 的响应
type DiagResponse struct {
	Diagnostics []DiagInfo `json:"diagnostics,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// GetDiagnostics 解析源码，返回语法错误诊断信息。
// 用于编辑器实时展示红色波浪线（Monaco markers）。
// 错误容忍：解析失败也返回已收集的错误（parser 错误恢复机制）。
func (s *IDEService) GetDiagnostics(source string) DiagResponse {
	_, errs := transpiler.Parse(source)
	if len(errs) == 0 {
		return DiagResponse{}
	}
	out := make([]DiagInfo, 0, len(errs))
	for _, e := range errs {
		pe, ok := e.(*transpiler.ParseError)
		if !ok {
			out = append(out, DiagInfo{
				Line:     1,
				Col:      1,
				Severity: "error",
				Message:  e.Error(),
				Source:   "eg-parser",
			})
			continue
		}
		out = append(out, DiagInfo{
			Line:     pe.Pos.Line,
			Col:      pe.Pos.Col,
			EndLine:  pe.Pos.Line,
			EndCol:   pe.Pos.Col + 1,
			Severity: "error",
			Message:  pe.Msg,
			Source:   "eg-parser",
		})
	}
	return DiagResponse{Diagnostics: out}
}

// FormatCodeResponse 是 FormatCode 的响应
type FormatCodeResponse struct {
	Code  string `json:"code,omitempty"`
	Error string `json:"error,omitempty"`
}

// FormatCode 对传入的 Go 源码调用 go/format 进行 gofmt 格式化。
// 用于前端"查看 Go 代码"窗口对转译输出做标准格式化，
// 或未来编辑器内对原生 Go 块做格式化（Shift+Alt+F）。
func (s *IDEService) FormatCode(goSrc string) FormatCodeResponse {
	out, err := transpiler.FormatGoCode(goSrc)
	if err != nil {
		return FormatCodeResponse{Code: out, Error: err.Error()}
	}
	return FormatCodeResponse{Code: out}
}
