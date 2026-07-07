// Package transpiler — 事件处理函数注册调试测试
package transpiler

import (
	"strings"
	"testing"
)

// TestEventHandlerRegistrationAST 验证 AST 通道正确生成事件处理函数注册
func TestEventHandlerRegistrationAST(t *testing.T) {
	src := `# 程序集 main

函数 按钮1_被单击()
	打印("被点击了")
结束函数

函数 主函数()
	创建窗口("主窗口", 400, 300)
	消息循环()
结束函数
`
	// 用 tryTranspileByAST（包含后处理：translateSupportCalls + injectSupportDefsAndImports + gofmt）
	out, ok := tryTranspileByAST(src)
	if !ok {
		t.Fatal("tryTranspileByAST 回退到正则通道（AST 通道不可用）")
	}
	t.Logf("AST 通道完整输出:\n%s", out)

	// 检查关键点
	checks := []struct {
		desc string
		want string
	}{
		{"事件处理函数声明", "func 按钮1_被单击()"},
		{"registerHandlersImpl 函数", "func registerHandlersImpl()"},
		{"RegisterEvent 调用", `runtimeUIService.RegisterEvent("按钮1", "被单击", 按钮1_被单击)`},
		{"mainImpl 函数", "func mainImpl()"},
		{"打印调用", `runtimeUIService.Println("被点击了")`},
		{"创建窗口调用", `runtimeUIService.NewWindow("主窗口", 400, 300)`},
		{"消息循环调用", `runtimeUIService.MessageLoop()`},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("[AST] %s: 期望包含 %q, 实际不含", c.desc, c.want)
		}
	}
}

// TestEventHandlerRegistrationRegex 验证正则通道正确生成事件处理函数注册
func TestEventHandlerRegistrationRegex(t *testing.T) {
	src := `# 程序集 main

函数 按钮1_被单击()
	打印("被点击了")
结束函数

函数 主函数()
	创建窗口("主窗口", 400, 300)
	消息循环()
结束函数
`
	out, err := Transpile(src)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}
	t.Logf("Transpile 输出:\n%s", out)

	// 检查关键点（不区分 AST/正则通道，只看最终输出）
	checks := []struct {
		desc string
		want string
	}{
		{"事件处理函数声明", "func 按钮1_被单击()"},
		{"registerHandlersImpl 函数", "func registerHandlersImpl()"},
		{"RegisterEvent 调用", `runtimeUIService.RegisterEvent("按钮1", "被单击", 按钮1_被单击)`},
		{"mainImpl 函数", "func mainImpl()"},
		{"打印调用", `runtimeUIService.Println("被点击了")`},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("[Transpile] %s: 期望包含 %q, 实际不含", c.desc, c.want)
		}
	}
}
