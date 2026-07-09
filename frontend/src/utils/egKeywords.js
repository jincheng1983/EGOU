// EGOU 关键字 / 类型 / 代码片段 / 支持库别名 —— 前端单一真源
//
// !!! 同步提醒 !!!
// 本文件必须与后端 internal/transpiler/lexer.go 的 keywords 表 + internal/transpiler/transpiler.go 的
// mapType 函数 + supportLibrary 表保持一致。修改任一处时同步另一处。
// 权威来源：docs/syntax_design.md

// ===== 关键字（与 transpiler.go 实际支持一致）=====
// 包含块结构结束关键字（结束函数/结束方法/结束类型/结束如果/结束循环/结束判断循环/结束选择）
// 这是易语言风格的核心特色，不是冗余
export const KEYWORDS = [
  // 声明类
  '程序集', '导入',
  '常量', '变量', '局部变量',
  '类型', '结构体', '接口', '结束类型',
  '枚举', '结束枚举', '序数',
  '函数', '结束函数', '方法', '结束方法', '主函数',
  '初始化',
  '标签',
  // 控制流
  '如果', '否则', '否则如果', '结束如果',
  '循环', '结束循环', '判断循环', '结束判断循环',
  '选择', '情况', '默认', '结束选择',
  '通道选择', '结束通道选择',
  '返回', '继续', '跳出', '抛出', '恢复', '跳转', '穿透',
  // 修饰符
  '范围', '映射',
  // 字面量
  '真', '假', '空',
  // 并发/通道（已实现）
  '延迟', '协程', '通道', '新建',
  // 逻辑运算符
  '且', '或', '非'
]

// ===== 类型关键字（与 transpiler.go mapType / lexer.go typeKeywords 一致）=====
export const TYPE_KEYWORDS = [
  // 有符号整数
  '整数型', '长整数型', '短整数型', '字节型',
  // 无符号整数（v54 新增）
  '无符号整数型', '无符号短整数型', '无符号长整数型', '无符号字节型',
  // 固定位宽整数（v54 新增）
  '有符号8位整数型', '有符号32位整数型',
  '无符号8位整数型', '无符号16位整数型', '无符号32位整数型', '无符号64位整数型',
  // 指针宽度（v54 新增）
  '无符号指针整数型',
  // Unicode 码点（v54 新增）
  '字符型',
  // 浮点
  '小数型', '双精度小数型',
  // 文本/布尔/任意/字节集
  '文本型', '逻辑型', '变体型', '字节集',
  '数组' // 复合类型后缀（如"整数数组"）
]

// ===== 运算符关键字（中文）=====
export const OPERATOR_KEYWORDS = [
  '加', '减', '乘', '除', '取余'
  // 注意：且/或/非 在 KEYWORDS 中
]

// ===== 块结构关键字配对表（用于代码折叠/流程线/跳转）=====
export const BLOCK_PAIRS = {
  '函数': '结束函数',
  '方法': '结束方法',
  '类型': '结束类型',
  '枚举': '结束枚举',
  '如果': '结束如果',
  '循环': '结束循环',
  '判断循环': '结束判断循环',
  '选择': '结束选择',
  '通道选择': '结束通道选择'
}

// 块开始关键字（用于流程线/折叠检测）
export const BLOCK_STARTS = Object.keys(BLOCK_PAIRS)

// 块结束关键字
export const BLOCK_ENDS = Object.values(BLOCK_PAIRS)

// 块中间关键字（else/else if/case/default，不配对但属于块内）
export const BLOCK_MIDS = ['否则', '否则如果', '情况', '默认']

// ===== 内置支持库命令中文别名（与 transpiler.go supportLibrary 一致）=====
// 用于编辑器补全提示
export const SUPPORT_ALIASES = {
  // 输入输出
  '打印': 'Println(args...)',
  '到文本': 'ToString(v) string',
  '到整数': 'ToInt(s) int',
  '到小数': 'ToFloat64(s) float64',
  '取长度': 'Len(v) int',
  // 对话框
  '信息框': 'MessageBox(text, title)',
  '确认框': 'QuestionBox(text, title) bool',
  '输入框': 'InputBox(prompt) string',
  '打开文件': 'OpenFileDialog() string',
  '保存文件': 'SaveFileDialog() string',
  // 文本处理
  '取文本长度': 'TextLen(s) int',
  '取文本左边': 'TextLeft(s, n) string',
  '取文本右边': 'TextRight(s, n) string',
  '寻找文本': 'FindText(s, sub, start) int',
  '替换文本': 'ReplaceAll(s, old, new) string',
  '分割文本': 'Split(s, sep) []string',
  // 窗口操作
  '创建窗口': 'NewWindow(title, w, h)',
  '加载窗口': 'LoadWindow(name)',
  '消息循环': 'MessageLoop()',
  '设置窗口位置': 'SetWindowPosition(x, y)',
  '设置窗口大小': 'SetWindowSize(w, h)',
  '设置窗口标题': 'SetWindowTitle(title)',
  '设置窗口透明度': 'SetWindowOpacity(alpha)',
  '设置窗口置顶': 'SetAlwaysOnTop(onTop)',
  '窗口居中': 'CenterWindow()',
  '最小化窗口': 'MinimizeWindow()',
  '最大化窗口': 'MaximizeWindow()',
  '恢复窗口': 'RestoreWindow()',
  '全屏窗口': 'FullScreenWindow()',
  '关闭窗口': 'CloseWindow()',
  '隐藏窗口': 'HideWindow()',
  '显示窗口': 'ShowWindow()',
  '取屏幕宽度': 'ScreenWidth() int',
  '取屏幕高度': 'ScreenHeight() int',
  // 组件创建
  '创建按钮': 'CreateButton(text)',
  '创建编辑框': 'CreateEdit(placeholder)',
  '创建多行编辑框': 'CreateTextarea(placeholder)',
  '创建标签': 'CreateLabel(text)',
  '创建复选框': 'CreateCheckbox(text)',
  '创建单选框': 'CreateRadio(text)',
  '创建列表框': 'CreateListbox(items)',
  '创建组合框': 'CreateCombobox(items)',
  '创建开关': 'CreateSwitch(checked)',
  '创建滑动条': 'CreateSlider(min, max)',
  '创建进度条': 'CreateProgress(value)',
  '创建图片': 'CreateImage(src)',
  '创建标签页': 'CreateTabs()',
  '创建卡片': 'CreateCard()',
  '创建分割线': 'CreateDivider()',
  // 网络
  'HTTP GET': 'HttpGet(url) string',
  'HTTP POST': 'HttpPost(url, data) string',
  // 系统
  '延时': 'Sleep(ms)',
  '取剪贴板文本': 'ClipboardGetText() string',
  '置剪贴板文本': 'ClipboardSetText(text)',
  // 资源
  '载入资源': 'LoadAsset(name)',
  '读资源文本': 'ReadAssetText(name) string',
  '列举资源': 'ListAssets() []string',
  '列举窗口': 'ListWindows() []string',
  '资源是否存在': 'HasAsset(name) bool'
}

// ===== 代码片段（易语言风格块结构，与 transpiler.go 语法一致）=====
// 占位符语法 ${1:默认值} / ${2} / $0 最终光标位置
// 注意：源码不写 {}，块结构靠"结束xxx"关键字闭合，转译器自动添加 Go 的 {}
export const SNIPPETS = {
  // 声明类
  '程序集': '# 程序集 ${1:main}\n',
  '导入': '导入 (\n\t"${1:包路径}"\n)\n',
  '常量': '常量 ${1:名字} ＝ ${2:值}\n',
  '变量': '变量 (\n\t${1:名字} ${2:类型}\n)\n',
  '局部变量': '局部变量 ${1:名字}, ${2:类型}\n',
  '类型': '类型 ${1:类型名} 结构体\n\t${2:字段}, ${3:类型}\n结束类型\n',
  // 函数/方法
  '函数': '函数 ${1:函数名}(${2:参数})\n\t${3}\n结束函数\n',
  '方法': '方法 (${1:接收者} ${2:类型}) ${3:方法名}(${4:参数})\n\t${5}\n结束方法\n',
  '主函数': '函数 主函数()\n\t${1}\n结束函数\n',
  // 控制流
  '如果': '如果 (${1:条件})\n\t${2}\n否则\n\t${3}\n结束如果\n',
  '否则如果': '否则如果 (${1:条件})\n\t${2}\n',
  '循环': '循环 (${1:i ＝ 0}; ${2:i < 10}; ${3:i＋＋})\n\t${4}\n结束循环\n',
  '判断循环': '判断循环 (${1:条件})\n\t${2}\n结束判断循环\n',
  '选择': '选择 (${1:表达式})\n\t情况 ${2:值1}:\n\t\t${3}\n\t默认:\n\t\t${4}\n结束选择\n',
  // 并发（对应 Go defer/go/chan）
  '延迟': '延迟 ${1:函数调用()}\n',
  '协程': '协程 ${1:函数调用()}\n',
  // 跳转/标签/穿透（v52/v53 新增）
  '跳转': '跳转 ${1:标签名}\n',
  '标签': '标签 ${1:标签名}\n',
  '穿透': '穿透\n',
  // 通道选择（对应 Go select）
  '通道选择': '通道选择\n\t情况 ${1:chanOp}:\n\t\t${2}\n结束通道选择\n',
  // 事件处理函数（自动注册到 registerHandlersImpl）
  '事件_被单击': '函数 ${1:组件名}_被单击()\n\t${2}\n结束函数\n',
  '事件_内容被改变': '函数 ${1:组件名}_内容被改变()\n\t${2}\n结束函数\n',
  '事件_状态被改变': '函数 ${1:组件名}_状态被改变()\n\t${2}\n结束函数\n',
  '事件_选项被改变': '函数 ${1:组件名}_选项被改变()\n\t${2}\n结束函数\n',
  '事件_值被改变': '函数 ${1:组件名}_值被改变()\n\t${2}\n结束函数\n',
  '事件_获得焦点': '函数 ${1:组件名}_获得焦点()\n\t${2}\n结束函数\n',
  '事件_失去焦点': '函数 ${1:组件名}_失去焦点()\n\t${2}\n结束函数\n'
}

// ===== 工具函数 =====

// 判断字符串是否是关键字
export function isKeyword(s) {
  return KEYWORDS.includes(s)
}

// 判断字符串是否是类型关键字
export function isTypeKeyword(s) {
  return TYPE_KEYWORDS.includes(s)
}

// 判断字符串是否是块开始关键字
export function isBlockStart(s) {
  return BLOCK_STARTS.includes(s)
}

// 判断字符串是否是块结束关键字
export function isBlockEnd(s) {
  return BLOCK_ENDS.includes(s)
}

// 获取块结束关键字（给定开始关键字）
export function getBlockEnd(start) {
  return BLOCK_PAIRS[start] || ''
}

// 判断字符串是否是支持库命令
export function isSupportAlias(s) {
  return Object.prototype.hasOwnProperty.call(SUPPORT_ALIASES, s)
}
