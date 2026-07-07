// Package valuecell 是 ycIDE 风格 YC_MDATA_INF 的 Go 端最小实现。
//
// 目的：让 transpiler 编译出来的 Go 代码可以统一用
//
//	v := valuecell.New(...)
//	runtime.Invoke("krnln.Add", a, b, v)
//
// 这样的形式调用支持库命令，把"易语类型 ↔ Go 类型"的转换集中到本包里。
//
// 当前阶段只保留骨架与类型定义，具体 marshal 逻辑等 commands.json 命令
// 陆续迁移到 internal/krnln、internal/egou 时再补全。
package valuecell

// Type 是易语类型在 Go 端的枚举，对应前端 type 字段的取值。
type Type string

const (
	TByte       Type = "字节型"
	TShort      Type = "短整数型"
	TInt        Type = "整数型"
	TLong       Type = "长整数型"
	TFloat      Type = "小数型"
	TDouble     Type = "双精度小数型"
	TBool       Type = "逻辑型"
	TText       Type = "文本型"
	TBytes      Type = "字节集"
	TDateTime   Type = "日期时间型"
	TGeneric    Type = "通用型"
	TWindow     Type = "窗口型"
	TButton     Type = "按钮型"
	TEdit       Type = "编辑框型"
	TLabel      Type = "标签型"
	TCheckbox   Type = "复选框型"
	TRadio      Type = "单选框型"
	TListbox    Type = "列表框型"
	TCombobox   Type = "组合框型"
	TSwitch     Type = "开关型"
	TSlider     Type = "滑动条型"
	TProgress   Type = "进度条型"
	TImage      Type = "图片型"
	TTabs       Type = "标签页型"
	TCard       Type = "卡片型"
	TDivider    Type = "分割线型"
	TVoid          Type = "无返回值"
	TFunctionPtr   Type = "函数指针"
	TMenu          Type = "菜单"
)

// Cell 是 YC_MDATA_INF 在 Go 端的最小对应：类型 + 值。
type Cell struct {
	Type  Type
	Value interface{}
}

// New 按易语类型构造一个 Cell。空值/未实现类型会原样落进 Value。
func New(t Type, v interface{}) *Cell {
	return &Cell{Type: t, Value: v}
}

// RiskLevel 按 krnln-command-mapping.md v1 风险分级自动推导 high/medium/low。
// 命中 high 集合直接归 high；命中 medium 归 medium；其它一律 low。
func RiskLevel(t Type) string {
	switch t {
	case TGeneric, TWindow, TButton, TEdit, TLabel, TCheckbox, TRadio,
		TListbox, TCombobox, TSwitch, TSlider, TProgress, TImage,
		TTabs, TCard, TDivider, TFunctionPtr, TMenu, TBytes:
		return "high"
	case TText, TDateTime:
		return "medium"
	default:
		return "low"
	}
}
