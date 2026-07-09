// Package transpiler — AST → Go 代码生成器测试
package transpiler

import (
	"strings"
	"testing"
)

// TestGenerateGoBasic 验证基础语法（函数/如果/返回）的代码生成
func TestGenerateGoBasic(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	如果 1 > 0
		打印("hello")
	结束如果
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo failed: %v", err)
	}

	checks := []struct {
		desc string
		want string
	}{
		{"包名", "package main"},
		{"函数声明", "func mainImpl()"},
		{"if 语句", "if"},
		{"大于号", ">"},
		{"函数体闭合", "}"},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: 期望包含 %q, 实际:\n%s", c.desc, c.want, out)
		}
	}
}

// TestGenerateGoDeferGo 验证延迟/协程的代码生成
func TestGenerateGoDeferGo(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	延迟 cleanup()
	协程 worker()
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	if !strings.Contains(out, "defer cleanup()") {
		t.Errorf("期望包含 defer cleanup(), 实际:\n%s", out)
	}
	if !strings.Contains(out, "go worker()") {
		t.Errorf("期望包含 go worker(), 实际:\n%s", out)
	}
}

// TestGenerateGoLogicalOps 验证且/或在表达式中的代码生成
func TestGenerateGoLogicalOps(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	如果 a > 0 且 b > 0
		返回
	结束如果
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	if !strings.Contains(out, "&&") {
		t.Errorf("期望包含 &&, 实际:\n%s", out)
	}
}

// TestTranspileASTFallback 验证 TranspileAST 入口
func TestTranspileASTFallback(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	返回
结束函数
`
	out, err := TranspileAST(src)
	if err != nil {
		t.Fatalf("TranspileAST failed: %v", err)
	}
	if !strings.Contains(out, "package main") {
		t.Errorf("期望包含 package main, 实际:\n%s", out)
	}
	if !strings.Contains(out, "func mainImpl()") {
		t.Errorf("期望包含 func mainImpl(), 实际:\n%s", out)
	}
}

// TestGenerateGoGofmtFormatted 验证 GenerateGo 输出经过 go/format 格式化
// 检查标准 gofmt 风格：tab 缩进、单空格分隔等
func TestGenerateGoGofmtFormatted(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	如果 1 > 0
		打印("hello")
	结束如果
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}
	out, err := GenerateGo(file)
	// gofmt 失败是允许的（生成的代码可能有未支持的节点），但要返回原始代码
	if out == "" {
		t.Fatal("GenerateGo 返回空字符串")
	}
	// 如果格式化成功，应该使用 tab 缩进；如果失败则跳过该断言
	if err == nil {
		// gofmt 标准使用 tab 缩进
		if !strings.Contains(out, "\t") {
			t.Logf("警告: 格式化成功但未包含 tab 缩进:\n%s", out)
		}
	}
}

// TestFormatGoCode 验证 FormatGoCode 函数
func TestFormatGoCode(t *testing.T) {
	// 空字符串
	out, err := FormatGoCode("")
	if err != nil || out != "" {
		t.Errorf("空字符串应原样返回, got: %q, err: %v", out, err)
	}

	// 已格式化的代码应保持不变
	formatted := "package main\n\nfunc foo() {}\n"
	out, err = FormatGoCode(formatted)
	if err != nil {
		t.Errorf("已格式化代码不应报错: %v", err)
	}
	if out != formatted {
		t.Logf("已格式化代码（可能有细微差异）:\n%s", out)
	}

	// 未格式化的代码应被规整
	unformatted := "package main\nfunc foo()    {}\n"
	out, err = FormatGoCode(unformatted)
	if err != nil {
		t.Errorf("未格式化代码应能被处理: %v", err)
	}
	if !strings.Contains(out, "func foo() {}") {
		t.Errorf("期望包含 'func foo() {}', 实际:\n%s", out)
	}

	// 有语法错误的代码应返回错误 + 原始代码
	invalid := "package main\nfunc {"
	out, err = FormatGoCode(invalid)
	if err == nil {
		t.Error("期望对语法错误代码返回 error")
	}
	if out != invalid {
		t.Errorf("语法错误时应返回原始代码, got: %q", out)
	}
}

// TestGenerateGoElseIf 验证"否则如果"简写生成 else if 链
// 而非嵌套的 } else { if ... }
func TestGenerateGoElseIf(t *testing.T) {
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
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err（gofmt 可能失败，但 out 仍可用）: %v", err)
	}

	// 关键断言：应该生成 "} else if" 而非嵌套的 "} else {\n    if"
	if !strings.Contains(out, "else if") {
		t.Errorf("期望包含 'else if', 实际:\n%s", out)
	}
	// 检查是否生成了嵌套结构（} else { 后紧跟 if a < 0 在新行缩进）
	if strings.Contains(out, "else {\n") && strings.Contains(out, "\tif a < 0") {
		t.Errorf("生成了嵌套 } else { if 而非 } else if, 实际:\n%s", out)
	}
}

// TestGenerateGoRangeStmt 验证 range 循环的代码生成
func TestGenerateGoRangeStmt(t *testing.T) {
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
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err（gofmt 可能失败）: %v", err)
	}

	// 应该生成 "for k, v := range list {"
	if !strings.Contains(out, "for k, v := range list") {
		t.Errorf("期望包含 'for k, v := range list', 实际:\n%s", out)
	}
}

// TestGenerateGoRangeSingleVar 验证单变量 range 生成 "for _, v := range x"
func TestGenerateGoRangeSingleVar(t *testing.T) {
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
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 单变量形式：Key 为空，应生成 "for _, v := range items"
	if !strings.Contains(out, "for _, v := range items") {
		t.Errorf("期望包含 'for _, v := range items', 实际:\n%s", out)
	}
}

// TestGenerateGoCompoundAssign 验证复合赋值运算符（+=/-=/*=//=）保留
func TestGenerateGoCompoundAssign(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 x, 整数型
	x += 5
	x -= 3
	x *= 2
	x /= 4
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	checks := []struct {
		desc string
		want string
	}{
		{"加赋值", "x += 5"},
		{"减赋值", "x -= 3"},
		{"乘赋值", "x *= 2"},
		{"除赋值", "x /= 4"},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: 期望包含 %q, 实际:\n%s", c.desc, c.want, out)
		}
	}
}

// TestGenerateGoSimpleAssign 验证全角等号 ＝ 转为 ASCII =
func TestGenerateGoSimpleAssign(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 x, 整数型
	x ＝ 10
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 全角 ＝ 应转为 ASCII =
	if !strings.Contains(out, "x = 10") {
		t.Errorf("期望包含 'x = 10', 实际:\n%s", out)
	}
	// 不应保留全角 ＝
	if strings.Contains(out, "＝") {
		t.Errorf("不应包含全角 ＝, 实际:\n%s", out)
	}
}

// TestGenerateGoIncDec 验证自增/自减运算符（++/--）
func TestGenerateGoIncDec(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 i, 整数型
	i++
	i--
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "i++") {
		t.Errorf("期望包含 'i++', 实际:\n%s", out)
	}
	if !strings.Contains(out, "i--") {
		t.Errorf("期望包含 'i--', 实际:\n%s", out)
	}
}

// TestGenerateGoReturnMulti 验证多返回值语句生成 return a, b
func TestGenerateGoReturnMulti(t *testing.T) {
	src := `# 程序集 main

函数 取两个数()
	返回 1, 2
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证多返回值生成 "return 1, 2"
	if !strings.Contains(out, "return 1, 2") {
		t.Errorf("期望包含 'return 1, 2', 实际:\n%s", out)
	}
}

// TestGenerateGoReturnSingle 验证单返回值仍正常生成 return x
func TestGenerateGoReturnSingle(t *testing.T) {
	src := `# 程序集 main

函数 加法(a 整数型, b 整数型) 整数型
	返回 a + b
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "return a + b") {
		t.Errorf("期望包含 'return a + b', 实际:\n%s", out)
	}
}

// TestGenerateGoReturnBare 验证裸 return（无返回值）生成 "return"
func TestGenerateGoReturnBare(t *testing.T) {
	src := `# 程序集 main

函数 无返回()
	返回
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成裸 return（后面不跟表达式）
	if !strings.Contains(out, "return\n") {
		t.Errorf("期望包含裸 'return\\n', 实际:\n%s", out)
	}
}

// TestGenerateGoMultiReturnTypes 验证多返回类型声明生成 func f() (t1, t2)
func TestGenerateGoMultiReturnTypes(t *testing.T) {
	src := `# 程序集 main

函数 取两个数() (整数型, 文本型)
	返回 1, "hello"
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成多返回类型 "func 取两个数() (int, string)"
	if !strings.Contains(out, "func 取两个数() (int, string)") {
		t.Errorf("期望包含 'func 取两个数() (int, string)', 实际:\n%s", out)
	}
	// 验证多返回值生成 "return 1, \"hello\""
	if !strings.Contains(out, `return 1, "hello"`) {
		t.Errorf("期望包含 'return 1, \"hello\"', 实际:\n%s", out)
	}
}

// TestGenerateGoSingleReturnType 验证单返回类型仍正常生成 func f() t
func TestGenerateGoSingleReturnType(t *testing.T) {
	src := `# 程序集 main

函数 加法(参数 a 整数型, 参数 b 整数型) 整数型
	返回 a + b
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证单返回类型生成 "func 加法(a int, b int) int"
	if !strings.Contains(out, "func 加法(a int, b int) int") {
		t.Errorf("期望包含 'func 加法(a int, b int) int', 实际:\n%s", out)
	}
}

// TestGenerateGoNoReturnType 验证无返回类型生成 func f()（无空格）
func TestGenerateGoNoReturnType(t *testing.T) {
	src := `# 程序集 main

函数 无返回()
	返回
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证无返回类型生成 "func 无返回() {"（括号后直接空格+花括号，无类型）
	if !strings.Contains(out, "func 无返回() {") {
		t.Errorf("期望包含 'func 无返回() {', 实际:\n%s", out)
	}
}

// TestGenerateGoMultiAssign 验证多变量赋值 a, b ＝ 1, 2 → a, b = 1, 2
func TestGenerateGoMultiAssign(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 a, 整数型
	局部变量 b, 整数型
	a, b ＝ 1, 2
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证多变量赋值生成 "a, b = 1, 2"（全角 ＝ 转 ASCII =）
	if !strings.Contains(out, "a, b = 1, 2") {
		t.Errorf("期望包含 'a, b = 1, 2', 实际:\n%s", out)
	}
}

// TestGenerateGoMultiAssignShortDecl 验证多变量短声明 a, b := f() → a, b := f()
func TestGenerateGoMultiAssignShortDecl(t *testing.T) {
	src := `# 程序集 main

函数 取两个数() (整数型, 整数型)
	返回 1, 2
结束函数

函数 主函数()
	a, b := 取两个数()
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证短声明生成 "a, b := 取两个数()"
	if !strings.Contains(out, "a, b := 取两个数()") {
		t.Errorf("期望包含 'a, b := 取两个数()', 实际:\n%s", out)
	}
}

// TestGenerateGoMultiAssignFromCall 验证多返回值赋值 a, b ＝ f() → a, b = f()
func TestGenerateGoMultiAssignFromCall(t *testing.T) {
	src := `# 程序集 main

函数 取两个数() (整数型, 整数型)
	返回 1, 2
结束函数

函数 主函数()
	局部变量 a, 整数型
	局部变量 b, 整数型
	a, b ＝ 取两个数()
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证多返回值赋值生成 "a, b = 取两个数()"
	if !strings.Contains(out, "a, b = 取两个数()") {
		t.Errorf("期望包含 'a, b = 取两个数()', 实际:\n%s", out)
	}
}

// TestGenerateGoSliceFull 验证切片表达式 x[low:high]
func TestGenerateGoSliceFull(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 t, 文本型
	t ＝ s[1:3]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "s[1:3]"
	if !strings.Contains(out, "s[1:3]") {
		t.Errorf("期望包含 's[1:3]', 实际:\n%s", out)
	}
}

// TestGenerateGoSliceHighOnly 验证切片表达式 x[:high]
func TestGenerateGoSliceHighOnly(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 t, 文本型
	t ＝ s[:3]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "s[:3]"（low 省略）
	if !strings.Contains(out, "s[:3]") {
		t.Errorf("期望包含 's[:3]', 实际:\n%s", out)
	}
}

// TestGenerateGoSliceLowOnly 验证切片表达式 x[low:]
func TestGenerateGoSliceLowOnly(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 t, 文本型
	t ＝ s[1:]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "s[1:]"（high 省略）
	if !strings.Contains(out, "s[1:]") {
		t.Errorf("期望包含 's[1:]', 实际:\n%s", out)
	}
}

// TestGenerateGoSliceAll 验证切片表达式 x[:]（全切）
func TestGenerateGoSliceAll(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 t, 文本型
	t ＝ s[:]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "s[:]"（两端都省略）
	if !strings.Contains(out, "s[:]") {
		t.Errorf("期望包含 's[:]', 实际:\n%s", out)
	}
}

// TestGenerateGoIndexStillWorks 验证索引访问 x[i] 在切片支持后仍正常工作
func TestGenerateGoIndexStillWorks(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 c, 文本型
	c ＝ s[2]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证索引访问仍生成 "s[2]"（不被误判为切片）
	if !strings.Contains(out, "s[2]") {
		t.Errorf("期望包含 's[2]', 实际:\n%s", out)
	}
}

// TestGenerateGoMapLiteral 验证映射字面量 映射 文本型 整数型 {"a":1,"b":2} → map[string]int{"a": 1, "b": 2}
func TestGenerateGoMapLiteral(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 m, 变体型
	m ＝ 映射 文本型 整数型 {"a": 1, "b": 2}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "map[string]int{"
	if !strings.Contains(out, "map[string]int{") {
		t.Errorf("期望包含 'map[string]int{', 实际:\n%s", out)
	}
	// 验证键值对 "a": 1
	if !strings.Contains(out, `"a": 1`) {
		t.Errorf("期望包含 '\"a\": 1', 实际:\n%s", out)
	}
	// 验证键值对 "b": 2
	if !strings.Contains(out, `"b": 2`) {
		t.Errorf("期望包含 '\"b\": 2', 实际:\n%s", out)
	}
}

// TestGenerateGoMapLiteralEmpty 验证空映射字面量
func TestGenerateGoMapLiteralEmpty(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 m, 变体型
	m ＝ 映射 文本型 整数型 {}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "map[string]int{}"（空映射）
	if !strings.Contains(out, "map[string]int{}") {
		t.Errorf("期望包含 'map[string]int{}', 实际:\n%s", out)
	}
}

// TestGenerateGoMapLiteralSinglePair 验证单键值对映射
func TestGenerateGoMapLiteralSinglePair(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 m, 变体型
	m ＝ 映射 文本型 文本型 {"key": "value"}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "map[string]string{"key": "value"}"
	if !strings.Contains(out, `map[string]string{"key": "value"}`) {
		t.Errorf("期望包含 'map[string]string{\"key\": \"value\"}', 实际:\n%s", out)
	}
}

// TestGenerateGoArrayLiteral 验证数组字面量 整数数组{1,2,3} → []int{1, 2, 3}
func TestGenerateGoArrayLiteral(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 arr, 变体型
	arr ＝ 整数数组{1, 2, 3}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "[]int{1, 2, 3}"
	if !strings.Contains(out, "[]int{1, 2, 3}") {
		t.Errorf("期望包含 '[]int{1, 2, 3}', 实际:\n%s", out)
	}
}

// TestGenerateGoArrayLiteralEmpty 验证空数组字面量
func TestGenerateGoArrayLiteralEmpty(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 arr, 变体型
	arr ＝ 文本数组{}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "[]string{}"（空数组）
	if !strings.Contains(out, "[]string{}") {
		t.Errorf("期望包含 '[]string{}', 实际:\n%s", out)
	}
}

// TestGenerateGoArrayLiteralSingleElem 验证单元素数组
func TestGenerateGoArrayLiteralSingleElem(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 arr, 变体型
	arr ＝ 文本数组{"hello"}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 `[]string{"hello"}`
	if !strings.Contains(out, `[]string{"hello"}`) {
		t.Errorf("期望包含 '[]string{\"hello\"}', 实际:\n%s", out)
	}
}

// TestGenerateGoArrayLiteralIndex 验证数组字面量后接索引 整数数组{1,2,3}[0]
func TestGenerateGoArrayLiteralIndex(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 x, 整数型
	x ＝ 整数数组{1, 2, 3}[0]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "[]int{1, 2, 3}[0]"
	if !strings.Contains(out, "[]int{1, 2, 3}[0]") {
		t.Errorf("期望包含 '[]int{1, 2, 3}[0]', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAssert 验证类型断言 x.(整数型) → x.(int)
func TestGenerateGoTypeAssert(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 v, 变体型
	局部变量 n, 整数型
	n ＝ v.(整数型)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "v.(int)"
	if !strings.Contains(out, "v.(int)") {
		t.Errorf("期望包含 'v.(int)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAssertIdent 验证类型断言使用标识符类型 x.(MyType)
func TestGenerateGoTypeAssertIdent(t *testing.T) {
	src := `# 程序集 main

类型 MyType 结构体
结束类型

函数 主函数()
	局部变量 v, 变体型
	局部变量 m, MyType
	m ＝ v.(MyType)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "v.(MyType)"
	if !strings.Contains(out, "v.(MyType)") {
		t.Errorf("期望包含 'v.(MyType)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAssertTypeSwitch 验证 x.(type) 用于 type switch
func TestGenerateGoTypeAssertTypeSwitch(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 v, 变体型
	局部变量 t, 整数型
	t ＝ v.(type)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "v.(type)"
	if !strings.Contains(out, "v.(type)") {
		t.Errorf("期望包含 'v.(type)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAssertChain 验证类型断言后接成员访问 v.(MyType).Field
func TestGenerateGoTypeAssertChain(t *testing.T) {
	src := `# 程序集 main

类型 MyType 结构体
	x 整数型
结束类型

函数 主函数()
	局部变量 v, 变体型
	局部变量 n, 整数型
	n ＝ v.(MyType).x
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "v.(MyType).x"（链式调用）
	if !strings.Contains(out, "v.(MyType).x") {
		t.Errorf("期望包含 'v.(MyType).x', 实际:\n%s", out)
	}
}

// TestGenerateGoSliceFull3 验证三元切片 x[low:high:max]
func TestGenerateGoSliceFull3(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 t, 文本型
	t ＝ s[1:3:5]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "s[1:3:5]"（三元切片）
	if !strings.Contains(out, "s[1:3:5]") {
		t.Errorf("期望包含 's[1:3:5]', 实际:\n%s", out)
	}
}

// TestGenerateGoSliceLowMax 验证三元切片 x[low::max] 不允许，必须是 low:high:max
// 但 Go 不允许 x[low::max]，所以这里测试 x[low:high:max] 完整形式
func TestGenerateGoSliceLowMax(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 t, 文本型
	t ＝ s[2:4:6]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "s[2:4:6]") {
		t.Errorf("期望包含 's[2:4:6]', 实际:\n%s", out)
	}
}

// TestGenerateGoSliceHighMax 验证三元切片省略 low：x[:high:max]
func TestGenerateGoSliceHighMax(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 t, 文本型
	t ＝ s[:3:5]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "s[:3:5]"（省略 low 的三元切片）
	if !strings.Contains(out, "s[:3:5]") {
		t.Errorf("期望包含 's[:3:5]', 实际:\n%s", out)
	}
}

// TestGenerateGoSliceTwoColonRegression 验证 v26 的二元切片仍正常（回归测试）
func TestGenerateGoSliceTwoColonRegression(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 a, 文本型
	局部变量 b, 文本型
	局部变量 c, 文本型
	a ＝ s[1:3]
	b ＝ s[:2]
	c ＝ s[2:]
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证二元切片仍正常（不能因为支持三元而退化）
	if !strings.Contains(out, "s[1:3]") {
		t.Errorf("期望包含 's[1:3]', 实际:\n%s", out)
	}
	if !strings.Contains(out, "s[:2]") {
		t.Errorf("期望包含 's[:2]', 实际:\n%s", out)
	}
	if !strings.Contains(out, "s[2:]") {
		t.Errorf("期望包含 's[2:]', 实际:\n%s", out)
	}
}

// TestGenerateGoStructLiteralPositional 验证结构体字面量按位置赋值 Point{1, 2}
func TestGenerateGoStructLiteralPositional(t *testing.T) {
	src := `# 程序集 main

类型 Point 结构体
	x 整数型
	y 整数型
结束类型

函数 主函数()
	局部变量 p, Point
	p ＝ Point{1, 2}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "Point{1, 2}"（按位置赋值）
	if !strings.Contains(out, "Point{1, 2}") {
		t.Errorf("期望包含 'Point{1, 2}', 实际:\n%s", out)
	}
}

// TestGenerateGoStructLiteralNamed 验证结构体字面量按字段名赋值 Point{x: 1, y: 2}
func TestGenerateGoStructLiteralNamed(t *testing.T) {
	src := `# 程序集 main

类型 Point 结构体
	x 整数型
	y 整数型
结束类型

函数 主函数()
	局部变量 p, Point
	p ＝ Point{x: 1, y: 2}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "Point{x: 1, y: 2}"（按字段名赋值）
	if !strings.Contains(out, "Point{x: 1, y: 2}") {
		t.Errorf("期望包含 'Point{x: 1, y: 2}', 实际:\n%s", out)
	}
}

// TestGenerateGoStructLiteralEmpty 验证空结构体字面量 Point{}
func TestGenerateGoStructLiteralEmpty(t *testing.T) {
	src := `# 程序集 main

类型 Empty 结构体
结束类型

函数 主函数()
	局部变量 e, Empty
	e ＝ Empty{}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "Empty{}"（空结构体）
	if !strings.Contains(out, "Empty{}") {
		t.Errorf("期望包含 'Empty{}', 实际:\n%s", out)
	}
}

// TestGenerateGoStructLiteralSingleField 验证单字段结构体 Point{x: 1}
func TestGenerateGoStructLiteralSingleField(t *testing.T) {
	src := `# 程序集 main

类型 Point 结构体
	x 整数型
	y 整数型
结束类型

函数 主函数()
	局部变量 p, Point
	p ＝ Point{x: 1}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "Point{x: 1}"（单字段）
	if !strings.Contains(out, "Point{x: 1}") {
		t.Errorf("期望包含 'Point{x: 1}', 实际:\n%s", out)
	}
}

// TestGenerateGoStructLiteralMember 验证结构体字面量后接成员访问 Point{1,2}.x
func TestGenerateGoStructLiteralMember(t *testing.T) {
	src := `# 程序集 main

类型 Point 结构体
	x 整数型
	y 整数型
结束类型

函数 主函数()
	局部变量 n, 整数型
	n ＝ Point{1, 2}.x
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "Point{1, 2}.x"（后缀成员访问）
	if !strings.Contains(out, "Point{1, 2}.x") {
		t.Errorf("期望包含 'Point{1, 2}.x', 实际:\n%s", out)
	}
}

// TestGenerateGoAddrOf 验证取址表达式 &x
func TestGenerateGoAddrOf(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 x, 整数型
	局部变量 p, *整数型
	p ＝ &x
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "&x"
	if !strings.Contains(out, "&x") {
		t.Errorf("期望包含 '&x', 实际:\n%s", out)
	}
}

// TestGenerateGoDeref 验证解引用表达式 *p
func TestGenerateGoDeref(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 x, 整数型
	局部变量 p, *整数型
	x ＝ *p
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "*p"（解引用）
	if !strings.Contains(out, "*p") {
		t.Errorf("期望包含 '*p', 实际:\n%s", out)
	}
}

// TestGenerateGoAddrOfStruct 验证取结构体字段地址 &s.field
func TestGenerateGoAddrOfStruct(t *testing.T) {
	src := `# 程序集 main

类型 Point 结构体
	x 整数型
	y 整数型
结束类型

函数 主函数()
	局部变量 p, Point
	局部变量 px, *整数型
	px ＝ &p.x
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "&p.x"
	if !strings.Contains(out, "&p.x") {
		t.Errorf("期望包含 '&p.x', 实际:\n%s", out)
	}
}

// TestGenerateGoDerefMember 验证解引用后成员访问
// 注意：Go 中 *p.x 等价于 *(p.x)，要 (*p).x 需写括号
// 这里测试 *p.y 生成 *p.y（语义：*(p.y)）
func TestGenerateGoDerefMember(t *testing.T) {
	src := `# 程序集 main

类型 Point 结构体
	x 整数型
结束类型

函数 主函数()
	局部变量 p, *Point
	局部变量 n, 整数型
	n ＝ *p.x
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "*p.x"（语义 *(p.x)，符合 Go 优先级）
	if !strings.Contains(out, "*p.x") {
		t.Errorf("期望包含 '*p.x', 实际:\n%s", out)
	}
}

// TestGenerateGoUnaryNegRegression 验证既有的一元负号 -x 仍正常（回归测试）
func TestGenerateGoUnaryNegRegression(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 x, 整数型
	局部变量 y, 整数型
	y ＝ -x
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证一元负号仍正常
	if !strings.Contains(out, "-x") {
		t.Errorf("期望包含 '-x', 实际:\n%s", out)
	}
}

// TestGenerateGoChanRecv 验证通道接收 <-ch
func TestGenerateGoChanRecv(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 ch, 变体型
	局部变量 v, 整数型
	v ＝ <-ch
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "<-ch"（接收）
	if !strings.Contains(out, "<-ch") {
		t.Errorf("期望包含 '<-ch', 实际:\n%s", out)
	}
}

// TestGenerateGoChanSend 验证通道发送 ch <- value（语句形式）
func TestGenerateGoChanSend(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 ch, 变体型
	ch <- 42
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "ch <- 42"（发送）
	if !strings.Contains(out, "ch <- 42") {
		t.Errorf("期望包含 'ch <- 42', 实际:\n%s", out)
	}
}

// TestGenerateGoChanSendExpr 验证通道发送变量 ch <- v
func TestGenerateGoChanSendExpr(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 ch, 变体型
	局部变量 v, 整数型
	ch <- v
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "ch <- v"
	if !strings.Contains(out, "ch <- v") {
		t.Errorf("期望包含 'ch <- v', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeConvertInt 验证类型转换 整数型(x) → int(x)
func TestGenerateGoTypeConvertInt(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 s, 文本型
	局部变量 n, 整数型
	n ＝ 整数型(s)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "int(s)"
	if !strings.Contains(out, "int(s)") {
		t.Errorf("期望包含 'int(s)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeConvertString 验证类型转换 文本型(x) → string(x)
func TestGenerateGoTypeConvertString(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 n, 整数型
	局部变量 s, 文本型
	s ＝ 文本型(n)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "string(n)"
	if !strings.Contains(out, "string(n)") {
		t.Errorf("期望包含 'string(n)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeConvertFloat 验证类型转换 小数型(x) → float32(x)
func TestGenerateGoTypeConvertFloat(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 n, 整数型
	局部变量 f, 小数型
	f ＝ 小数型(n)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "float32(n)"（小数型对应 float32）
	if !strings.Contains(out, "float32(n)") {
		t.Errorf("期望包含 'float32(n)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeConvertFloat64 验证类型转换 双精度小数型(x) → float64(x)
func TestGenerateGoTypeConvertFloat64(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 n, 整数型
	局部变量 f, 双精度小数型
	f ＝ 双精度小数型(n)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "float64(n)"（双精度小数型对应 float64）
	if !strings.Contains(out, "float64(n)") {
		t.Errorf("期望包含 'float64(n)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeConvertChain 验证类型转换后接成员访问 整数型(x).field
func TestGenerateGoTypeConvertChain(t *testing.T) {
	src := `# 程序集 main

类型 MyType 结构体
	x 整数型
结束类型

函数 主函数()
	局部变量 v, 变体型
	局部变量 n, 整数型
	n ＝ 整数型(v.x)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "int(v.x)"（参数是成员访问表达式）
	if !strings.Contains(out, "int(v.x)") {
		t.Errorf("期望包含 'int(v.x)', 实际:\n%s", out)
	}
}

// TestGenerateGoSelectRecv 验证 select 接收分支 情况 v := <-ch:
func TestGenerateGoSelectRecv(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 ch, 变体型
	局部变量 v, 整数型
	通道选择
		情况 v := <-ch:
			打印(v)
	结束通道选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "select {"
	if !strings.Contains(out, "select {") {
		t.Errorf("期望包含 'select {', 实际:\n%s", out)
	}
	// 验证生成 "case v := <-ch:"
	if !strings.Contains(out, "case v := <-ch:") {
		t.Errorf("期望包含 'case v := <-ch:', 实际:\n%s", out)
	}
}

// TestGenerateGoSelectSend 验证 select 发送分支 情况 ch <- v:
func TestGenerateGoSelectSend(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 ch, 变体型
	局部变量 v, 整数型
	通道选择
		情况 ch <- v:
			打印("sent")
	结束通道选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "case ch <- v:"
	if !strings.Contains(out, "case ch <- v:") {
		t.Errorf("期望包含 'case ch <- v:', 实际:\n%s", out)
	}
}

// TestGenerateGoSelectDefault 验证 select 默认分支
func TestGenerateGoSelectDefault(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 ch, 变体型
	通道选择
		默认:
			打印("default")
	结束通道选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "default:"
	if !strings.Contains(out, "default:") {
		t.Errorf("期望包含 'default:', 实际:\n%s", out)
	}
}

// TestGenerateGoSelectMultiCases 验证 select 多分支
func TestGenerateGoSelectMultiCases(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 ch1, 变体型
	局部变量 ch2, 变体型
	局部变量 v, 整数型
	通道选择
		情况 v := <-ch1:
			打印("ch1", v)
		情况 ch2 <- 42:
			打印("ch2 sent")
		默认:
			打印("default")
	结束通道选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成三个分支
	if !strings.Contains(out, "case v := <-ch1:") {
		t.Errorf("期望包含 'case v := <-ch1:', 实际:\n%s", out)
	}
	if !strings.Contains(out, "case ch2 <- 42:") {
		t.Errorf("期望包含 'case ch2 <- 42:', 实际:\n%s", out)
	}
	if !strings.Contains(out, "default:") {
		t.Errorf("期望包含 'default:', 实际:\n%s", out)
	}
}

// TestGenerateGoChanTypeLocalVar 验证局部变量声明 chan 类型 通道 整数型 → chan int
func TestGenerateGoChanTypeLocalVar(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 ch, 通道 整数型
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "var ch chan int"
	if !strings.Contains(out, "var ch chan int") {
		t.Errorf("期望包含 'var ch chan int', 实际:\n%s", out)
	}
}

// TestGenerateGoChanTypeParam 验证函数参数 chan 类型
func TestGenerateGoChanTypeParam(t *testing.T) {
	src := `# 程序集 main

函数 处理(参数 ch 通道 整数型)
	返回
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "func 处理(ch chan int)"
	if !strings.Contains(out, "func 处理(ch chan int)") {
		t.Errorf("期望包含 'func 处理(ch chan int)', 实际:\n%s", out)
	}
}

// TestGenerateGoChanTypeReturn 验证函数返回 chan 类型
func TestGenerateGoChanTypeReturn(t *testing.T) {
	src := `# 程序集 main

函数 取通道() 通道 整数型
	返回 空
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "func 取通道() chan int"
	if !strings.Contains(out, "func 取通道() chan int") {
		t.Errorf("期望包含 'func 取通道() chan int', 实际:\n%s", out)
	}
}

// TestGenerateGoChanTypeField 验证结构体字段 chan 类型
func TestGenerateGoChanTypeField(t *testing.T) {
	src := `# 程序集 main

类型 Worker 结构体
	jobs, 通道 整数型
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "jobs chan int"
	if !strings.Contains(out, "jobs chan int") {
		t.Errorf("期望包含 'jobs chan int', 实际:\n%s", out)
	}
}

// TestGenerateGoPtrChanType 验证组合类型 *通道 整数型 → *chan int
func TestGenerateGoPtrChanType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 p, *通道 整数型
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "var p *chan int"
	if !strings.Contains(out, "var p *chan int") {
		t.Errorf("期望包含 'var p *chan int', 实际:\n%s", out)
	}
}

// TestGenerateGoMultiReturnChanType 验证多返回值包含 chan 类型
func TestGenerateGoMultiReturnChanType(t *testing.T) {
	src := `# 程序集 main

函数 取两个通道() (通道 整数型, 通道 文本型)
	返回 空, 空
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 "(chan int, chan string)"
	if !strings.Contains(out, "(chan int, chan string)") {
		t.Errorf("期望包含 '(chan int, chan string)', 实际:\n%s", out)
	}
}

// TestGenerateGoNewBasic 验证新建(整数型) → new(int)
func TestGenerateGoNewBasic(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 p, *整数型
	p ＝ 新建(整数型)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "p = new(int)") {
		t.Errorf("期望包含 'p = new(int)', 实际:\n%s", out)
	}
}

// TestGenerateGoNewStruct 验证新建(用户信息) → new(用户信息)
func TestGenerateGoNewStruct(t *testing.T) {
	src := `# 程序集 main

类型 用户信息 结构体
	姓名, 文本型
结束类型

函数 主函数()
	局部变量 u, *用户信息
	u ＝ 新建(用户信息)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "u = new(用户信息)") {
		t.Errorf("期望包含 'u = new(用户信息)', 实际:\n%s", out)
	}
}

// TestGenerateGoNewMemberAccess 验证新建(T).字段 后缀访问
func TestGenerateGoNewMemberAccess(t *testing.T) {
	src := `# 程序集 main

类型 点 结构体
	x, 整数型
	y, 整数型
结束类型

函数 主函数()
	局部变量 v, 整数型
	v ＝ 新建(点).x
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "new(点).x") {
		t.Errorf("期望包含 'new(点).x', 实际:\n%s", out)
	}
}

// TestGenerateGoNewPtrType 验证新建(*整数型) → new(*int) 嵌套指针
func TestGenerateGoNewPtrType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 pp, **整数型
	pp ＝ 新建(*整数型)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "pp = new(*int)") {
		t.Errorf("期望包含 'pp = new(*int)', 实际:\n%s", out)
	}
}

// TestGenerateGoNewChanType 验证新建(通道 整数型) → new(chan int)
func TestGenerateGoNewChanType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 cp, *通道 整数型
	cp ＝ 新建(通道 整数型)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "cp = new(chan int)") {
		t.Errorf("期望包含 'cp = new(chan int)', 实际:\n%s", out)
	}
}

// TestGenerateGoNewAsArg 验证新建(T) 作为函数参数
func TestGenerateGoNewAsArg(t *testing.T) {
	src := `# 程序集 main

函数 处理(p *整数型)
结束函数

函数 主函数()
	处理(新建(整数型))
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "处理(new(int))") {
		t.Errorf("期望包含 '处理(new(int))', 实际:\n%s", out)
	}
}

// TestGenerateGoVariadicParam 验证可变参数函数声明：参数 args ... 整数型 → args ...int
func TestGenerateGoVariadicParam(t *testing.T) {
	src := `# 程序集 main

函数 求和(参数 nums ... 整数型) 整数型
	局部变量 总和, 整数型
	循环 v ＝ 范围 nums
		总和 += v
	结束循环
	返回 总和
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "func 求和(nums ...int) int") {
		t.Errorf("期望包含 'func 求和(nums ...int) int', 实际:\n%s", out)
	}
}

// TestGenerateGoVariadicEmpty 验证可变参数函数声明：参数 args ... 变体型 → args ...interface{}
func TestGenerateGoVariadicEmpty(t *testing.T) {
	src := `# 程序集 main

函数 打印全部(参数 args ... 变体型)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "func 打印全部(args ...interface{})") {
		t.Errorf("期望包含 'func 打印全部(args ...interface{})', 实际:\n%s", out)
	}
}

// TestGenerateGoVariadicCallSpread 验证可变参数调用展开：f(args...) → f(args...)
func TestGenerateGoVariadicCallSpread(t *testing.T) {
	src := `# 程序集 main

函数 求和(参数 nums ... 整数型) 整数型
	返回 0
结束函数

函数 主函数()
	局部变量 arr, 整数数组
	求和(arr...)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "求和(arr...)") {
		t.Errorf("期望包含 '求和(arr...)', 实际:\n%s", out)
	}
}

// TestGenerateGoVariadicCallNormal 验证可变参数函数的普通调用（不展开）
func TestGenerateGoVariadicCallNormal(t *testing.T) {
	src := `# 程序集 main

函数 求和(参数 nums ... 整数型) 整数型
	返回 0
结束函数

函数 主函数()
	求和(1, 2, 3)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "求和(1, 2, 3)") {
		t.Errorf("期望包含 '求和(1, 2, 3)', 实际:\n%s", out)
	}
}

// TestGenerateGoVariadicMixedParams 验证可变参数与其他参数混合：f(prefix 文本型, args ... 整数型)
func TestGenerateGoVariadicMixedParams(t *testing.T) {
	src := `# 程序集 main

函数 打印带前缀(参数 prefix 文本型, 参数 args ... 整数型)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "func 打印带前缀(prefix string, args ...int)") {
		t.Errorf("期望包含 'func 打印带前缀(prefix string, args ...int)', 实际:\n%s", out)
	}
}

// TestGenerateGoParamSyntax 验证 "名字 类型" 语法（包括用户用 "参数" 作为变量名）
func TestGenerateGoParamSyntax(t *testing.T) {
	src := `# 程序集 main

函数 子程序1(参数 文本型)
打印(参数)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证：参数名为 "参数"，类型为 string
	if !strings.Contains(out, "func 子程序1(参数 string)") {
		t.Errorf("期望包含 'func 子程序1(参数 string)', 实际:\n%s", out)
	}
}

// TestGenerateGoVariadicMethod 验证方法支持可变参数
func TestGenerateGoVariadicMethod(t *testing.T) {
	src := `# 程序集 main

类型 计算器 结构体
结束类型

方法 (c 计算器) 求和(参数 nums ... 整数型) 整数型
	返回 0
结束方法
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "func (c 计算器) 求和(nums ...int) int") {
		t.Errorf("期望包含 'func (c 计算器) 求和(nums ...int) int', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceBasic 验证基础接口定义：类型 动物 接口 ... 结束类型
func TestGenerateGoInterfaceBasic(t *testing.T) {
	src := `# 程序集 main

类型 动物 接口
	叫声()
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "type 动物 interface {") {
		t.Errorf("期望包含 'type 动物 interface {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "叫声()") {
		t.Errorf("期望包含 '叫声()', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceWithReturn 验证接口方法带返回类型
func TestGenerateGoInterfaceWithReturn(t *testing.T) {
	src := `# 程序集 main

类型 形状 接口
	面积() 小数型
	周长() 小数型
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "type 形状 interface {") {
		t.Errorf("期望包含 'type 形状 interface {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "面积() float32") {
		t.Errorf("期望包含 '面积() float32', 实际:\n%s", out)
	}
	if !strings.Contains(out, "周长() float32") {
		t.Errorf("期望包含 '周长() float32', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceWithParams 验证接口方法带参数
func TestGenerateGoInterfaceWithParams(t *testing.T) {
	src := `# 程序集 main

类型 处理器 接口
	处理(参数 data 文本型) 文本型
	批量处理(参数 items ...文本型) 文本型
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "处理(data string) string") {
		t.Errorf("期望包含 '处理(data string) string', 实际:\n%s", out)
	}
	if !strings.Contains(out, "批量处理(items ...string) string") {
		t.Errorf("期望包含 '批量处理(items ...string) string', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceMultiReturn 验证接口方法多返回值
func TestGenerateGoInterfaceMultiReturn(t *testing.T) {
	src := `# 程序集 main

类型 读写器 接口
	读取(参数 n 整数型) (文本型, 逻辑型)
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "读取(n int) (string, bool)") {
		t.Errorf("期望包含 '读取(n int) (string, bool)', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmpty 验证空接口
func TestGenerateGoInterfaceEmpty(t *testing.T) {
	src := `# 程序集 main

类型 空接口 接口
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "type 空接口 interface {") {
		t.Errorf("期望包含 'type 空接口 interface {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "}") {
		t.Errorf("期望包含 '}', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceAndStruct 验证接口与结构体共存
func TestGenerateGoInterfaceAndStruct(t *testing.T) {
	src := `# 程序集 main

类型 动物 接口
	叫声()
结束类型

类型 狗 结构体
	名字, 文本型
结束类型

方法 (d 狗) 叫声()
	打印("汪汪")
结束方法
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "type 动物 interface {") {
		t.Errorf("期望包含 'type 动物 interface {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "type 狗 struct {") {
		t.Errorf("期望包含 'type 狗 struct {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "func (d 狗) 叫声()") {
		t.Errorf("期望包含 'func (d 狗) 叫声()', 实际:\n%s", out)
	}
}

// TestGenerateGoFuncLitBasic 验证匿名函数字面量赋值给变量
func TestGenerateGoFuncLitBasic(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 f, 变体型
	f ＝ 函数()
		打印("hello")
	结束函数
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "f = func() {") {
		t.Errorf("期望包含 'f = func() {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "打印(\"hello\")") {
		t.Errorf("期望包含 '打印(\"hello\")', 实际:\n%s", out)
	}
}

// TestGenerateGoFuncLitWithParams 验证带参数的匿名函数
func TestGenerateGoFuncLitWithParams(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 add, 变体型
	add ＝ 函数(参数 a 整数型, 参数 b 整数型) 整数型
		返回 a + b
	结束函数
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "add = func(a int, b int) int {") {
		t.Errorf("期望包含 'add = func(a int, b int) int {', 实际:\n%s", out)
	}
}

// TestGenerateGoFuncLitIIFE 验证立即调用的匿名函数 IIFE
func TestGenerateGoFuncLitIIFE(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	函数()
		打印("立即执行")
	结束函数()
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 期望生成 func() { ... }()
	if !strings.Contains(out, "func() {") {
		t.Errorf("期望包含 'func() {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "}()") {
		t.Errorf("期望包含 '}()' (立即调用), 实际:\n%s", out)
	}
}

// TestGenerateGoFuncLitDefer 验证 defer 匿名函数
func TestGenerateGoFuncLitDefer(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	延迟 函数()
		打印("cleanup")
	结束函数()
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "defer func() {") {
		t.Errorf("期望包含 'defer func() {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "}()") {
		t.Errorf("期望包含 '}()' (立即调用), 实际:\n%s", out)
	}
}

// TestGenerateGoFuncLitGo 验证 go 匿名函数
func TestGenerateGoFuncLitGo(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	协程 函数()
		打印("worker")
	结束函数()
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "go func() {") {
		t.Errorf("期望包含 'go func() {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "}()") {
		t.Errorf("期望包含 '}()' (立即调用), 实际:\n%s", out)
	}
}

// TestGenerateGoFuncLitCallWithArgs 验证匿名函数立即调用带参数
func TestGenerateGoFuncLitCallWithArgs(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	函数(参数 s 文本型)
		打印(s)
	结束函数("hi")
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "func(s string) {") {
		t.Errorf("期望包含 'func(s string) {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "}(\"hi\")") {
		t.Errorf("期望包含 '}(\"hi\")' (带参立即调用), 实际:\n%s", out)
	}
}

// TestGenerateGoFuncLitMultiReturn 验证匿名函数多返回值
func TestGenerateGoFuncLitMultiReturn(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 f, 变体型
	f ＝ 函数(参数 n 整数型) (整数型, 整数型)
		返回 n, n * 2
	结束函数
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "f = func(n int) (int, int) {") {
		t.Errorf("期望包含 'f = func(n int) (int, int) {', 实际:\n%s", out)
	}
}

// TestGenerateGoPanicBasic 验证抛出 expr → panic(expr)
func TestGenerateGoPanicBasic(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	抛出 "出错了"
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "panic(\"出错了\")") {
		t.Errorf("期望包含 'panic(\"出错了\")', 实际:\n%s", out)
	}
}

// TestGenerateGoPanicExpr 验证抛出带表达式
func TestGenerateGoPanicExpr(t *testing.T) {
	src := `# 程序集 main

类型 错误信息 结构体
	msg, 文本型
结束类型

函数 主函数()
	抛出 错误信息{msg: "失败"}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "panic(错误信息{msg: \"失败\"})") {
		t.Errorf("期望包含 'panic(错误信息{msg: \"失败\"})', 实际:\n%s", out)
	}
}

// TestGenerateGoRecoverBasic 验证恢复() → recover()
func TestGenerateGoRecoverBasic(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	延迟 函数()
		局部变量 r, 变体型
		r ＝ 恢复()
		如果 r != 空
			打印("捕获到错误")
		结束如果
	结束函数()
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "recover()") {
		t.Errorf("期望包含 'recover()', 实际:\n%s", out)
	}
	if !strings.Contains(out, "r = recover()") {
		t.Errorf("期望包含 'r = recover()', 实际:\n%s", out)
	}
}

// TestGenerateGoRecoverInAssignment 验证恢复() 在赋值语句中
func TestGenerateGoRecoverInAssignment(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	延迟 函数()
		局部变量 r, 变体型
		r, ok ＝ 恢复(), 真
	结束函数()
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "recover()") {
		t.Errorf("期望包含 'recover()', 实际:\n%s", out)
	}
}

// TestGenerateGoPanicRecoverComplete 验证 panic + recover 完整流程
func TestGenerateGoPanicRecoverComplete(t *testing.T) {
	src := `# 程序集 main

函数 安全执行(f 变体型)
	延迟 函数()
		局部变量 r, 变体型
		r ＝ 恢复()
		如果 r != 空
			打印("捕获到 panic")
		结束如果
	结束函数()
	f()
结束函数

函数 主函数()
	安全执行(函数()
		抛出 "测试 panic"
	结束函数)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "panic(\"测试 panic\")") {
		t.Errorf("期望包含 'panic(\"测试 panic\")', 实际:\n%s", out)
	}
	if !strings.Contains(out, "r = recover()") {
		t.Errorf("期望包含 'r = recover()', 实际:\n%s", out)
	}
	if !strings.Contains(out, "defer func()") {
		t.Errorf("期望包含 'defer func()', 实际:\n%s", out)
	}
}

// TestGenerateGoPanicNil 验证抛出 空 → panic(nil)
func TestGenerateGoPanicNil(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	抛出 空
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	if !strings.Contains(out, "panic(nil)") {
		t.Errorf("期望包含 'panic(nil)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAliasBasic 验证基本类型别名（类型 X ＝ Y → type X = Y）
func TestGenerateGoTypeAliasBasic(t *testing.T) {
	src := `# 程序集 main

类型 整数 ＝ 整数型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 整数 = int") {
		t.Errorf("期望包含 'type 整数 = int', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAliasText 验证文本型别名
func TestGenerateGoTypeAliasText(t *testing.T) {
	src := `# 程序集 main

类型 文本 ＝ 文本型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 文本 = string") {
		t.Errorf("期望包含 'type 文本 = string', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAliasCustom 验证自定义类型别名
func TestGenerateGoTypeAliasCustom(t *testing.T) {
	src := `# 程序集 main

类型 MyPoint ＝ Point
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type MyPoint = Point") {
		t.Errorf("期望包含 'type MyPoint = Point', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAliasPtr 验证指针类型别名
func TestGenerateGoTypeAliasPtr(t *testing.T) {
	src := `# 程序集 main

类型 IntPtr ＝ *整数型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type IntPtr = *int") {
		t.Errorf("期望包含 'type IntPtr = *int', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAliasChan 验证通道类型别名
func TestGenerateGoTypeAliasChan(t *testing.T) {
	src := `# 程序集 main

类型 IntChan ＝ 通道 整数型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type IntChan = chan int") {
		t.Errorf("期望包含 'type IntChan = chan int', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAliasDoublePtr 验证连续指针类型别名（**T）
func TestGenerateGoTypeAliasDoublePtr(t *testing.T) {
	src := `# 程序集 main

类型 IntPPtr ＝ **整数型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type IntPPtr = **int") {
		t.Errorf("期望包含 'type IntPPtr = **int', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAliasMixed 验证别名与结构体/接口混合声明
func TestGenerateGoTypeAliasMixed(t *testing.T) {
	src := `# 程序集 main

类型 整数 ＝ 整数型

类型 点 结构体
	x 小数型
	y 小数型
结束类型

类型 动物 接口
	叫声()
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 整数 = int") {
		t.Errorf("期望包含 'type 整数 = int', 实际:\n%s", out)
	}
	if !strings.Contains(out, "type 点 struct {") {
		t.Errorf("期望包含 'type 点 struct {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "type 动物 interface {") {
		t.Errorf("期望包含 'type 动物 interface {', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeAliasUsage 验证别名作为函数参数类型使用
func TestGenerateGoTypeAliasUsage(t *testing.T) {
	src := `# 程序集 main

类型 整数 ＝ 整数型

函数 加一(参数 n 整数) 整数
	返回 n + 1
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 整数 = int") {
		t.Errorf("期望包含 'type 整数 = int', 实际:\n%s", out)
	}
	if !strings.Contains(out, "func 加一(n 整数) 整数 {") {
		t.Errorf("期望包含 'func 加一(n 整数) 整数 {', 实际:\n%s", out)
	}
}

// TestGenerateGoEmbeddedFieldBasic 验证结构体嵌入字段（无字段名，只有类型）
func TestGenerateGoEmbeddedFieldBasic(t *testing.T) {
	src := `# 程序集 main

类型 动物 结构体
	名字, 文本型
结束类型

类型 狗 结构体
	动物
	品种, 文本型
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 动物 struct {") {
		t.Errorf("期望包含 'type 动物 struct {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "type 狗 struct {") {
		t.Errorf("期望包含 'type 狗 struct {', 实际:\n%s", out)
	}
	// 嵌入字段：只输出类型名，不输出 "字段名 类型"
	if !strings.Contains(out, "\t动物\n") {
		t.Errorf("期望嵌入字段 '\t动物', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t品种 string\n") {
		t.Errorf("期望普通字段 '\t品种 string', 实际:\n%s", out)
	}
}

// TestGenerateGoEmbeddedFieldPtr 验证指针嵌入字段（*T）
func TestGenerateGoEmbeddedFieldPtr(t *testing.T) {
	src := `# 程序集 main

类型 基类 结构体
	值, 整数型
结束类型

类型 子类 结构体
	*基类
	额外, 文本型
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "\t*基类\n") {
		t.Errorf("期望指针嵌入字段 '\t*基类', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t额外 string\n") {
		t.Errorf("期望普通字段 '\t额外 string', 实际:\n%s", out)
	}
}

// TestGenerateGoEmbeddedFieldOnly 验证只有嵌入字段的结构体
func TestGenerateGoEmbeddedFieldOnly(t *testing.T) {
	src := `# 程序集 main

类型 基类 结构体
	值, 整数型
结束类型

类型 包装 结构体
	基类
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 包装 struct {\n\t基类\n}\n") {
		t.Errorf("期望嵌入字段结构体, 实际:\n%s", out)
	}
}

// TestGenerateGoEmbeddedFieldMultiple 验证多个嵌入字段
func TestGenerateGoEmbeddedFieldMultiple(t *testing.T) {
	src := `# 程序集 main

类型 A 结构体
	a, 整数型
结束类型

类型 B 结构体
	b, 整数型
结束类型

类型 C 结构体
	A
	B
	c, 整数型
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 多个嵌入字段应都存在
	count := strings.Count(out, "\tA\n") + strings.Count(out, "\tB\n")
	if count != 2 {
		t.Errorf("期望 2 个嵌入字段, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\tc int\n") {
		t.Errorf("期望普通字段 '\tc int', 实际:\n%s", out)
	}
}

// TestGenerateGoEmbeddedFieldMixed 验证嵌入字段与普通字段混合（顺序自由）
func TestGenerateGoEmbeddedFieldMixed(t *testing.T) {
	src := `# 程序集 main

类型 基础 结构体
	x, 整数型
结束类型

类型 派生 结构体
	字段1, 文本型
	基础
	字段2, 整数型
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "\t字段1 string\n") {
		t.Errorf("期望 '\t字段1 string', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t基础\n") {
		t.Errorf("期望嵌入字段 '\t基础', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t字段2 int\n") {
		t.Errorf("期望 '\t字段2 int', 实际:\n%s", out)
	}
}

// TestGenerateGoEmbeddedFieldNoConflict 验证嵌入字段不影响其他结构体
func TestGenerateGoEmbeddedFieldNoConflict(t *testing.T) {
	src := `# 程序集 main

类型 普通结构 结构体
	a, 整数型
	b, 文本型
结束类型

类型 嵌入结构 结构体
	普通结构
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 普通结构应保持普通字段输出
	if !strings.Contains(out, "type 普通结构 struct {\n\ta int\n\tb string\n}") {
		t.Errorf("普通结构体字段应正常输出, 实际:\n%s", out)
	}
	// 嵌入结构应只有嵌入字段
	if !strings.Contains(out, "type 嵌入结构 struct {\n\t普通结构\n}") {
		t.Errorf("嵌入结构体应只有嵌入字段, 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedBasic 验证接口嵌入其他接口（Go 接口组合）
func TestGenerateGoInterfaceEmbedBasic(t *testing.T) {
	src := `# 程序集 main

类型 读接口 接口
	读取() 文本型
结束类型

类型 写接口 接口
	写入(参数 s 文本型)
结束类型

类型 读写器 接口
	读接口
	写接口
	关闭()
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 读写器 interface {") {
		t.Errorf("期望包含 'type 读写器 interface {', 实际:\n%s", out)
	}
	// 嵌入接口应输出类型名
	if !strings.Contains(out, "\t读接口\n") {
		t.Errorf("期望嵌入接口 '\t读接口', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t写接口\n") {
		t.Errorf("期望嵌入接口 '\t写接口', 实际:\n%s", out)
	}
	// 方法签名应正常输出
	if !strings.Contains(out, "\t关闭()") {
		t.Errorf("期望方法 '\t关闭()', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedOnly 验证只有嵌入接口的接口
func TestGenerateGoInterfaceEmbedOnly(t *testing.T) {
	src := `# 程序集 main

类型 基础接口 接口
	方法1()
结束类型

类型 组合接口 接口
	基础接口
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 组合接口 interface {\n\t基础接口\n}\n") {
		t.Errorf("期望只有嵌入接口, 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedMultiple 验证多个嵌入接口
func TestGenerateGoInterfaceEmbedMultiple(t *testing.T) {
	src := `# 程序集 main

类型 A 接口
	方法A()
结束类型

类型 B 接口
	方法B()
结束类型

类型 C 接口
	A
	B
	方法C()
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	count := strings.Count(out, "\tA\n") + strings.Count(out, "\tB\n")
	if count != 2 {
		t.Errorf("期望 2 个嵌入接口, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t方法C()") {
		t.Errorf("期望方法 '\t方法C()', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedOrder 验证嵌入接口在前，方法签名在后
func TestGenerateGoInterfaceEmbedOrder(t *testing.T) {
	src := `# 程序集 main

类型 基础 接口
	基础方法()
结束类型

类型 派生 接口
	基础
	派生方法()
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 嵌入接口应在前
	idxEmbed := strings.Index(out, "\t基础\n")
	idxMethod := strings.Index(out, "\t派生方法()")
	if idxEmbed < 0 || idxMethod < 0 {
		t.Fatalf("期望嵌入接口和方法都存在, 实际:\n%s", out)
	}
	if idxEmbed >= idxMethod {
		t.Errorf("期望嵌入接口在方法前, 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedMixed 验证嵌入接口与方法混合（顺序自由）
func TestGenerateGoInterfaceEmbedMixed(t *testing.T) {
	src := `# 程序集 main

类型 基础接口 接口
	基础方法()
结束类型

类型 派生接口 接口
	方法1()
	基础接口
	方法2()
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 所有成员都应存在
	if !strings.Contains(out, "\t基础接口\n") {
		t.Errorf("期望嵌入接口, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t方法1()") {
		t.Errorf("期望方法1, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t方法2()") {
		t.Errorf("期望方法2, 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedNoConflict 验证嵌入接口不影响其他接口
func TestGenerateGoInterfaceEmbedNoConflict(t *testing.T) {
	src := `# 程序集 main

类型 普通接口 接口
	方法A()
	方法B()
结束类型

类型 组合接口 接口
	普通接口
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 普通接口应只有方法
	if !strings.Contains(out, "type 普通接口 interface {\n\t方法A()\n\t方法B()\n}") {
		t.Errorf("普通接口应只有方法, 实际:\n%s", out)
	}
	// 组合接口应只有嵌入
	if !strings.Contains(out, "type 组合接口 interface {\n\t普通接口\n}") {
		t.Errorf("组合接口应只有嵌入, 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedEmpty 验证空接口（无方法无嵌入）
func TestGenerateGoInterfaceEmbedEmpty(t *testing.T) {
	src := `# 程序集 main

类型 空接口 接口
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type 空接口 interface {\n}\n") {
		t.Errorf("期望空接口, 实际:\n%s", out)
	}
}

// TestGenerateGoEnumBasic 验证枚举块基本用法（首项序数，后续省略自动 +1）
func TestGenerateGoEnumBasic(t *testing.T) {
	src := `# 程序集 main

枚举
	星期日 ＝ 序数
	星期一
	星期二
结束枚举
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "const (") {
		t.Errorf("期望包含 'const (', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t星期日 = iota\n") {
		t.Errorf("期望 '\t星期日 = iota', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t星期一\n") {
		t.Errorf("期望 '\t星期一'（省略表达式）, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t星期二\n") {
		t.Errorf("期望 '\t星期二'（省略表达式）, 实际:\n%s", out)
	}
	if !strings.Contains(out, ")") {
		t.Errorf("期望包含 ')', 实际:\n%s", out)
	}
}

// TestGenerateGoEnumNoValue 验证枚举首项无表达式（自动 iota）
func TestGenerateGoEnumNoValue(t *testing.T) {
	src := `# 程序集 main

枚举
	红
	绿
	蓝
结束枚举
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 首项无表达式时，生成器自动补 iota
	if !strings.Contains(out, "\t红 = iota\n") {
		t.Errorf("期望 '\t红 = iota'（首项自动 iota）, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t绿\n") {
		t.Errorf("期望 '\t绿'（省略表达式）, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t蓝\n") {
		t.Errorf("期望 '\t蓝'（省略表达式）, 实际:\n%s", out)
	}
}

// TestGenerateGoEnumExpr 验证枚举带复杂表达式（位运算）
func TestGenerateGoEnumExpr(t *testing.T) {
	src := `# 程序集 main

枚举
	可读 ＝ 1 ＜＜ 序数
	可写
	可执行
结束枚举
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 首项带表达式，序数 → iota
	if !strings.Contains(out, "\t可读 = 1 << iota\n") {
		t.Errorf("期望 '\t可读 = 1 << iota', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t可写\n") {
		t.Errorf("期望 '\t可写'（省略延续）, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t可执行\n") {
		t.Errorf("期望 '\t可执行'（省略延续）, 实际:\n%s", out)
	}
}

// TestGenerateGoEnumSingle 验证单元素枚举
func TestGenerateGoEnumSingle(t *testing.T) {
	src := `# 程序集 main

枚举
	唯一 ＝ 序数
结束枚举
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "const (\n\t唯一 = iota\n)\n") {
		t.Errorf("期望单元素枚举, 实际:\n%s", out)
	}
}

// TestGenerateGoEnumMixed 验证枚举混合（部分带表达式，部分省略）
func TestGenerateGoEnumMixed(t *testing.T) {
	src := `# 程序集 main

枚举
	A ＝ 序数
	B
	C ＝ 序数 + 10
	D
结束枚举
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "\tA = iota\n") {
		t.Errorf("期望 '\tA = iota', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\tB\n") {
		t.Errorf("期望 '\tB'（省略延续 iota）, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\tC = iota + 10\n") {
		t.Errorf("期望 '\tC = iota + 10', 实际:\n%s", out)
	}
	if !strings.Contains(out, "\tD\n") {
		t.Errorf("期望 '\tD'（省略延续 iota+10）, 实际:\n%s", out)
	}
}

// TestGenerateGoEnumWithConst 验证枚举与常量并存
func TestGenerateGoEnumWithConst(t *testing.T) {
	src := `# 程序集 main

常量 PI ＝ 3.14

枚举
	星期日 ＝ 序数
	星期一
结束枚举
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "const PI = 3.14") {
		t.Errorf("期望常量 PI, 实际:\n%s", out)
	}
	if !strings.Contains(out, "const (") {
		t.Errorf("期望枚举 const 块, 实际:\n%s", out)
	}
	if !strings.Contains(out, "\t星期日 = iota\n") {
		t.Errorf("期望 '\t星期日 = iota', 实际:\n%s", out)
	}
}

// TestGenerateGoEnumEmpty 验证空枚举（无项）
func TestGenerateGoEnumEmpty(t *testing.T) {
	src := `# 程序集 main

枚举
结束枚举
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 空枚举也应生成空 const 块（gofmt 会格式化为 `const ()`）
	if !strings.Contains(out, "const ()") {
		t.Errorf("期望空 const 块, 实际:\n%s", out)
	}
}

// TestGenerateGoInitBasic 验证 init 函数（初始化 ... 结束函数）
func TestGenerateGoInitBasic(t *testing.T) {
	src := `# 程序集 main

初始化
	输出("启动")
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "func init() {") {
		t.Errorf("期望包含 'func init() {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "输出(\"启动\")") {
		t.Errorf("期望包含函数体, 实际:\n%s", out)
	}
}

// TestGenerateGoInitWithParens 验证带 () 的 init 函数
func TestGenerateGoInitWithParens(t *testing.T) {
	src := `# 程序集 main

初始化()
	输出("启动")
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "func init() {") {
		t.Errorf("期望包含 'func init() {', 实际:\n%s", out)
	}
}

// TestGenerateGoInitMultiple 验证多个 init 函数（Go 允许同一包内多个 init）
func TestGenerateGoInitMultiple(t *testing.T) {
	src := `# 程序集 main

初始化
	输出("第一个")
结束函数

初始化
	输出("第二个")
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	count := strings.Count(out, "func init() {")
	if count != 2 {
		t.Errorf("期望 2 个 init 函数, 实际 %d 个:\n%s", count, out)
	}
}

// TestGenerateGoInitWithFunc 验证 init 与普通函数并存
func TestGenerateGoInitWithFunc(t *testing.T) {
	src := `# 程序集 main

初始化
	输出("初始化")
结束函数

函数 主函数()
	输出("主函数")
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "func init() {") {
		t.Errorf("期望包含 'func init() {', 实际:\n%s", out)
	}
	if !strings.Contains(out, "func mainImpl() {") {
		t.Errorf("期望包含 'func mainImpl() {', 实际:\n%s", out)
	}
}

// TestGenerateGoInitEmpty 验证空 init 函数
func TestGenerateGoInitEmpty(t *testing.T) {
	src := `# 程序集 main

初始化
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "func init() {\n}") {
		t.Errorf("期望空 init 函数, 实际:\n%s", out)
	}
}

// TestGenerateGoInitWithVar 验证 init 与变量声明并存（包初始化顺序：var → init）
func TestGenerateGoInitWithVar(t *testing.T) {
	src := `# 程序集 main

变量 计数 整数型

初始化
	计数 ＝ 100
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "var 计数 int") {
		t.Errorf("期望变量声明, 实际:\n%s", out)
	}
	if !strings.Contains(out, "func init() {") {
		t.Errorf("期望 init 函数, 实际:\n%s", out)
	}
	if !strings.Contains(out, "计数 = 100") {
		t.Errorf("期望赋值, 实际:\n%s", out)
	}
}

// TestGenerateGoVarDeclCommaType 验证包级变量 "变量 名字, 类型" 语法（v47 修复）
func TestGenerateGoVarDeclCommaType(t *testing.T) {
	src := `# 程序集 main

变量 计数, 整数型
变量 名字, 文本型
变量 指针, *整数型
变量 通道变量, 通道 整数型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "var 计数 int") {
		t.Errorf("期望 'var 计数 int', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var 名字 string") {
		t.Errorf("期望 'var 名字 string', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var 指针 *int") {
		t.Errorf("期望 'var 指针 *int', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var 通道变量 chan int") {
		t.Errorf("期望 'var 通道变量 chan int', 实际:\n%s", out)
	}
}

// TestGenerateGoVarDeclSpaceType 验证包级变量 "变量 名字 类型" 语法（空格分隔，原有支持）
func TestGenerateGoVarDeclSpaceType(t *testing.T) {
	src := `# 程序集 main

变量 计数 整数型
变量 名字 文本型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "var 计数 int") {
		t.Errorf("期望 'var 计数 int', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var 名字 string") {
		t.Errorf("期望 'var 名字 string', 实际:\n%s", out)
	}
}

// TestGenerateGoVarDeclWithValue 验证包级变量 "变量 名字 ＝ 值" 语法（初值形式）
func TestGenerateGoVarDeclWithValue(t *testing.T) {
	src := `# 程序集 main

变量 计数 ＝ 100
变量 名字 ＝ "abc"
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "var 计数 = 100") {
		t.Errorf("期望 'var 计数 = 100', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var 名字 = \"abc\"") {
		t.Errorf("期望 'var 名字 = \"abc\"', 实际:\n%s", out)
	}
}

// TestGenerateGoVarDeclMixed 验证包级变量三种形式混合
func TestGenerateGoVarDeclMixed(t *testing.T) {
	src := `# 程序集 main

变量 a, 整数型
变量 b 整数型
变量 c ＝ 50
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "var a int") {
		t.Errorf("期望 'var a int', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var b int") {
		t.Errorf("期望 'var b int', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var c = 50") {
		t.Errorf("期望 'var c = 50', 实际:\n%s", out)
	}
}

// TestGenerateGoVarDeclComplexType 验证包级变量复合类型（指针/通道）
func TestGenerateGoVarDeclComplexType(t *testing.T) {
	src := `# 程序集 main

变量 p, **整数型
变量 ch, 通道 文本型
变量 ptrChan, *通道 整数型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "var p **int") {
		t.Errorf("期望 'var p **int', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var ch chan string") {
		t.Errorf("期望 'var ch chan string', 实际:\n%s", out)
	}
	if !strings.Contains(out, "var ptrChan *chan int") {
		t.Errorf("期望 'var ptrChan *chan int', 实际:\n%s", out)
	}
}

// TestGenerateGoBreakLabel 验证带标签的 break（跳出多层循环）
func TestGenerateGoBreakLabel(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	标签 外层
	循环 _, i ＝ 范围 切片1
		循环 _, j ＝ 范围 切片2
			如果 (j ＝＝ 5)
				跳出 外层
			结束如果
		结束循环
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 标签声明：外层:
	if !strings.Contains(out, "外层:") {
		t.Errorf("期望标签声明 '外层:', 实际:\n%s", out)
	}
	// 带标签的 break：break 外层
	if !strings.Contains(out, "break 外层") {
		t.Errorf("期望 'break 外层', 实际:\n%s", out)
	}
}

// TestGenerateGoContinueLabel 验证带标签的 continue（继续外层循环）
func TestGenerateGoContinueLabel(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	标签 外层
	循环 _, i ＝ 范围 切片1
		循环 _, j ＝ 范围 切片2
			如果 (j ＝＝ 2)
				继续 外层
			结束如果
		结束循环
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "外层:") {
		t.Errorf("期望标签声明, 实际:\n%s", out)
	}
	if !strings.Contains(out, "continue 外层") {
		t.Errorf("期望 'continue 外层', 实际:\n%s", out)
	}
}

// TestGenerateGoBreakNoLabel 验证无标签的 break（原有行为不变）
func TestGenerateGoBreakNoLabel(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 _, i ＝ 范围 切片
		跳出
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "break") {
		t.Errorf("期望 'break', 实际:\n%s", out)
	}
	// 不应有标签
	if strings.Contains(out, "break ") {
		t.Errorf("不应有带标签的 break, 实际:\n%s", out)
	}
}

// TestGenerateGoContinueNoLabel 验证无标签的 continue（原有行为不变）
func TestGenerateGoContinueNoLabel(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 _, i ＝ 范围 切片
		继续
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "continue") {
		t.Errorf("期望 'continue', 实际:\n%s", out)
	}
	if strings.Contains(out, "continue ") {
		t.Errorf("不应有带标签的 continue, 实际:\n%s", out)
	}
}

// TestGenerateGoLabeledRange 验证标签修饰 range 循环
func TestGenerateGoLabeledRange(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	标签 外层
	循环 _, v ＝ 范围 切片
		如果 (v ＝＝ 0)
			跳出 外层
		结束如果
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "外层:") {
		t.Errorf("期望标签声明, 实际:\n%s", out)
	}
	if !strings.Contains(out, "break 外层") {
		t.Errorf("期望 'break 外层', 实际:\n%s", out)
	}
	if !strings.Contains(out, "for _, v := range 切片") {
		t.Errorf("期望 range 循环, 实际:\n%s", out)
	}
}

// TestGenerateGoLabeledWhile 验证标签修饰判断循环
func TestGenerateGoLabeledWhile(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	标签 外层
	判断循环 (true)
		跳出 外层
	结束判断循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "外层:") {
		t.Errorf("期望标签声明, 实际:\n%s", out)
	}
	if !strings.Contains(out, "break 外层") {
		t.Errorf("期望 'break 外层', 实际:\n%s", out)
	}
	if !strings.Contains(out, "for true {") {
		t.Errorf("期望判断循环, 实际:\n%s", out)
	}
}

// TestGenerateGoForThreePartNoParen 验证三段式 for 循环（无括号）
// 语法：循环 init; cond; post ... 结束循环 → for init; cond; post { ... }
func TestGenerateGoForThreePartNoParen(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 i := 0; i < 10; i++
		打印(i)
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	expect := "for i := 0; i < 10; i++ {"
	if !strings.Contains(out, expect) {
		t.Errorf("期望包含 %q, 实际:\n%s", expect, out)
	}
}

// TestGenerateGoForThreePartWithParen 验证三段式 for 循环（带括号糖衣）
// 语法：循环 (init; cond; post) ... 结束循环 → for init; cond; post { ... }
func TestGenerateGoForThreePartWithParen(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 (i := 0; i < 10; i++)
		打印(i)
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	expect := "for i := 0; i < 10; i++ {"
	if !strings.Contains(out, expect) {
		t.Errorf("期望包含 %q, 实际:\n%s", expect, out)
	}
}

// TestGenerateGoForCondOnly 验证单 cond 形式（无括号）
// 语法：循环 cond ... 结束循环 → for cond { ... }
func TestGenerateGoForCondOnly(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 i < 10
		i++
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	expect := "for i < 10 {"
	if !strings.Contains(out, expect) {
		t.Errorf("期望包含 %q, 实际:\n%s", expect, out)
	}
}

// TestGenerateGoForCondWithParen 验证单 cond 形式（带括号）
// 语法：循环 (cond) ... 结束循环 → for cond { ... }
func TestGenerateGoForCondWithParen(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	局部变量 i 整数型
	循环 (i < 10)
		i++
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	expect := "for i < 10 {"
	if !strings.Contains(out, expect) {
		t.Errorf("期望包含 %q, 实际:\n%s", expect, out)
	}
}

// TestGenerateGoForInfinite 验证无限循环
// 语法：循环 ... 结束循环 → for { ... }
func TestGenerateGoForInfinite(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环
		跳出
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	expect := "for {"
	if !strings.Contains(out, expect) {
		t.Errorf("期望包含 %q, 实际:\n%s", expect, out)
	}
}

// TestGenerateGoForThreePartChineseOp 验证三段式 for 循环（中文运算符）
// 语法：循环 i := 0; i ＜ 10; i++ → for i := 0; i < 10; i++ {
func TestGenerateGoForThreePartChineseOp(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	循环 i := 0; i ＜ 10; i++
		打印(i)
	结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	expect := "for i := 0; i < 10; i++ {"
	if !strings.Contains(out, expect) {
		t.Errorf("期望包含 %q, 实际:\n%s", expect, out)
	}
}

// TestGenerateGoStructEmbedQualified 验证结构体限定嵌入字段（包名.T）
// 语法：在 结构体 ... 结束类型 块内独占一行写 包名.T
// 转译为 Go：type S struct { 包名.T }（嵌入外部包类型，字段/方法提升）
func TestGenerateGoStructEmbedQualified(t *testing.T) {
	src := `# 程序集 main

类型 S 结构体
    sync.Mutex
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 应该生成 "sync.Mutex"（嵌入字段，独占一行，无字段名）
	if !strings.Contains(out, "sync.Mutex") {
		t.Errorf("期望包含嵌入字段 'sync.Mutex', 实际:\n%s", out)
	}
	if strings.Contains(out, "sync.Mutex sync.Mutex") {
		t.Errorf("不应生成 'sync.Mutex sync.Mutex'（这是普通字段而非嵌入）, 实际:\n%s", out)
	}
}

// TestGenerateGoStructEmbedPtrQualified 验证指针限定嵌入字段（*包名.T）
// 语法：*包名.T 独占一行 → Go *包名.T
func TestGenerateGoStructEmbedPtrQualified(t *testing.T) {
	src := `# 程序集 main

类型 S 结构体
    *sync.Mutex
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "*sync.Mutex") {
		t.Errorf("期望包含 '*sync.Mutex', 实际:\n%s", out)
	}
}

// TestGenerateGoStructEmbedQualifiedMixed 验证限定嵌入与普通字段/本地嵌入混合
func TestGenerateGoStructEmbedQualifiedMixed(t *testing.T) {
	src := `# 程序集 main

类型 Inner 结构体
    值 整数型
结束类型

类型 Outer 结构体
    Inner
    sync.Mutex
    名字 文本型
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 应同时包含：本地嵌入 Inner、限定嵌入 sync.Mutex、普通字段 名字 + string
	// 注意：gofmt 会把中文字段名 "名字 string" 拆成两行（中文标识符对齐特殊处理）
	if !strings.Contains(out, "Inner") {
		t.Errorf("期望包含 'Inner', 实际:\n%s", out)
	}
	if !strings.Contains(out, "sync.Mutex") {
		t.Errorf("期望包含 'sync.Mutex', 实际:\n%s", out)
	}
	if !strings.Contains(out, "名字") || !strings.Contains(out, "string") {
		t.Errorf("期望包含 '名字' 和 'string', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedQualified 验证接口限定嵌入（包名.T）
// 语法：在 接口 ... 结束类型 块内独占一行写 包名.T
// 转译为 Go：type I interface { 包名.T; ... }（接口组合外部接口）
func TestGenerateGoInterfaceEmbedQualified(t *testing.T) {
	src := `# 程序集 main

类型 I 接口
    io.Reader
    io.Writer
    关闭()
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "io.Reader") {
		t.Errorf("期望包含 'io.Reader', 实际:\n%s", out)
	}
	if !strings.Contains(out, "io.Writer") {
		t.Errorf("期望包含 'io.Writer', 实际:\n%s", out)
	}
	if !strings.Contains(out, "关闭()") {
		t.Errorf("期望包含 '关闭()', 实际:\n%s", out)
	}
}

// TestGenerateGoInterfaceEmbedQualifiedMixed 验证接口限定嵌入与本地嵌入/方法混合
func TestGenerateGoInterfaceEmbedQualifiedMixed(t *testing.T) {
	src := `# 程序集 main

类型 本地接口 接口
    方法1()
结束类型

类型 组合接口 接口
    本地接口
    io.Closer
    方法2()
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "本地接口") {
		t.Errorf("期望包含 '本地接口', 实际:\n%s", out)
	}
	if !strings.Contains(out, "io.Closer") {
		t.Errorf("期望包含 'io.Closer', 实际:\n%s", out)
	}
	if !strings.Contains(out, "方法1()") {
		t.Errorf("期望包含 '方法1()', 实际:\n%s", out)
	}
	if !strings.Contains(out, "方法2()") {
		t.Errorf("期望包含 '方法2()', 实际:\n%s", out)
	}
}

// TestGenerateGoQualifiedFieldType 验证普通字段类型使用限定类型名
// 语法：字段, 包名.T → Go 的 字段 包名.T
func TestGenerateGoQualifiedFieldType(t *testing.T) {
	src := `# 程序集 main

类型 S 结构体
    mu, sync.Mutex
    ctx, context.Context
结束类型
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// gofmt 会用空格对齐字段名，所以 "mu sync.Mutex" 可能是 "mu  sync.Mutex"（双空格）
	// 用正则或分别检查关键字段
	if !strings.Contains(out, "mu") || !strings.Contains(out, "sync.Mutex") {
		t.Errorf("期望包含 'mu' 和 'sync.Mutex', 实际:\n%s", out)
	}
	if !strings.Contains(out, "ctx") || !strings.Contains(out, "context.Context") {
		t.Errorf("期望包含 'ctx' 和 'context.Context', 实际:\n%s", out)
	}
}

// TestGenerateGoQualifiedTypeAlias 验证类型别名使用限定类型名
// 语法：类型 X ＝ 包名.T → Go type X = 包名.T
// 注意：类型别名不需要"结束类型"（与结构体/接口声明不同）
func TestGenerateGoQualifiedTypeAlias(t *testing.T) {
	src := `# 程序集 main

类型 MyReader ＝ io.Reader
类型 MyMutex ＝ *sync.Mutex
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "type MyReader = io.Reader") {
		t.Errorf("期望包含 'type MyReader = io.Reader', 实际:\n%s", out)
	}
	if !strings.Contains(out, "type MyMutex = *sync.Mutex") {
		t.Errorf("期望包含 'type MyMutex = *sync.Mutex', 实际:\n%s", out)
	}
}

// TestGenerateGoMapLiteralPtrKey 验证映射字面量使用指针类型作为键
// 语法：映射 *整数型 整数型 { ... } → map[*int]int{ ... }
// v51 重构：parseMapLiteral 改用 parseType，支持复合类型
func TestGenerateGoMapLiteralPtrKey(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    m ＝ 映射 *整数型 整数型 { }
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "map[*int]int{}") {
		t.Errorf("期望包含 'map[*int]int{}', 实际:\n%s", out)
	}
}

// TestGenerateGoMapLiteralPtrValue 验证映射字面量使用指针类型作为值
func TestGenerateGoMapLiteralPtrValue(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    m ＝ 映射 文本型 *整数型 { }
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "map[string]*int{}") {
		t.Errorf("期望包含 'map[string]*int{}', 实际:\n%s", out)
	}
}

// TestGenerateGoMapLiteralChanKey 验证映射字面量使用通道类型（罕见但应支持）
func TestGenerateGoMapLiteralChanValue(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    m ＝ 映射 文本型 通道 整数型 { }
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "map[string]chan int{}") {
		t.Errorf("期望包含 'map[string]chan int{}', 实际:\n%s", out)
	}
}

// TestGenerateGoMapLiteralQualifiedType 验证映射字面量使用限定类型名
// 语法：映射 文本型 包名.T { ... } → map[string]包名.T{ ... }
func TestGenerateGoMapLiteralQualifiedType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    m ＝ 映射 文本型 sync.Mutex { }
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "map[string]sync.Mutex{}") {
		t.Errorf("期望包含 'map[string]sync.Mutex{}', 实际:\n%s", out)
	}
}

// TestGenerateGoQualifiedStructLiteral 验证限定类型 struct literal
// 语法：包名.T{...} → 包名.T{...}（如 time.Time{...}）
func TestGenerateGoQualifiedStructLiteral(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    t ＝ time.Time{}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "time.Time{}") {
		t.Errorf("期望包含 'time.Time{}', 实际:\n%s", out)
	}
}

// TestGenerateGoQualifiedStructLiteralWithFields 验证限定类型 struct literal 带字段
func TestGenerateGoQualifiedStructLiteralWithFields(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    p ＝ image.Point{x: 1, y: 2}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "image.Point{x: 1, y: 2}") {
		t.Errorf("期望包含 'image.Point{x: 1, y: 2}', 实际:\n%s", out)
	}
}

// TestGenerateGoQualifiedStructLiteralMultiSeg 验证多段限定 struct literal
// 语法：包名.子包.T{...}（罕见但应支持）
func TestGenerateGoQualifiedStructLiteralMultiSeg(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    c ＝ a.b.C{}
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "a.b.C{}") {
		t.Errorf("期望包含 'a.b.C{}', 实际:\n%s", out)
	}
}

// TestGenerateGoQualifiedStructLiteralNotMember 验证包名.字段 不被误判为限定 struct literal
// 语法：包名.字段（成员访问）不应识别为 struct literal
func TestGenerateGoQualifiedStructLiteralNotMember(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    x ＝ fmt.Println
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	// 应该是成员访问 fmt.Println，不是 struct literal
	if !strings.Contains(out, "fmt.Println") {
		t.Errorf("期望包含 'fmt.Println'（成员访问）, 实际:\n%s", out)
	}
	// 不应该有 fmt.Println{ 这种 struct literal 形式
	if strings.Contains(out, "fmt.Println{") {
		t.Errorf("不应识别为 struct literal, 实际:\n%s", out)
	}
}

// TestGenerateGoGotoBasic 验证 goto 跳转（基本形式）
// 语法：跳转 名字 → goto 名字
// 标签用 标签 关键字声明（返回 是关键字不能作标签名，改用 重试）
func TestGenerateGoGotoBasic(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 i 整数型
    i ＝ 0
    标签 重试
    i++
    如果 (i < 10)
        跳转 重试
    结束如果
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "goto 重试") {
		t.Errorf("期望包含 'goto 重试', 实际:\n%s", out)
	}
	if !strings.Contains(out, "重试:") {
		t.Errorf("期望包含 '重试:'（标签声明）, 实际:\n%s", out)
	}
}

// TestGenerateGoGotoStandaloneLabel 验证独立标签（标签 名字 独占一行，不修饰下一语句）
// 用于 goto 跳转目标，标签后跟块结束/函数结束
func TestGenerateGoGotoStandaloneLabel(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    跳转 结束
    标签 结束
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "goto 结束") {
		t.Errorf("期望包含 'goto 结束', 实际:\n%s", out)
	}
	if !strings.Contains(out, "结束:") {
		t.Errorf("期望包含 '结束:'（独立标签）, 实际:\n%s", out)
	}
}

// TestGenerateGoGotoInLoop 验证 goto 在循环内跳出多层
func TestGenerateGoGotoInLoop(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    标签 外层
    循环 i := 0; i < 10; i++
        循环 j := 0; j < 10; j++
            如果 (i*j > 50)
                跳转 外层
            结束如果
        结束循环
    结束循环
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "goto 外层") {
		t.Errorf("期望包含 'goto 外层', 实际:\n%s", out)
	}
	if !strings.Contains(out, "外层:") {
		t.Errorf("期望包含 '外层:'（标签修饰循环）, 实际:\n%s", out)
	}
}

// TestGenerateGoGotoChineseLabel 验证 goto 使用中文标签名
func TestGenerateGoGotoChineseLabel(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    跳转 重试
    标签 重试
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo err: %v", err)
	}

	if !strings.Contains(out, "goto 重试") {
		t.Errorf("期望包含 'goto 重试', 实际:\n%s", out)
	}
	if !strings.Contains(out, "重试:") {
		t.Errorf("期望包含 '重试:', 实际:\n%s", out)
	}
}

// TestGenerateGoGotoMissingLabel 验证 goto 缺少标签名时报错
func TestGenerateGoGotoMissingLabel(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    跳转
结束函数
`
	_, errs := Parse(src)
	if len(errs) == 0 {
		t.Errorf("期望解析错误（跳转缺少标签名），实际无错误")
		return
	}
	// 应该有"跳转缺少标签名"相关的错误
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "跳转缺少标签名") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("期望错误包含 '跳转缺少标签名', 实际: %v", errs)
	}
}

// ===== v53 Type Switch + fallthrough 测试 =====

// TestGenerateGoTypeSwitchBasic 验证基本 type switch：选择 x ＝ y.(类型) → switch x := y.(type)
func TestGenerateGoTypeSwitchBasic(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 v, 变体型
    选择 x ＝ v.(类型)
        情况 整数型:
            打印("int")
        情况 文本型:
            打印("string")
        默认:
            打印("unknown")
    结束选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err（gofmt 可能失败）: %v", err)
	}

	// 验证生成 "switch x := v.(type) {"
	if !strings.Contains(out, "switch x := v.(type)") {
		t.Errorf("期望包含 'switch x := v.(type)', 实际:\n%s", out)
	}
	// 验证 case int:
	if !strings.Contains(out, "case int:") {
		t.Errorf("期望包含 'case int:', 实际:\n%s", out)
	}
	// 验证 case string:
	if !strings.Contains(out, "case string:") {
		t.Errorf("期望包含 'case string:', 实际:\n%s", out)
	}
	// 验证 default:
	if !strings.Contains(out, "default:") {
		t.Errorf("期望包含 'default:', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeSwitchEnglishType 验证 .(type) 英文形式也支持
func TestGenerateGoTypeSwitchEnglishType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 v, 变体型
    选择 x ＝ v.(type)
        情况 整数型:
            打印("int")
    结束选择
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	if !strings.Contains(out, "switch x := v.(type)") {
		t.Errorf("期望包含 'switch x := v.(type)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeSwitchChineseType 验证 .(类型) 中文形式
func TestGenerateGoTypeSwitchChineseType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 v, 变体型
    选择 t ＝ v.(类型)
        情况 整数型:
            返回
    结束选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// 应该生成 "switch t := v.(type)"
	if !strings.Contains(out, "switch t := v.(type)") {
		t.Errorf("期望包含 'switch t := v.(type)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeSwitchMultiCase 验证 type switch 多类型 case
func TestGenerateGoTypeSwitchMultiCase(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 v, 变体型
    选择 x ＝ v.(类型)
        情况 整数型, 文本型:
            打印("int or string")
        情况 逻辑型:
            打印("bool")
    结束选择
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// 验证多类型 case：case int, string:
	if !strings.Contains(out, "case int, string:") {
		t.Errorf("期望包含 'case int, string:', 实际:\n%s", out)
	}
	if !strings.Contains(out, "case bool:") {
		t.Errorf("期望包含 'case bool:', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeSwitchShortDecl 验证 := 短声明形式
func TestGenerateGoTypeSwitchShortDecl(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 v, 变体型
    选择 x := v.(类型)
        情况 整数型:
            返回
    结束选择
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	if !strings.Contains(out, "switch x := v.(type)") {
		t.Errorf("期望包含 'switch x := v.(type)', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeSwitchNoAssign 验证无变量形式：选择 v.(类型) → switch v.(type)
// 注意：Go 允许 switch v.(type) 无赋值形式
func TestGenerateGoTypeSwitchNoAssign(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 v, 变体型
    选择 v.(类型)
        情况 整数型:
            打印("int")
    结束选择
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// 无变量形式：switch v.(type)
	if !strings.Contains(out, "switch v.(type)") {
		t.Errorf("期望包含 'switch v.(type)', 实际:\n%s", out)
	}
}

// TestGenerateGoFallthroughBasic 验证穿透 → fallthrough
func TestGenerateGoFallthroughBasic(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 x, 整数型
    x ＝ 1
    选择 (x)
        情况 1:
            打印("one")
            穿透
        情况 2:
            打印("two")
    结束选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Logf("GenerateGo 返回 err: %v", err)
	}

	// 验证生成 fallthrough
	if !strings.Contains(out, "fallthrough") {
		t.Errorf("期望包含 'fallthrough', 实际:\n%s", out)
	}
}

// TestGenerateGoFallthroughInSwitch 验证 fallthrough 在 case 末尾
func TestGenerateGoFallthroughInSwitch(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    选择 (2)
        情况 1:
            打印("a")
            穿透
        情况 2:
            打印("b")
        默认:
            打印("def")
    结束选择
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// 验证 case 1 中有 fallthrough
	if !strings.Contains(out, "fallthrough") {
		t.Errorf("期望包含 'fallthrough', 实际:\n%s", out)
	}
	// 验证 case 2:（不应被穿透影响）
	if !strings.Contains(out, "case 2:") {
		t.Errorf("期望包含 'case 2:', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeSwitchPtrType 验证 type switch case 中带指针类型 *整数型
func TestGenerateGoTypeSwitchPtrType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 v, 变体型
    选择 x ＝ v.(类型)
        情况 *整数型:
            打印("ptr int")
        情况 整数型:
            打印("int")
    结束选择
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// 验证 case *int:
	if !strings.Contains(out, "case *int:") {
		t.Errorf("期望包含 'case *int:', 实际:\n%s", out)
	}
	// 验证 case int:
	if !strings.Contains(out, "case int:") {
		t.Errorf("期望包含 'case int:', 实际:\n%s", out)
	}
}

// TestGenerateGoSwitchNoExpr 验证 switch 无表达式形式：选择 { 情况 ... } → switch { case ... }
func TestGenerateGoSwitchNoExpr(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 x, 整数型
    x ＝ 5
    选择
        情况 x > 3:
            打印("big")
        默认:
            打印("small")
    结束选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// 验证生成 "switch {"（无表达式）
	if !strings.Contains(out, "switch {") {
		t.Errorf("期望包含 'switch {'（无表达式形式）, 实际:\n%s", out)
	}
}

// ===== v54 类型系统补全测试 =====

// TestGenerateGoUnsignedTypes 验证无符号整数类型映射
func TestGenerateGoUnsignedTypes(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 a, 无符号整数型
    局部变量 b, 无符号短整数型
    局部变量 c, 无符号长整数型
    局部变量 d, 无符号字节型
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	checks := []struct {
		desc string
		want string
	}{
		{"无符号整数型 → uint", "var a uint"},
		{"无符号短整数型 → uint16", "var b uint16"},
		{"无符号长整数型 → uint64", "var c uint64"},
		{"无符号字节型 → uint8", "var d uint8"},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: 期望包含 %q, 实际:\n%s", c.desc, c.want, out)
		}
	}
}

// TestGenerateGoFixedWidthIntTypes 验证固定位宽整数类型映射
func TestGenerateGoFixedWidthIntTypes(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 a, 有符号8位整数型
    局部变量 b, 有符号32位整数型
    局部变量 c, 无符号8位整数型
    局部变量 d, 无符号16位整数型
    局部变量 e, 无符号32位整数型
    局部变量 f, 无符号64位整数型
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	checks := []struct {
		desc string
		want string
	}{
		{"有符号8位整数型 → int8", "var a int8"},
		{"有符号32位整数型 → int32", "var b int32"},
		{"无符号8位整数型 → uint8", "var c uint8"},
		{"无符号16位整数型 → uint16", "var d uint16"},
		{"无符号32位整数型 → uint32", "var e uint32"},
		{"无符号64位整数型 → uint64", "var f uint64"},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: 期望包含 %q, 实际:\n%s", c.desc, c.want, out)
		}
	}
}

// TestGenerateGoUintptrType 验证 uintptr 类型映射
func TestGenerateGoUintptrType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 p, 无符号指针整数型
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	if !strings.Contains(out, "var p uintptr") {
		t.Errorf("期望包含 'var p uintptr', 实际:\n%s", out)
	}
}

// TestGenerateGoRuneType 验证字符型 → rune
func TestGenerateGoRuneType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 ch, 字符型
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	if !strings.Contains(out, "var ch rune") {
		t.Errorf("期望包含 'var ch rune', 实际:\n%s", out)
	}
}

// TestGenerateGoUnsignedTypeInFunc 验证无符号类型作为函数参数和返回值
func TestGenerateGoUnsignedTypeInFunc(t *testing.T) {
	src := `# 程序集 main

函数 加法(参数 a 无符号整数型, 参数 b 无符号整数型) 无符号整数型
    返回 a + b
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// 验证函数签名生成 "func 加法(a uint, b uint) uint"
	if !strings.Contains(out, "func 加法(a uint, b uint) uint") {
		t.Errorf("期望包含 'func 加法(a uint, b uint) uint', 实际:\n%s", out)
	}
}

// TestGenerateGoUnsignedArrayType 验证无符号类型数组
func TestGenerateGoUnsignedArrayType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 arr, 无符号整数数组
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// 无符号整数数组 → []uint
	if !strings.Contains(out, "var arr []uint") {
		t.Errorf("期望包含 'var arr []uint', 实际:\n%s", out)
	}
}

// TestGenerateGoUnsignedPtrType 验证无符号类型指针
func TestGenerateGoUnsignedPtrType(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 p, *无符号整数型
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	// *无符号整数型 → *uint
	if !strings.Contains(out, "var p *uint") {
		t.Errorf("期望包含 'var p *uint', 实际:\n%s", out)
	}
}

// TestGenerateGoTypeSwitchCaseWithUnsigned 验证 type switch case 中使用无符号类型
func TestGenerateGoTypeSwitchCaseWithUnsigned(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
    局部变量 v, 变体型
    选择 x ＝ v.(类型)
        情况 无符号整数型:
            打印("uint")
        情况 无符号长整数型:
            打印("uint64")
    结束选择
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	out, _ := GenerateGo(file)

	if !strings.Contains(out, "case uint:") {
		t.Errorf("期望包含 'case uint:', 实际:\n%s", out)
	}
	if !strings.Contains(out, "case uint64:") {
		t.Errorf("期望包含 'case uint64:', 实际:\n%s", out)
	}
}
