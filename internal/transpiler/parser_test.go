// Package transpiler — AST 解析器测试
//
// 验证 延迟/协程 语句能被 parser.go 正确解析为 DeferStmt/GoStmt
package transpiler

import (
	"testing"
)

// TestParseDeferGo 验证延迟/协程语句的 AST 解析
func TestParseDeferGo(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	延迟 cleanup()
	协程 worker()
结束函数
`

	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}
	if len(file.Decls) != 1 {
		t.Fatalf("期望 1 个 Decl, 实际 %d", len(file.Decls))
	}

	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("期望 *FuncDecl, 实际 %T", file.Decls[0])
	}
	if fn.Name != "主函数" {
		t.Errorf("期望函数名 主函数, 实际 %q", fn.Name)
	}
	if fn.Body == nil || len(fn.Body.Stmts) != 2 {
		t.Fatalf("期望 2 条语句, 实际 %d", len(fn.Body.Stmts))
	}

	// 第1条：DeferStmt
	def, ok := fn.Body.Stmts[0].(*DeferStmt)
	if !ok {
		t.Fatalf("期望 *DeferStmt, 实际 %T", fn.Body.Stmts[0])
	}
	call, ok := def.Call.(*CallExpr)
	if !ok {
		t.Errorf("期望 DeferStmt.Call 是 *CallExpr, 实际 %T", def.Call)
	}
	if id, ok := call.Func.(*Ident); !ok || id.Name != "cleanup" {
		t.Errorf("期望调用 cleanup(), 实际 %+v", call.Func)
	}

	// 第2条：GoStmt
	goStmt, ok := fn.Body.Stmts[1].(*GoStmt)
	if !ok {
		t.Fatalf("期望 *GoStmt, 实际 %T", fn.Body.Stmts[1])
	}
	call2, ok := goStmt.Call.(*CallExpr)
	if !ok {
		t.Errorf("期望 GoStmt.Call 是 *CallExpr, 实际 %T", goStmt.Call)
	}
	if id, ok := call2.Func.(*Ident); !ok || id.Name != "worker" {
		t.Errorf("期望调用 worker(), 实际 %+v", call2.Func)
	}
}

// TestParseLogicalOps 验证 且/或/非 在表达式中的解析
func TestParseLogicalOps(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	如果 a > 0 且 b > 0
		打印("both")
	结束如果
	如果 a < 0 或 b < 0
		打印("neg")
	结束如果
结束函数
`

	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil || len(file.Decls) != 1 {
		t.Fatal("期望 1 个 Decl")
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatal("期望 *FuncDecl")
	}
	if len(fn.Body.Stmts) != 2 {
		t.Fatalf("期望 2 条语句, 实际 %d", len(fn.Body.Stmts))
	}

	// 第1条 if 的 cond 应该是 BinaryExpr 且 Op="且"
	ifStmt, ok := fn.Body.Stmts[0].(*IfStmt)
	if !ok {
		t.Fatalf("期望 *IfStmt, 实际 %T", fn.Body.Stmts[0])
	}
	bin, ok := ifStmt.Cond.(*BinaryExpr)
	if !ok {
		t.Fatalf("期望 *BinaryExpr, 实际 %T", ifStmt.Cond)
	}
	if bin.Op != "且" {
		t.Errorf("期望 Op=且, 实际 %q", bin.Op)
	}

	// 第2条 if 的 cond 应该是 BinaryExpr 且 Op="或"
	ifStmt2, ok := fn.Body.Stmts[1].(*IfStmt)
	if !ok {
		t.Fatalf("期望 *IfStmt, 实际 %T", fn.Body.Stmts[1])
	}
	bin2, ok := ifStmt2.Cond.(*BinaryExpr)
	if !ok {
		t.Fatalf("期望 *BinaryExpr, 实际 %T", ifStmt2.Cond)
	}
	if bin2.Op != "或" {
		t.Errorf("期望 Op=或, 实际 %q", bin2.Op)
	}
}

// TestParseElseIf 验证"否则如果"简写解析为嵌套 IfStmt
// 嵌套结构：IfStmt.Else 只含一个 IfStmt
func TestParseElseIf(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	如果 a > 0
		打印("正")
	否则如果 a < 0
		打印("负")
	否则
		打印("零")
	结束如果
结束函数
`

	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil || len(file.Decls) != 1 {
		t.Fatal("期望 1 个 Decl")
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatal("期望 *FuncDecl")
	}
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("期望 1 条语句, 实际 %d", len(fn.Body.Stmts))
	}

	ifStmt, ok := fn.Body.Stmts[0].(*IfStmt)
	if !ok {
		t.Fatalf("期望 *IfStmt, 实际 %T", fn.Body.Stmts[0])
	}
	if ifStmt.Else == nil {
		t.Fatal("期望外层 IfStmt.Else 非 nil")
	}
	if len(ifStmt.Else.Stmts) != 1 {
		t.Fatalf("期望 Else 块只含 1 条语句, 实际 %d", len(ifStmt.Else.Stmts))
	}
	// Else 块的唯一语句应是嵌套 IfStmt（即"否则如果"）
	nestedIf, ok := ifStmt.Else.Stmts[0].(*IfStmt)
	if !ok {
		t.Fatalf("期望 Else 块内是 *IfStmt, 实际 %T", ifStmt.Else.Stmts[0])
	}
	// 嵌套 IfStmt 应有自己的 Else 块（"否则"分支）
	if nestedIf.Else == nil {
		t.Fatal("期望嵌套 IfStmt.Else 非 nil")
	}
	if len(nestedIf.Else.Stmts) != 1 {
		t.Logf("嵌套 Else 块语句数: %d", len(nestedIf.Else.Stmts))
	}
}

// TestParseRangeStmt 验证 range 循环解析（循环 k, v ＝ 范围 x）
func TestParseRangeStmt(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 k, v ＝ 范围 list
		打印(k, v)
	结束循环
结束函数
`

	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil || len(file.Decls) != 1 {
		t.Fatal("期望 1 个 Decl")
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatal("期望 *FuncDecl")
	}
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("期望 1 条语句, 实际 %d", len(fn.Body.Stmts))
	}

	// 应该是 *RangeStmt 而非 *ForStmt
	rangeStmt, ok := fn.Body.Stmts[0].(*RangeStmt)
	if !ok {
		t.Fatalf("期望 *RangeStmt, 实际 %T", fn.Body.Stmts[0])
	}
	if rangeStmt.Key != "k" {
		t.Errorf("期望 Key=k, 实际 %q", rangeStmt.Key)
	}
	if rangeStmt.Value != "v" {
		t.Errorf("期望 Value=v, 实际 %q", rangeStmt.Value)
	}
	if rangeStmt.X == nil {
		t.Fatal("期望 X 非 nil")
	}
	if id, ok := rangeStmt.X.(*Ident); !ok || id.Name != "list" {
		t.Errorf("期望 X 是 ident 'list', 实际 %+v", rangeStmt.X)
	}
}

// TestParseRangeStmtSingleVar 验证单变量 range（循环 v ＝ 范围 x）
func TestParseRangeStmtSingleVar(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 v ＝ 范围 items
		打印(v)
	结束循环
结束函数
`

	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil || len(file.Decls) != 1 {
		t.Fatal("期望 1 个 Decl")
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatal("期望 *FuncDecl")
	}
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("期望 1 条语句, 实际 %d", len(fn.Body.Stmts))
	}

	rangeStmt, ok := fn.Body.Stmts[0].(*RangeStmt)
	if !ok {
		t.Fatalf("期望 *RangeStmt, 实际 %T", fn.Body.Stmts[0])
	}
	// 单变量形式：Key 为空，Value 是变量名
	if rangeStmt.Key != "" {
		t.Errorf("期望 Key 为空, 实际 %q", rangeStmt.Key)
	}
	if rangeStmt.Value != "v" {
		t.Errorf("期望 Value=v, 实际 %q", rangeStmt.Value)
	}
}

// TestParseForStmtCond 验证普通 for 循环（循环 cond）不会被误识别为 range
func TestParseForStmtCond(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 i < 10
		i = i + 1
	结束循环
结束函数
`

	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil || len(file.Decls) != 1 {
		t.Fatal("期望 1 个 Decl")
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatal("期望 *FuncDecl")
	}
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("期望 1 条语句, 实际 %d", len(fn.Body.Stmts))
	}

	// 普通条件循环应该是 *ForStmt 而非 *RangeStmt
	forStmt, ok := fn.Body.Stmts[0].(*ForStmt)
	if !ok {
		t.Fatalf("期望 *ForStmt, 实际 %T", fn.Body.Stmts[0])
	}
	if forStmt.Cond == nil {
		t.Fatal("期望 Cond 非 nil")
	}
}
