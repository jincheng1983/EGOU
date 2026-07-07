package transpiler

import (
	"strings"
	"testing"
)

// TestStringxSourceTranspile 验证 stringx 扩展包源码能正确转译为合法 Go 代码。
// 这是回归测试：之前 stringx 用了不合规的语法（带括号命名返回、Python 风格遍历、
// 不存在的合并文本命令、易语言计次循环、# 注释、未声明变量），导致任何项目编译时
// 合并 stringx 后全部失败。同时回归了 parseRangeStmt 不接受中文变量名的 bug。
func TestStringxSourceTranspile(t *testing.T) {
	src := `# 程序集 stringx
// 示例扩展包：字符串扩展函数。

// 反转文本：把文本字符顺序反转。
函数 反转文本(参数 原始文本 文本型) 文本型
    局部变量 结果, 文本型
    结果 = ""
    循环 字符 ＝ 范围 原始文本
        结果 = 文本型(字符) + 结果
    结束循环
    返回 结果
结束函数

// 重复文本：把文本重复 N 次。
函数 重复文本(参数 原始文本 文本型, 参数 次数 整数型) 文本型
    局部变量 结果, 文本型
    结果 = ""
    循环 i := 0; i < 次数; i++
        结果 = 结果 + 原始文本
    结束循环
    返回 结果
结束函数
`
	out, err := Transpile(src)
	if err != nil {
		t.Fatalf("Transpile stringx 失败: %v", err)
	}
	if !strings.Contains(out, "func 反转文本(原始文本 string) string") {
		t.Errorf("反转文本签名生成错误，实际:\n%s", out)
	}
	if !strings.Contains(out, "func 重复文本(原始文本 string, 次数 int) string") {
		t.Errorf("重复文本签名生成错误，实际:\n%s", out)
	}
	if !strings.Contains(out, "for _, 字符 := range 原始文本") {
		t.Errorf("range 循环生成错误，实际:\n%s", out)
	}
	if !strings.Contains(out, "for i := 0; i < 次数; i++") {
		t.Errorf("三段式 for 循环生成错误，实际:\n%s", out)
	}
	if !strings.Contains(out, "string(字符)") {
		t.Errorf("文本型(字符) 类型转换生成错误，实际:\n%s", out)
	}
	if !strings.Contains(out, "var 结果 string") {
		t.Errorf("局部变量声明生成错误，实际:\n%s", out)
	}
}
