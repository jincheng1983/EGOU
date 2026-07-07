// Package egou 是 EGOU 桌面应用运行时支持库对应 Go 实现层。
//
// 命名规则与 krnln 包一致：commandId 后缀驼峰，前面加 Nx 前缀。
// 例如 "egou.MessageBox" -> NxMessageBox。
package egou

// NxMessageBox 对应 "egou.MessageBox"（信息框）
// 真实实现迁移到 uiservice.go 的 MessageBox 服务时由这里 re-export。
func NxMessageBox(content, title string) bool {
	_ = content
	_ = title
	return true
}

// NxToString 对应 "egou.ToString"（到文本）
// 走 fmt.Sprintf("%v", v)，与易语行为基本一致。
func NxToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return formatDefault(t)
	}
}
