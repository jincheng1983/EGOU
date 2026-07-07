package egou

import "fmt"

// formatDefault 把任意值格式化成易语"到文本"风格的字符串。
// 与 fmt.Sprintf("%v", v) 的区别在于 nil/[]byte 等特殊类型的展示。
func formatDefault(v interface{}) string {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}
