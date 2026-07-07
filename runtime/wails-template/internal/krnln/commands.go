// Package krnln 是 EGOU 的"系统核心支持库"对应 Go 实现层。
//
// 命名/映射规则（抄 ycIDE ycmd v1）：
//   - commandId 全局唯一：<lib>.<Name>，例如 "krnln.If"
//   - Go 符号：以 commandId 后缀 + 命名空间点号拆解，本包内的命令统一挂
//     在 KrLn 包，函数名 = 后缀驼峰。例如 "krnln.If" -> KrLnIf。
//   - 参数 / 返回值通过 ValueCell 统一 marshal，调用方只关心具体类型。
//
// 后续会把 lib/krnln/commands.json 里的命令逐步迁到这里实现。
package krnln

// KrLnIf 对应 "krnln.If"（如果真）
// 实际逻辑由 transpiler 把 .eg 代码翻译成 Go 直接调用本函数，
// 因此本文件目前只保留签名与占位，方便后续把流程控制命令归位。
func KrLnIf(condition bool) {
	_ = condition
}

// KrLnIfe 对应 "krnln.Ife"（如果...否则...结束如果）
func KrLnIfe(condition bool) (entered bool) {
	return condition
}

// KrLnWhile 对应 "krnln.While"（判断循环首）
func KrLnWhile(condition func() bool) {
	for condition() {
		// block 由调用方实现
	}
}
