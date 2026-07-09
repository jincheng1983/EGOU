package transpiler

import (
	"strings"
	"testing"
)

// TestMergedLibsPackageFirst 验证合并扩展包后转译输出中 package 声明在所有 func 之前。
// 回归场景：扩展包 source.eg 被 stripLibEntryDeclarations 剥离 # 程序集 后合并到主源码前面，
// 主源码的 # 程序集 main 生成 package main，但位置在扩展包函数之后，
// 导致 Go 编译报 "expected 'package', found 'func'"。
func TestMergedLibsPackageFirst(t *testing.T) {
	merged := `// ===== 扩展包自动合并 =====
#@eg-file global:libs/stringx/source.eg
// 示例扩展包

函数 反转文本(原始文本 文本型) 文本型
    局部变量 结果, 文本型
    结果 = ""
    循环 字符 ＝ 范围 原始文本
        结果 = 文本型(字符) + 结果
    结束循环
    返回 结果
结束函数

#@eg-file project:main.eg
# 程序集 main
导入 (
	"runtime"
)

函数 主函数() {
	加载窗口("启动窗口")
}
`
	out, err := Transpile(merged)
	if err != nil {
		t.Fatalf("Transpile 失败: %v", err)
	}
	if !strings.Contains(out, "package main") {
		t.Errorf("缺少 package main 声明")
	}
	pkgIdx := strings.Index(out, "package main")
	firstFuncIdx := strings.Index(out, "func ")
	if pkgIdx < 0 || firstFuncIdx < 0 {
		t.Fatalf("package main 或 func 缺失")
	}
	if pkgIdx > firstFuncIdx {
		t.Errorf("package main (位置 %d) 在第一个 func (位置 %d) 之后，Go 编译会失败", pkgIdx, firstFuncIdx)
	}
}
