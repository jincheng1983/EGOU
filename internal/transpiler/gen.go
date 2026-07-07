// Package transpiler — AST → Go 代码生成器（实验性）
//
// 设计目标：
//   - 基于 AST 生成 Go 源码，作为 transpiler.go 正则转译的并行通道
//   - 复用 transpiler.go 的 mapType / translateExpression 等辅助函数
//   - 不追求 100% 覆盖，先覆盖核心语法，未覆盖的节点回退到正则转译
//   - 为未来 transpiler.go 完全迁移到 AST 打基础
//
// 入口：
//
//	file, errs := Parse(src)
//	goSrc := GenerateGo(file)  // 生成 Go 代码
//
// 与 Transpile 的关系：
//   - Transpile：现有正则+字符串替换，稳定但无结构
//   - GenerateGo：基于 AST，结构化但覆盖率有限
//   - TranspileAST：先试 AST，失败回退到 Transpile
package transpiler

import (
	"fmt"
	"go/format"
	"strings"
)

// GenerateGo 遍历 AST 生成 Go 源码。
// 输入：Parse 返回的 *File
// 输出：经过 go/format 格式化的 Go 源码字符串
// 若 go/format 失败（例如生成的代码有语法错误），回退返回原始未格式化代码 + 错误。
func GenerateGo(file *File) (string, error) {
	if file == nil {
		return "", fmt.Errorf("file is nil")
	}

	var g codeGen
	g.writeFile(file)
	raw := g.buf.String()
	// 集成 go/format 对生成的 Go 代码进行格式化（gofmt 标准）
	formatted, err := format.Source([]byte(raw))
	if err != nil {
		// 格式化失败时返回原始代码 + 错误，调用方可决定是否使用
		return raw, fmt.Errorf("gofmt 格式化失败: %w", err)
	}
	return string(formatted), nil
}

// TranspileAST 是基于 AST 的转译入口。
// 策略：先 Parse，若有致命错误（无 File）则返回错误；
// 解析成功后 GenerateGo 生成 Go 代码（保留中文别名），
// 然后做支持库命令后处理（与 Transpile 正则通道对齐）：
//   1. translateSupportCalls：中文别名 → runtimeUIService.English / 英文键
//   2. 补入支持库 imports 到 import 块
//   3. 末尾注入支持库函数定义（goDef）
//   4. go/format 格式化
// 调用方（如 IDEService）可选择回退到正则 Transpile。
func TranspileAST(src string) (string, error) {
	file, errs := Parse(src)
	if file == nil {
		return "", fmt.Errorf("AST 解析失败: %v", errs)
	}
	out, err := GenerateGo(file)
	if err != nil {
		if out == "" {
			return "", err
		}
	}

	// 后处理 1：支持库命令替换（中文别名 → 英文键/runtimeUIService.xxx）
	out = translateSupportCalls(out)

	// 后处理 2 & 3：补入支持库 imports + 注入 goDef
	out = injectSupportDefsAndImports(out)

	// 后处理 4：gofmt 格式化（translateSupportCalls 可能改变缩进）
	if formatted, ferr := format.Source([]byte(out)); ferr == nil {
		out = string(formatted)
	}

	// 如果有解析错误但仍生成了 AST，附加警告注释
	if len(errs) > 0 {
		var sb strings.Builder
		sb.WriteString("// AST 解析警告（已尝试恢复）:\n")
		limit := len(errs)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			sb.WriteString("//   " + errs[i].Error() + "\n")
		}
		if len(errs) > 5 {
			fmt.Fprintf(&sb, "//   ... 还有 %d 条\n", len(errs)-5)
		}
		sb.WriteString(out)
		return sb.String(), nil
	}
	return out, nil
}

// injectSupportDefsAndImports 在 AST 生成的 Go 代码中补入支持库 imports 和 goDef
// 输入是已做 translateSupportCalls 的 Go 源码（含中文别名已被替换为英文键）
// 流程：
//   1. detectSupportCommands 扫描用过的命令
//   2. collectSupportImports 收集需要的 imports
//   3. 在 import 块末尾补入支持库 imports（去重）
//   4. 在文件末尾追加 goDef
func injectSupportDefsAndImports(src string) string {
	usedCmds := detectSupportCommands(src)
	if len(usedCmds) == 0 {
		return src
	}

	// 收集支持库 imports
	supportImports := collectSupportImports(usedCmds)

	out := src

	// 补入 imports：如果已有 import 块，在 ')' 之前插入；否则新建一个 import 块
	if len(supportImports) > 0 {
		lines := strings.Split(out, "\n")
		// 查找 import 块的 ')' 行
		importCloseIdx := -1
		hasImportBlock := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == ")" {
				// 检查前面是否有 import (
				for j := i - 1; j >= 0; j-- {
					t := strings.TrimSpace(lines[j])
					if t == "import (" {
						importCloseIdx = i
						hasImportBlock = true
						break
					}
					if t != "" && !strings.HasPrefix(t, "//") {
						break
					}
				}
				if hasImportBlock {
					break
				}
			}
		}

		// 收集已有的 import 路径，去重
		seen := make(map[string]bool)
		for _, line := range lines {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "\"") && strings.HasSuffix(t, "\"") {
				seen[strings.Trim(t, "\"")] = true
			}
		}

		var newImports []string
		for _, imp := range supportImports {
			if !seen[imp] {
				newImports = append(newImports, imp)
			}
		}

		if len(newImports) > 0 {
			var sb strings.Builder
			for _, imp := range newImports {
				sb.WriteString("\t\"" + imp + "\"\n")
			}
			insertStr := sb.String()

			if importCloseIdx >= 0 {
				// 在 ')' 之前插入
				result := make([]string, 0, len(lines)+len(newImports))
				result = append(result, lines[:importCloseIdx]...)
				result = append(result, insertStr[:len(insertStr)-1]) // 去掉末尾 \n
				result = append(result, lines[importCloseIdx:]...)
				out = strings.Join(result, "\n")
			} else {
				// 没有 import 块，在 package 行之后插入
				result := make([]string, 0, len(lines)+len(newImports)+3)
				pkgIdx := -1
				for i, line := range lines {
					if strings.HasPrefix(line, "package ") {
						pkgIdx = i
						break
					}
				}
				if pkgIdx >= 0 {
					result = append(result, lines[:pkgIdx+1]...)
					result = append(result, "")
					result = append(result, "import (")
					for _, imp := range newImports {
						result = append(result, "\t\""+imp+"\"")
					}
					result = append(result, ")", "")
					result = append(result, lines[pkgIdx+1:]...)
					out = strings.Join(result, "\n")
				}
			}
		}
	}

	// 注入 goDef
	var goDefBuf strings.Builder
	goDefBuf.WriteString("\n// 支持库函数\n")
	for alias := range usedCmds {
		cmd := supportLibrary[alias]
		if cmd.goDef == "" {
			continue
		}
		goDefBuf.WriteString(cmd.goDef)
		goDefBuf.WriteString("\n")
	}
	if goDefBuf.Len() > len("\n// 支持库函数\n") {
		out = out + goDefBuf.String()
	}

	return out
}

// FormatGoCode 使用 go/format 对任意 Go 源码进行 gofmt 格式化。
// 用于 IDEService.FormatCode API：前端可调用对转译后的 Go 代码做格式化预览。
// 失败时返回原始 src + error，调用方可决定是否使用。
func FormatGoCode(src string) (string, error) {
	if strings.TrimSpace(src) == "" {
		return src, nil
	}
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return src, err
	}
	return string(formatted), nil
}

// codeGen 是代码生成器内部状态
type codeGen struct {
	buf            strings.Builder
	indent         int          // 当前缩进级别
	imports        []string     // 收集到的导入路径
	packageNm      string       // 包名（来自 # 程序集）
	hasMainImpl    bool         // 是否已生成 mainImpl（用户定义了"主函数"）
	eventHandlers  []string     // 事件处理函数注册语句（供 registerHandlersImpl 使用）
	fileMarkers    []FileMarker // #@eg-file 标记（多文件合并时用于生成 //line 指令）
}

// writeFile 生成文件级结构：package + imports + decls
// 注意：此处不处理支持库命令替换和 goDef 注入，由 TranspileAST 后处理完成
func (g *codeGen) writeFile(file *File) {
	// 0. 记录 #@eg-file 标记，供 writeLineDirective 查询
	g.fileMarkers = file.FileMarkers

	// 1. 包名
	g.packageNm = file.Package
	if g.packageNm == "" {
		g.packageNm = "main"
	}
	fmt.Fprintf(&g.buf, "package %s\n\n", g.packageNm)

	// 2. 导入（仅用户 imports，支持库 imports 由 TranspileAST 后处理补入）
	if len(file.Imports) > 0 {
		g.buf.WriteString("import (\n")
		for _, imp := range file.Imports {
			if imp.Alias != "" {
				fmt.Fprintf(&g.buf, "\t%s %s\n", imp.Alias, imp.Path)
			} else {
				fmt.Fprintf(&g.buf, "\t%s\n", imp.Path)
			}
		}
		g.buf.WriteString(")\n\n")
	}

	// 3. 顶层声明（多文件合并时在每个声明前插入 //line 指令，让编译错误指向 .eg 源码行）
	for _, d := range file.Decls {
		g.writeLineDirective(d.Position().Line)
		g.writeDecl(d)
		g.buf.WriteString("\n")
	}

	// 4. 顶层可执行语句自动包装到 init() 函数
	//    Go 不允许函数外有可执行语句，转译器自动包装到 init() 函数中
	//    init() 会在 mainImpl 之前自动执行，语义等同脚本式顶层语句（如 Python）
	//    （与正则通道 transpiler.go:399-409 对齐）
	if len(file.TopLevelStmts) > 0 {
		g.buf.WriteString("\n// 顶层可执行语句自动包装到 init() 函数（Go 不允许函数外可执行语句）\n")
		g.buf.WriteString("// init() 在 mainImpl 之前自动执行，语义等同脚本式顶层语句\n")
		g.buf.WriteString("func init() {\n")
		g.indent = 1
		for _, s := range file.TopLevelStmts {
			g.writeStmt(s)
		}
		g.indent = 0
		g.buf.WriteString("}\n")
	}

	// 5. 生成事件处理函数注册表 registerHandlersImpl
	//    Wails 运行时模板的 main.go 在执行主函数前调用它
	//    （与正则通道 transpiler.go:686-692 对齐）
	g.buf.WriteString("\nfunc registerHandlersImpl() {\n")
	for _, h := range g.eventHandlers {
		fmt.Fprintf(&g.buf, "\t%s\n", h)
	}
	g.buf.WriteString("}\n")

	// 6. 如果用户代码没有定义"主函数"（即没有生成 mainImpl），补充空占位符
	//    避免 main.go 中的 runtimeUIService.SetMainFuncs(registerHandlersImpl, mainImpl) 编译失败
	//    （与正则通道 transpiler.go:694-707 对齐）
	if !g.hasMainImpl {
		g.buf.WriteString("\n// 空主函数占位符：用户 .eg 代码未定义\"主函数\"时由转译器自动补充\n")
		g.buf.WriteString("func mainImpl() {}\n")
	}
}

// writeLineDirective 在声明前插入 //line 指令，让 Go 编译器错误指向 .eg 源码行。
// 仅在多文件合并场景（存在 #@eg-file 标记）下生效；单文件无标记时不插入，保持输出干净。
// 文件内行号 = globalLine - marker.GlobalLine（#@eg-file 标记行本身不计入文件内容，下一行才是第 1 行）。
// 声明在所有标记之前（主源码）用默认文件名 "源码.eg"，与正则通道 transpiler.go:361-363 对齐。
func (g *codeGen) writeLineDirective(globalLine int) {
	if len(g.fileMarkers) == 0 {
		return
	}
	// 找到声明所属的源文件：最大的 GlobalLine <= globalLine
	fileName := "源码.eg"
	fileLocalLine := globalLine
	for _, m := range g.fileMarkers {
		if m.GlobalLine <= globalLine {
			fileName = m.FileName
			fileLocalLine = globalLine - m.GlobalLine
		}
	}
	fmt.Fprintf(&g.buf, "//line \"%s\":%d\n", fileName, fileLocalLine)
}

// writeDecl 分发顶层声明
func (g *codeGen) writeDecl(d Decl) {
	switch n := d.(type) {
	case *ImportDecl:
		// 已在 writeFile 处理，这里不重复
	case *FuncDecl:
		g.writeFuncDecl(n)
	case *MethodDecl:
		g.writeMethodDecl(n)
	case *TypeDecl:
		g.writeTypeDecl(n)
	case *TypeAliasDecl:
		// 类型别名：type X = Y（Go 1.9+）
		fmt.Fprintf(&g.buf, "type %s = %s\n", n.Name, mapType(n.Underlying))
	case *ConstDecl:
		g.writeConstDecl(n)
	case *ConstBlockDecl:
		g.writeConstBlockDecl(n)
	case *EnumDecl:
		g.writeEnumDecl(n)
	case *VarDecl:
		g.writeVarDecl(n)
	case *VarBlockDecl:
		g.writeVarBlockDecl(n)
	case *EmbedBlock:
		// 嵌入原生 Go 代码原样输出
		g.buf.WriteString(n.Content)
		g.buf.WriteString("\n")
	}
}

// writeReturnTypes 生成函数/方法的返回类型部分
// 空切片：不输出；单元素：输出 " t"；多元素：输出 " (t1, t2)"
func (g *codeGen) writeReturnTypes(types []string) {
	if len(types) == 0 {
		return
	}
	if len(types) == 1 {
		fmt.Fprintf(&g.buf, " %s", mapType(types[0]))
		return
	}
	g.buf.WriteString(" (")
	for i, t := range types {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString(mapType(t))
	}
	g.buf.WriteString(")")
}

// writeParams 输出参数列表（支持可变参数 ...Type）
func (g *codeGen) writeParams(params []*ParamDecl) {
	for i, p := range params {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		if p.Variadic {
			fmt.Fprintf(&g.buf, "%s ...%s", p.Name, mapType(p.Type))
		} else {
			fmt.Fprintf(&g.buf, "%s %s", p.Name, mapType(p.Type))
		}
	}
}

// writeFuncDecl 生成函数声明
// "主函数" 自动映射为 "mainImpl"，避免与 Wails 运行时模板的 main 冲突
// （正则通道在 translateFunctionSignature 做同样映射，transpiler.go:804-806）
func (g *codeGen) writeFuncDecl(d *FuncDecl) {
	// func Name(params) returnType { body }
	goName := d.Name
	if d.Name == "主函数" {
		goName = "mainImpl"
		g.hasMainImpl = true
	}
	// 事件处理函数注册：函数名形如 "按钮1_被单击" → 注册到 registerHandlersImpl
	if comp, evt, ok := parseEventHandlerName(d.Name); ok {
		g.eventHandlers = append(g.eventHandlers, fmt.Sprintf(`runtimeUIService.RegisterEvent("%s", "%s", %s)`, comp, evt, d.Name))
	}
	fmt.Fprintf(&g.buf, "func %s(", goName)
	g.writeParams(d.Params)
	g.buf.WriteString(")")
	g.writeReturnTypes(d.ReturnTypes)
	g.buf.WriteString(" {\n")
	if d.Body != nil {
		g.indent = 1
		g.writeBlockStmt(d.Body)
	}
	g.buf.WriteString("}\n")
}

// writeMethodDecl 生成方法声明
func (g *codeGen) writeMethodDecl(d *MethodDecl) {
	// func (recv Type) Name(params) returnType { body }
	if d.Receiver != nil {
		fmt.Fprintf(&g.buf, "func (%s %s) %s(", d.Receiver.Name, mapType(d.Receiver.Type), d.Name)
	} else {
		fmt.Fprintf(&g.buf, "func %s(", d.Name)
	}
	g.writeParams(d.Params)
	g.buf.WriteString(")")
	g.writeReturnTypes(d.ReturnTypes)
	g.buf.WriteString(" {\n")
	if d.Body != nil {
		g.indent = 1
		g.writeBlockStmt(d.Body)
	}
	g.buf.WriteString("}\n")
}

// writeTypeDecl 生成类型声明
func (g *codeGen) writeTypeDecl(d *TypeDecl) {
	if d.Kind == "接口" {
		// type Name interface { embeddedInterfaces + methods }
		fmt.Fprintf(&g.buf, "type %s interface {\n", d.Name)
		// 先输出嵌入的接口（Go 接口组合）
		for _, ei := range d.EmbeddedInterfaces {
			fmt.Fprintf(&g.buf, "\t%s\n", ei.Name)
		}
		// 再输出方法签名
		for _, m := range d.Methods {
			g.buf.WriteString("\t")
			g.writeMethodSig(m)
			g.buf.WriteString("\n")
		}
		g.buf.WriteString("}\n")
		return
	}
	// type Name struct { fields }
	fmt.Fprintf(&g.buf, "type %s struct {\n", d.Name)
	for _, f := range d.Fields {
		if f.Embedded {
			// 嵌入字段：只输出类型名（Go 嵌入字段语法）
			fmt.Fprintf(&g.buf, "\t%s\n", mapType(f.Type))
		} else {
			fmt.Fprintf(&g.buf, "\t%s %s\n", f.Name, mapType(f.Type))
		}
	}
	g.buf.WriteString("}\n")
}

// writeMethodSig 输出接口方法签名：Name(params) returnTypes
func (g *codeGen) writeMethodSig(m *MethodSig) {
	fmt.Fprintf(&g.buf, "%s(", m.Name)
	g.writeParams(m.Params)
	g.buf.WriteString(")")
	g.writeReturnTypes(m.ReturnTypes)
}

// writeConstDecl 生成常量声明
func (g *codeGen) writeConstDecl(d *ConstDecl) {
	// const Name = value
	fmt.Fprintf(&g.buf, "const %s = ", d.Name)
	if d.Value != nil {
		g.writeExpr(d.Value)
	} else {
		g.buf.WriteString("nil")
	}
	g.buf.WriteString("\n")
}

// writeConstBlockDecl 生成多常量块（常量 ( ... ) → const ( ... )）
func (g *codeGen) writeConstBlockDecl(d *ConstBlockDecl) {
	g.buf.WriteString("const (\n")
	for _, item := range d.Items {
		g.buf.WriteString("\t")
		fmt.Fprintf(&g.buf, "%s = ", item.Name)
		if item.Value != nil {
			g.writeExpr(item.Value)
		} else {
			g.buf.WriteString("nil")
		}
		g.buf.WriteString("\n")
	}
	g.buf.WriteString(")\n")
}

// writeEnumDecl 生成枚举声明（转译为 Go const 块 + iota）
// 输出格式：
//   const (
//       Name1 = iota          // 首项带表达式
//       Name2                 // 后续省略，自动 iota +1
//       Name3
//   )
// 若首项无表达式，则全部用 iota（Go 省略表达式时自动延续上一行的表达式）
func (g *codeGen) writeEnumDecl(d *EnumDecl) {
	g.buf.WriteString("const (\n")
	for i, item := range d.Items {
		g.buf.WriteString("\t")
		if item.HasValue && item.Value != nil {
			// 显式表达式：Name = Expr
			fmt.Fprintf(&g.buf, "%s = ", item.Name)
			g.writeExpr(item.Value)
		} else {
			// 省略表达式：Go const 块中后续项省略表达式自动延续上一行
			// 但首项必须显式 iota，否则 Go 视为无类型常量错误
			if i == 0 {
				fmt.Fprintf(&g.buf, "%s = iota", item.Name)
			} else {
				fmt.Fprintf(&g.buf, "%s", item.Name)
			}
		}
		g.buf.WriteString("\n")
	}
	g.buf.WriteString(")\n")
}

// writeVarDecl 生成包级变量声明
func (g *codeGen) writeVarDecl(d *VarDecl) {
	// var Name Type 或 var Name = value
	fmt.Fprintf(&g.buf, "var %s ", d.Name)
	if d.Type != "" {
		g.buf.WriteString(mapType(d.Type))
	}
	if d.Value != nil {
		g.buf.WriteString(" = ")
		g.writeExpr(d.Value)
	}
	g.buf.WriteString("\n")
}

// writeVarBlockDecl 生成多变量块（变量 ( ... ) → var ( ... )）
// 块内项格式：Name Type / Name = value（Go 包级 var 块内不允许 := 短声明）
func (g *codeGen) writeVarBlockDecl(d *VarBlockDecl) {
	g.buf.WriteString("var (\n")
	for _, item := range d.Items {
		g.buf.WriteString("\t")
		fmt.Fprintf(&g.buf, "%s ", item.Name)
		if item.Type != "" {
			g.buf.WriteString(mapType(item.Type))
		}
		if item.Value != nil {
			g.buf.WriteString(" = ")
			g.writeExpr(item.Value)
		}
		g.buf.WriteString("\n")
	}
	g.buf.WriteString(")\n")
}

// writeBlockStmt 生成语句块
func (g *codeGen) writeBlockStmt(b *BlockStmt) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		g.writeIndent()
		g.writeStmt(s)
		g.buf.WriteString("\n")
	}
}

// writeIndent 写入当前缩进
func (g *codeGen) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("\t")
	}
}

// writeStmt 分发语句
func (g *codeGen) writeStmt(s Stmt) {
	switch n := s.(type) {
	case *IfStmt:
		g.writeIfStmt(n)
	case *ForStmt:
		g.writeForStmt(n)
	case *RangeStmt:
		g.writeRangeStmt(n)
	case *WhileStmt:
		g.writeWhileStmt(n)
	case *SwitchStmt:
		g.writeSwitchStmt(n)
	case *SelectStmt:
		g.writeSelectStmt(n)
	case *ReturnStmt:
		g.writeReturnStmt(n)
	case *BreakStmt:
		g.buf.WriteString("break")
		if n.Label != "" {
			g.buf.WriteString(" ")
			g.buf.WriteString(n.Label)
		}
	case *ContinueStmt:
		g.buf.WriteString("continue")
		if n.Label != "" {
			g.buf.WriteString(" ")
			g.buf.WriteString(n.Label)
		}
	case *LabeledStmt:
		// Label: Stmt（Go 标签语法）
		// Stmt 为 nil 时输出 "Label:" 单独一行（标签修饰空语句，用于 goto 跳转目标）
		fmt.Fprintf(&g.buf, "%s: ", n.Label)
		if n.Stmt != nil {
			g.writeStmt(n.Stmt)
		}
	case *GotoStmt:
		// goto Label（Go goto 语法）
		fmt.Fprintf(&g.buf, "goto %s", n.Label)
	case *FallthroughStmt:
		// fallthrough（switch case 末尾穿透到下一个 case 体）
		g.buf.WriteString("fallthrough")
	case *DeferStmt:
		g.buf.WriteString("defer ")
		g.writeExpr(n.Call)
	case *GoStmt:
		g.buf.WriteString("go ")
		g.writeExpr(n.Call)
	case *PanicStmt:
		// 抛出 expr → panic(expr)
		g.buf.WriteString("panic(")
		if n.X != nil {
			g.writeExpr(n.X)
		}
		g.buf.WriteString(")")
	case *ExprStmt:
		g.writeExpr(n.X)
	case *AssignStmt:
		g.writeExpr(n.Lhs)
		// 保留复合赋值运算符（+=/-=/*=//=），全角 ＝ 转为 ASCII =
		op := n.Op
		if op == "＝" {
			op = "="
		}
		g.buf.WriteString(" ")
		g.buf.WriteString(op)
		g.buf.WriteString(" ")
		g.writeExpr(n.Rhs)
	case *IncDecStmt:
		g.writeExpr(n.X)
		g.buf.WriteString(n.Tok) // ++ 或 --
	case *MultiAssignStmt:
		// 多变量赋值/短声明：a, b ＝ c, d 或 a, b := c, d
		for i, e := range n.Lhs {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			g.writeExpr(e)
		}
		g.buf.WriteString(" ")
		// 保留运算符，全角 ＝ 转为 ASCII =
		op := n.Op
		if op == "＝" {
			op = "="
		}
		g.buf.WriteString(op)
		g.buf.WriteString(" ")
		for i, e := range n.Rhs {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			g.writeExpr(e)
		}
	case *LocalVarDeclStmt:
		// 局部变量 名字 类型 或 局部变量 名字 ＝ 值
		// 通道变量声明（Type 以 "通道 " 开头）：var Name chan T = Value
		if n.Value != nil {
			if strings.HasPrefix(n.Type, "通道 ") {
				elemType := strings.TrimSpace(strings.TrimPrefix(n.Type, "通道"))
				fmt.Fprintf(&g.buf, "var %s chan %s = ", n.Name, mapType(elemType))
			} else {
				fmt.Fprintf(&g.buf, "%s := ", n.Name)
			}
			g.writeExpr(n.Value)
		} else if n.Type != "" {
			fmt.Fprintf(&g.buf, "var %s %s", n.Name, mapType(n.Type))
		} else {
			g.buf.WriteString(n.Name)
		}
	case *BlockStmt:
		g.buf.WriteString("{\n")
		g.indent++
		g.writeBlockStmt(n)
		g.indent--
		g.writeIndent()
		g.buf.WriteString("}")
	}
}

// writeIfStmt 生成如果语句
// 识别"否则如果"简写：Else 块只含一个 IfStmt 时生成 "} else if ... {"
// 否则生成标准的 "} else { ... }"
func (g *codeGen) writeIfStmt(s *IfStmt) {
	g.writeIfHead(s)
	g.writeIfTail(s)
}

// writeIfHead 生成 "if cond {" + Then 块
func (g *codeGen) writeIfHead(s *IfStmt) {
	g.buf.WriteString("if ")
	g.writeExpr(s.Cond)
	g.buf.WriteString(" {\n")
	g.indent++
	if s.Then != nil {
		g.writeBlockStmt(s.Then)
	}
	g.indent--
	g.writeIndent()
}

// writeIfTail 生成 else 部分，识别嵌套 IfStmt 生成 else if 链
func (g *codeGen) writeIfTail(s *IfStmt) {
	if s.Else == nil {
		g.buf.WriteString("}")
		return
	}
	// 检查 Else 块是否只含一个 IfStmt（即"否则如果"简写）
	if len(s.Else.Stmts) == 1 {
		if nestedIf, ok := s.Else.Stmts[0].(*IfStmt); ok {
			g.buf.WriteString("} else ")
			g.writeIfHead(nestedIf)
			g.writeIfTail(nestedIf)
			return
		}
	}
	g.buf.WriteString("} else {\n")
	g.indent++
	g.writeBlockStmt(s.Else)
	g.indent--
	g.writeIndent()
	g.buf.WriteString("}")
}

// writeForStmt 生成循环语句
// 输出形式：
//   - 三段式（Init 或 Post 非 nil）：for init; cond; post {
//   - 单 cond（Init/Post 均为 nil，Cond 非 nil）：for cond {
//   - 无限循环（三者均为 nil）：for {
func (g *codeGen) writeForStmt(s *ForStmt) {
	g.buf.WriteString("for ")
	if s.Init != nil || s.Post != nil {
		// 三段式：分号必须保留，缺失的部分留空（对应 Go for init; cond; post {）
		if s.Init != nil {
			g.writeStmt(s.Init)
		}
		g.buf.WriteString("; ")
		if s.Cond != nil {
			g.writeExpr(s.Cond)
		}
		g.buf.WriteString("; ")
		if s.Post != nil {
			g.writeStmt(s.Post)
		}
	} else {
		// 单 cond 或无限循环
		if s.Cond != nil {
			g.writeExpr(s.Cond)
		}
	}
	g.buf.WriteString(" {\n")
	g.indent++
	if s.Body != nil {
		g.writeBlockStmt(s.Body)
	}
	g.indent--
	g.writeIndent()
	g.buf.WriteString("}")
}

// writeRangeStmt 生成范围循环
// 三种形式：
//   - 双变量：for k, v := range x
//   - 单变量（仅值）：for _, v := range x（Key 为空，补 _）
//   - 无变量：for range x
func (g *codeGen) writeRangeStmt(s *RangeStmt) {
	g.buf.WriteString("for ")
	if s.Key != "" && s.Value != "" {
		// 双变量
		g.buf.WriteString(s.Key)
		g.buf.WriteString(", ")
		g.buf.WriteString(s.Value)
		g.buf.WriteString(" := range ")
	} else if s.Key != "" {
		// 只有 Key（不常见，按 Go 习惯生成 for k := range）
		g.buf.WriteString(s.Key)
		g.buf.WriteString(" := range ")
	} else if s.Value != "" {
		// 只有 Value（单变量 range，Key 补 _）
		g.buf.WriteString("_, ")
		g.buf.WriteString(s.Value)
		g.buf.WriteString(" := range ")
	} else {
		// 无变量
		g.buf.WriteString("range ")
	}
	g.writeExpr(s.X)
	g.buf.WriteString(" {\n")
	g.indent++
	if s.Body != nil {
		g.writeBlockStmt(s.Body)
	}
	g.indent--
	g.writeIndent()
	g.buf.WriteString("}")
}

// writeWhileStmt 生成判断循环（对应 Go 的 for cond { }）
func (g *codeGen) writeWhileStmt(s *WhileStmt) {
	g.buf.WriteString("for ")
	g.writeExpr(s.Cond)
	g.buf.WriteString(" {\n")
	g.indent++
	if s.Body != nil {
		g.writeBlockStmt(s.Body)
	}
	g.indent--
	g.writeIndent()
	g.buf.WriteString("}")
}

// writeSwitchStmt 生成选择语句
// 支持三种形式（v53 扩展）：
//   1. switch X { ... }             — 普通 switch
//   2. switch { ... }               — 无 X（X 为 nil）
//   3. switch x := y.(type) { ... } — type switch（TypeVar 非空）
func (g *codeGen) writeSwitchStmt(s *SwitchStmt) {
	g.buf.WriteString("switch ")
	if s.TypeVar != "" {
		// type switch: switch x := y.(type) { ... }
		// s.X 是 TypeAssertExpr{Type: ""}，其 X 字段是被断言的表达式 y
		g.buf.WriteString(s.TypeVar)
		g.buf.WriteString(" := ")
		if tae, ok := s.X.(*TypeAssertExpr); ok && tae.Type == "" {
			if tae.X != nil {
				g.writeExpr(tae.X)
			}
		} else if s.X != nil {
			// 容错：直接输出表达式
			g.writeExpr(s.X)
		}
		g.buf.WriteString(".(type)")
	} else {
		if s.X != nil {
			g.writeExpr(s.X)
		}
	}
	g.buf.WriteString(" {\n")
	for _, c := range s.Cases {
		g.writeIndent()
		if c.Values == nil {
			g.buf.WriteString("default:\n")
		} else {
			g.buf.WriteString("case ")
			for i, v := range c.Values {
				if i > 0 {
					g.buf.WriteString(", ")
				}
				// type switch case：值是类型名（Ident.Name 是中文类型字符串），需 mapType 转换
				if s.TypeVar != "" {
					if id, ok := v.(*Ident); ok {
						g.buf.WriteString(mapType(id.Name))
						continue
					}
				}
				g.writeExpr(v)
			}
			g.buf.WriteString(":\n")
		}
		g.indent++
		if c.Body != nil {
			g.writeBlockStmt(c.Body)
		}
		g.indent--
	}
	g.writeIndent()
	g.buf.WriteString("}")
}

// writeSelectStmt 生成通道选择语句（select { case ... : ... default: ... }）
func (g *codeGen) writeSelectStmt(s *SelectStmt) {
	g.buf.WriteString("select {\n")
	for _, c := range s.Cases {
		g.writeIndent()
		if c.Comm == nil {
			g.buf.WriteString("default:\n")
		} else {
			g.buf.WriteString("case ")
			g.writeStmt(c.Comm)
			g.buf.WriteString(":\n")
		}
		g.indent++
		if c.Body != nil {
			g.writeBlockStmt(c.Body)
		}
		g.indent--
	}
	g.writeIndent()
	g.buf.WriteString("}")
}

// writeReturnStmt 生成返回语句（支持多返回值：return a, b, c）
func (g *codeGen) writeReturnStmt(s *ReturnStmt) {
	g.buf.WriteString("return")
	if len(s.Values) > 0 {
		g.buf.WriteString(" ")
		for i, e := range s.Values {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			g.writeExpr(e)
		}
	}
}

// writeExpr 分发表达式
func (g *codeGen) writeExpr(e Expr) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *Ident:
		g.buf.WriteString(n.Name)
	case *Literal:
		g.writeLiteral(n)
	case *BinaryExpr:
		g.writeExpr(n.Lhs)
		fmt.Fprintf(&g.buf, " %s ", translateOp(n.Op))
		g.writeExpr(n.Rhs)
	case *UnaryExpr:
		g.buf.WriteString(translateOp(n.Op))
		g.writeExpr(n.X)
	case *ChanExpr:
		if n.Value == nil {
			// 接收：<-ch
			g.buf.WriteString("<-")
			g.writeExpr(n.Chan)
		} else {
			// 发送：ch <- value
			g.writeExpr(n.Chan)
			g.buf.WriteString(" <- ")
			g.writeExpr(n.Value)
		}
	case *TypeConvertExpr:
		// 类型转换：整数型(x) → int(x)
		g.buf.WriteString(mapType(n.Type))
		g.buf.WriteString("(")
		g.writeExpr(n.Arg)
		g.buf.WriteString(")")
	case *NewExpr:
		// 新建(T) → new(T)，类型通过 mapType 转换
		g.buf.WriteString("new(")
		g.buf.WriteString(mapType(n.Type))
		g.buf.WriteString(")")
	case *ChanMakeExpr:
		// 新建 通道 T → make(chan T)，元素类型通过 mapType 转换
		g.buf.WriteString("make(chan ")
		g.buf.WriteString(mapType(n.ElemType))
		g.buf.WriteString(")")
	case *FuncLit:
		// 匿名函数字面量：func(params) returnTypes { body }
		g.buf.WriteString("func(")
		g.writeParams(n.Params)
		g.buf.WriteString(")")
		g.writeReturnTypes(n.ReturnTypes)
		g.buf.WriteString(" {\n")
		if n.Body != nil {
			g.indent++
			g.writeBlockStmt(n.Body)
			g.indent--
		}
		g.writeIndent()
		g.buf.WriteString("}")
	case *RecoverExpr:
		// 恢复() → recover()
		g.buf.WriteString("recover()")
	case *IotaExpr:
		// 序数 → iota
		g.buf.WriteString("iota")
	case *CallExpr:
		g.writeExpr(n.Func)
		g.buf.WriteString("(")
		for i, a := range n.Args {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			g.writeExpr(a)
		}
		if n.Ellipsis {
			g.buf.WriteString("...")
		}
		g.buf.WriteString(")")
	case *MemberExpr:
		g.writeExpr(n.X)
		g.buf.WriteString(".")
		g.buf.WriteString(n.Sel)
	case *IndexExpr:
		g.writeExpr(n.X)
		g.buf.WriteString("[")
		g.writeExpr(n.Index)
		g.buf.WriteString("]")
	case *SliceExpr:
		g.writeExpr(n.X)
		g.buf.WriteString("[")
		if n.Low != nil {
			g.writeExpr(n.Low)
		}
		g.buf.WriteString(":")
		if n.High != nil {
			g.writeExpr(n.High)
		}
		// 三元切片 x[low:high:max]
		if n.Max != nil {
			g.buf.WriteString(":")
			g.writeExpr(n.Max)
		}
		g.buf.WriteString("]")
	case *ArrayLiteral:
		fmt.Fprintf(&g.buf, "[]%s{", mapType(n.ElemType))
		for i, e := range n.Elements {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			g.writeExpr(e)
		}
		g.buf.WriteString("}")
	case *MapLiteral:
		fmt.Fprintf(&g.buf, "map[%s]%s{", mapType(n.KeyType), mapType(n.ValueType))
		for i, p := range n.Pairs {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			g.writeExpr(p.Key)
			g.buf.WriteString(": ")
			g.writeExpr(p.Value)
		}
		g.buf.WriteString("}")
	case *StructLiteral:
		// TypeName{...} 或 TypeName{field: value, ...}
		fmt.Fprintf(&g.buf, "%s{", n.TypeName)
		for i, p := range n.Pairs {
			if i > 0 {
				g.buf.WriteString(", ")
			}
			if p.Key != nil {
				// 字段名:值 形式
				g.writeExpr(p.Key)
				g.buf.WriteString(": ")
			}
			g.writeExpr(p.Value)
		}
		g.buf.WriteString("}")
	case *KeyValueExpr:
		g.writeExpr(n.Key)
		g.buf.WriteString(": ")
		g.writeExpr(n.Value)
	case *ParenExpr:
		g.buf.WriteString("(")
		g.writeExpr(n.X)
		g.buf.WriteString(")")
	case *TypeAssertExpr:
		g.writeExpr(n.X)
		g.buf.WriteString(".(")
		if n.Type == "" {
			// x.(type) 形式（仅用于 type switch 头部）
			g.buf.WriteString("type")
		} else {
			g.buf.WriteString(mapType(n.Type))
		}
		g.buf.WriteString(")")
	}
}

// writeLiteral 生成字面量
func (g *codeGen) writeLiteral(l *Literal) {
	switch l.Kind {
	case "bool":
		// 真假已转为 true/false
		g.buf.WriteString(l.Value)
	case "nil":
		g.buf.WriteString("nil")
	case "number":
		g.buf.WriteString(l.Value)
	case "string":
		g.buf.WriteString(l.Value) // 保留原始 "..."
	case "char":
		g.buf.WriteString(l.Value) // 保留原始 '...'
	default:
		g.buf.WriteString(l.Value)
	}
}

// translateOp 把中文操作符转为 Go 操作符
// 复用 transpiler.go 的逻辑，但简化为纯映射
func translateOp(op string) string {
	switch op {
	case "＝":
		return "=="
	case "≠":
		return "!="
	case "＞":
		return ">"
	case "＜":
		return "<"
	case "≥":
		return ">="
	case "≤":
		return "<="
	case "＜＝":
		return "<="
	case "＞＝":
		return ">="
	case "＝＝":
		return "=="
	case "＜＜":
		return "<<"
	case "＞＞":
		return ">>"
	case "且":
		return "&&"
	case "或":
		return "||"
	case "非":
		return "!"
	case "＋":
		return "+"
	case "－":
		return "-"
	case "×":
		return "*"
	case "÷":
		return "/"
	}
	return op
}
