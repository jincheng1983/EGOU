package transpiler

import (
	"strings"
	"testing"
)

// TestUserButton2EventHandler 验证用户实际项目的合并源码能正确转译出事件注册代码。
// 用户项目结构：
//   main.eg: 主函数() 调用 加载窗口("启动窗口")
//   启动窗口.eg: 函数 button2_被单击() 包含 打印 和 信息框
// 组件名 button2 是英文+数字，事件后缀"被单击"是中文，验证混合命名能被正确识别。
func TestUserButton2EventHandler(t *testing.T) {
	// 模拟 collectProjectSources 合并后的源码
	merged := `#@eg-file main.eg
# 程序集 main

函数 主函数()
加载窗口("启动窗口")
打印("窗口程序已启动")
消息循环()
结束函数

#@eg-file 启动窗口.eg

函数 button2_被单击()
// 事件处理
  打印("被点击了")
  信息框("被点击了")
结束函数
`
	out, err := Transpile(merged)
	if err != nil {
		t.Fatalf("转译失败: %v", err)
	}

	// 验证事件处理函数被保留
	if !strings.Contains(out, "func button2_被单击()") {
		t.Errorf("事件处理函数 button2_被单击 未出现在转译输出中\n输出:\n%s", out)
	}

	// 验证 RegisterEvent 被正确生成
	expectedReg := `runtimeUIService.RegisterEvent("button2", "被单击", button2_被单击)`
	if !strings.Contains(out, expectedReg) {
		t.Errorf("RegisterEvent 未正确生成\n期望包含: %s\n输出:\n%s", expectedReg, out)
	}

	// 验证 registerHandlersImpl 包含注册调用
	if !strings.Contains(out, "func registerHandlersImpl()") {
		t.Errorf("registerHandlersImpl 未生成\n输出:\n%s", out)
	}

	// 验证打印和信息框被转译为 runtimeUIService 调用
	if !strings.Contains(out, `runtimeUIService.Println("被点击了")`) {
		t.Errorf("打印 未被转译为 Println\n输出:\n%s", out)
	}
	if !strings.Contains(out, `runtimeUIService.MessageBox("被点击了")`) {
		t.Errorf("信息框 未被转译为 MessageBox\n输出:\n%s", out)
	}

	t.Logf("转译输出:\n%s", out)
}
