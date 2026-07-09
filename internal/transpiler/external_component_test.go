package transpiler

import (
	"strings"
	"testing"
)

// TestExternalComponentCreate 验证外置组件创建命令的转译。
// "创建日期选择器(名称, 文本, x, y, w, h)" 应被替换为
// "runtimeUIService.CreateComponent("datepicker", 名称, 文本, x, y, w, h)"。
func TestExternalComponentCreate(t *testing.T) {
	defer ClearExternalCreates()
	defer ClearExternalEventSuffixes()
	RegisterExternalCreate("创建日期选择器", "datepicker")

	src := `# 程序集 main

函数 主函数()
	创建日期选择器("dp1", "请选择日期", 10, 10, 140, 28)
	消息循环()
结束函数
`
	out, err := Transpile(src)
	if err != nil {
		t.Fatalf("转译失败: %v", err)
	}
	if !strings.Contains(out, `runtimeUIService.CreateComponent("datepicker",`) {
		t.Errorf("外置组件创建命令未正确转译\n输出:\n%s", out)
	}
}

// TestExternalComponentEventSuffix 验证外置组件自定义事件后缀的自动注册。
// "树形框1_节点被点击" 应被识别为事件处理函数，生成 RegisterEvent 调用。
func TestExternalComponentEventSuffix(t *testing.T) {
	defer ClearExternalCreates()
	defer ClearExternalEventSuffixes()
	RegisterExternalEventSuffix("节点被点击")

	src := `# 程序集 main

函数 主函数()
	消息循环()
结束函数

函数 树形框1_节点被点击()
	打印("节点被点击")
结束函数
`
	out, err := Transpile(src)
	if err != nil {
		t.Fatalf("转译失败: %v", err)
	}
	if !strings.Contains(out, `RegisterEvent("树形框1", "节点被点击"`) {
		t.Errorf("外置组件事件后缀未自动注册\n输出:\n%s", out)
	}
}
