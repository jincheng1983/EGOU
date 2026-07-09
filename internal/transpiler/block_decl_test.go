// Package transpiler — 多行 常量(...)/变量(...) 块解析测试
package transpiler

import (
	"strings"
	"testing"
)

// TestConstBlockDecl 验证多常量块（常量 ( ... )）正确解析和生成 const ( ... )
func TestConstBlockDecl(t *testing.T) {
	src := `# 程序集 main

常量 (
	Pi ＝ 3.14
	E ＝ 2.71
)
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	// 验证生成了 ConstBlockDecl（而非被跳过）
	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo failed: %v", err)
	}

	// 应包含 const ( 块
	if !strings.Contains(out, "const (") {
		t.Errorf("期望包含 'const (', 实际:\n%s", out)
	}
	// 两项常量都应生成（gofmt 会对齐等号，故用值匹配而非严格 "名字 = 值"）
	if !strings.Contains(out, "3.14") {
		t.Errorf("期望包含 '3.14', 实际:\n%s", out)
	}
	if !strings.Contains(out, "2.71") {
		t.Errorf("期望包含 '2.71', 实际:\n%s", out)
	}
	// 验证常量名也在输出中
	if !strings.Contains(out, "Pi") {
		t.Errorf("期望包含 'Pi', 实际:\n%s", out)
	}
	if !strings.Contains(out, "E") {
		t.Errorf("期望包含 'E', 实际:\n%s", out)
	}
}

// TestVarBlockDecl 验证多变量块（变量 ( ... )）正确解析和生成 var ( ... )
func TestVarBlockDecl(t *testing.T) {
	src := `# 程序集 main

变量 (
	x 整数型
	y ＝ 10
	z, 文本型
)
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

	// 应包含 var ( 块
	if !strings.Contains(out, "var (") {
		t.Errorf("期望包含 'var (', 实际:\n%s", out)
	}
	// 三种形式都应正确生成
	if !strings.Contains(out, "x int") {
		t.Errorf("期望包含 'x int'（名字 类型形式）, 实际:\n%s", out)
	}
	if !strings.Contains(out, "y = 10") {
		t.Errorf("期望包含 'y = 10'（名字 ＝ 值形式）, 实际:\n%s", out)
	}
	if !strings.Contains(out, "z string") {
		t.Errorf("期望包含 'z string'（名字, 类型形式）, 实际:\n%s", out)
	}
}

// TestSingleConstVarStillWorks 验证单常量/单变量声明未受影响（回归测试）
func TestSingleConstVarStillWorks(t *testing.T) {
	src := `# 程序集 main

常量 Pi ＝ 3.14
变量 x 整数型
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

	// 单常量应生成 "const Pi = 3.14"（非块形式）
	if !strings.Contains(out, "const Pi = 3.14") {
		t.Errorf("期望包含 'const Pi = 3.14', 实际:\n%s", out)
	}
	// 单变量应生成 "var x int"
	if !strings.Contains(out, "var x int") {
		t.Errorf("期望包含 'var x int', 实际:\n%s", out)
	}
}
