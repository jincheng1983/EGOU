package app

import (
	"strings"
	"testing"
)

// TestStripEntryDeclarations_ImportBlockNotEatEventHandlers 验证修复的 bug：
// 旧实现在遇到 `导入 (` 后，由于 `if skipDepth == -1` 检查被错误地放在
// `if skipDepth == 0` 块内部，导致导入块之后的所有行（包括事件处理函数）
// 都被静默剥离。修复后导入块应正确终止于 `)`，后续事件处理函数应保留。
func TestStripEntryDeclarations_ImportBlockNotEatEventHandlers(t *testing.T) {
	src := `# 程序集 窗口1
导入 (
	"fmt"
)

函数 按钮1_被单击()
	打印("被点击了")
结束函数

函数 辅助函数()
	返回
结束函数
`
	got := stripEntryDeclarations(src)

	// 事件处理函数必须保留
	if !strings.Contains(got, "函数 按钮1_被单击()") {
		t.Errorf("事件处理函数被错误剥离！\n输出:\n%s", got)
	}
	if !strings.Contains(got, "打印(\"被点击了\")") {
		t.Errorf("事件处理函数体被错误剥离！\n输出:\n%s", got)
	}
	// 辅助函数也必须保留
	if !strings.Contains(got, "函数 辅助函数()") {
		t.Errorf("辅助函数被错误剥离！\n输出:\n%s", got)
	}
	// 导入块应被剥离
	if strings.Contains(got, "导入 (") {
		t.Errorf("导入块未被剥离！\n输出:\n%s", got)
	}
	if strings.Contains(got, "\"fmt\"") {
		t.Errorf("导入内容未被剥离！\n输出:\n%s", got)
	}
	// 程序集头应被剥离
	if strings.Contains(got, "程序集") {
		t.Errorf("程序集头未被剥离！\n输出:\n%s", got)
	}
}

// TestStripEntryDeclarations_MainFuncStripped 验证主函数段被正确剥离，
// 但主函数段之后的事件处理函数应保留。
func TestStripEntryDeclarations_MainFuncStripped(t *testing.T) {
	src := `# 程序集 窗口1
函数 主函数()
	载入窗口("窗口1")
	进入消息循环()
结束函数

函数 按钮1_被单击()
	打印("被点击了")
结束函数
`
	got := stripEntryDeclarations(src)

	// 主函数段应被剥离
	if strings.Contains(got, "函数 主函数()") {
		t.Errorf("主函数段未被剥离！\n输出:\n%s", got)
	}
	if strings.Contains(got, "载入窗口") {
		t.Errorf("主函数体未被剥离！\n输出:\n%s", got)
	}
	// 主函数段之后的事件处理函数应保留
	if !strings.Contains(got, "函数 按钮1_被单击()") {
		t.Errorf("主函数后的事件处理函数被错误剥离！\n输出:\n%s", got)
	}
}

// TestStripEntryDeclarations_ImportThenMain 验证导入块和主函数段同时存在时，
// 两者都被正确剥离，且后续事件处理函数保留。
func TestStripEntryDeclarations_ImportThenMain(t *testing.T) {
	src := `# 程序集 窗口1
导入 (
	"fmt"
	"strings"
)

函数 主函数()
	载入窗口("窗口1")
结束函数

函数 按钮1_被单击()
	打印("被点击了")
结束函数

函数 按钮2_被单击()
	信息框("hello")
结束函数
`
	got := stripEntryDeclarations(src)

	// 导入块和主函数段都应被剥离
	if strings.Contains(got, "导入 (") || strings.Contains(got, "函数 主函数()") {
		t.Errorf("导入块或主函数段未被剥离！\n输出:\n%s", got)
	}
	// 两个事件处理函数都应保留
	if !strings.Contains(got, "函数 按钮1_被单击()") {
		t.Errorf("按钮1事件处理函数被错误剥离！\n输出:\n%s", got)
	}
	if !strings.Contains(got, "函数 按钮2_被单击()") {
		t.Errorf("按钮2事件处理函数被错误剥离！\n输出:\n%s", got)
	}
	if !strings.Contains(got, "打印(\"被点击了\")") {
		t.Errorf("按钮1事件处理函数体被错误剥离！\n输出:\n%s", got)
	}
	if !strings.Contains(got, "信息框(\"hello\")") {
		t.Errorf("按钮2事件处理函数体被错误剥离！\n输出:\n%s", got)
	}
}
