// Package transpiler — 端到端转译测试
//
// 验证 延迟/协程/通道/且/或/非 等新关键字能正确转译为 Go 代码
package transpiler

import (
	"strings"
	"testing"
)

// TestTranspileDeferGoChan 验证延迟/协程/通道/且/或/非的转译
func TestTranspileDeferGoChan(t *testing.T) {
	src := `# 程序集 main
导入 (
	"fmt"
	"time"
)

函数 主函数()
	通道 整数型 ch ＝ 新建 通道 整数型
	协程 func() {
		ch <- 1
	}()
	延迟 func() {
		fmt.打印("cleanup")
	}()
	如果 a > 0 且 b > 0
		fmt.打印("both positive")
	结束如果
	如果 a < 0 或 b < 0
		fmt.打印("has negative")
	结束如果
结束函数
`

	out, err := Transpile(src)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	checks := []struct {
		desc string
		want string
	}{
		{"延迟转defer", "defer"},
		{"协程转go", "go"},
		{"且转&&", "&&"},
		{"或转||", "||"},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: 期望包含 %q, 实际输出:\n%s", c.desc, c.want, out)
		}
	}
}

// TestTranspileElseIf 验证"否则如果"简写转译为 Go 的 "else if"
func TestTranspileElseIf(t *testing.T) {
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

	out, err := Transpile(src)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	if !strings.Contains(out, "else if") {
		t.Errorf("期望包含 'else if', 实际输出:\n%s", out)
	}
	// 不应生成嵌套的 "} else {\n    if a < 0"
	// 注意：最后的 "} else {" 是正常的（"否则"分支），只检查 a<0 是否在 else if 行
	if strings.Contains(out, "} else {\n") && strings.Contains(out, "\tif a < 0") {
		t.Errorf("生成了嵌套 } else { if 而非 } else if, 实际:\n%s", out)
	}
}

// TestTranspileASTSupportCmds 验证 AST 通道完整转译含支持库命令的代码
// 覆盖 v65 新增功能：TranspileAST 后处理（支持库命令替换 + goDef 注入 + imports 补入）
func TestTranspileASTSupportCmds(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	打印("hello")
	到文本(123)
结束函数
`

	out, err := TranspileAST(src)
	if err != nil {
		t.Fatalf("TranspileAST failed: %v", err)
	}

	checks := []struct {
		desc string
		want string
	}{
		{"打印替换为Println", "Println("},
		{"到文本替换为ToString", "ToString("},
		{"ToString的goDef注入", "func ToString("},
		{"fmt import补入", "\"fmt\""},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: 期望包含 %q, 实际输出:\n%s", c.desc, c.want, out)
		}
	}

	// 不应保留中文别名（"打印(" 已替换为 "Println("）
	if strings.Contains(out, "打印(") {
		t.Errorf("不应保留中文别名 '打印(', 实际输出:\n%s", out)
	}
}

// TestTranspileASTChanVarDecl 验证 AST 通道变量声明完整转译
// 覆盖 v65 新增功能：parseChanVarDeclStmt（通道 整数型 ch ＝ 新建 通道 整数型）
func TestTranspileASTChanVarDecl(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	通道 整数型 ch ＝ 新建 通道 整数型
	ch <- 1
	值 ＝ <-ch
结束函数
`

	out, err := TranspileAST(src)
	if err != nil {
		t.Fatalf("TranspileAST failed: %v", err)
	}

	checks := []struct {
		desc string
		want string
	}{
		{"通道变量声明", "var ch chan int"},
		{"make初始化", "make(chan int)"},
		{"发送语句", "ch <- 1"},
		{"接收语句", "<-ch"},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: 期望包含 %q, 实际输出:\n%s", c.desc, c.want, out)
		}
	}
}

// TestTranspileTopLevelExecStmts 验证顶层可执行语句自动包装到 init() 函数。
// Go 不允许函数外有可执行语句，转译器自动用 init() 包装，语义等同脚本式顶层语句。
func TestTranspileTopLevelExecStmts(t *testing.T) {
	src := `# 程序集 main
导入 (
	"fmt"
)

打印("程序启动")
局部变量 x ＝ 10
fmt.Println(x)
函数 foo()
	返回
结束函数
打印("程序结束")
`
	out, err := Transpile(src)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}
	// 应生成 init() 函数包装顶层可执行语句
	if !strings.Contains(out, "func init() {") {
		t.Errorf("期望生成 init() 函数包装顶层可执行语句，实际输出:\n%s", out)
	}
	// 第一条打印应在第一个 init() 中
	if !strings.Contains(out, "Println(\"程序启动\")") {
		t.Errorf("期望包含 Println(\"程序启动\")，实际输出:\n%s", out)
	}
	// 第二条打印应在第二个 init() 中（中间被函数声明分隔）
	if !strings.Contains(out, "Println(\"程序结束\")") {
		t.Errorf("期望包含 Println(\"程序结束\")，实际输出:\n%s", out)
	}
	// 函数 foo 应正常生成，不在 init() 中
	if !strings.Contains(out, "func foo() {") {
		t.Errorf("期望包含 func foo() {，实际输出:\n%s", out)
	}
	// 不应报"函数外部不能有可执行语句"错误
	if strings.Contains(out, "函数外部不能有可执行语句") {
		t.Errorf("不应报顶层可执行语句错误，实际输出:\n%s", out)
	}
}
