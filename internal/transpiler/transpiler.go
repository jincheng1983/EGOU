// Package transpiler 将 .eg 中文化源码转译为标准 Go 源码。
// 当前为最小可行原型（MVP），采用行级状态机实现，后续会替换为基于 AST 的完整编译器前端。
package transpiler

import (
	"fmt"
	"go/format"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// 支持库命令中文别名 -> 英文键
type supportCmd struct {
	english       string
	goDef         string
	imports       []string
	runtimeMethod string // 若非空，调用 runtimeUIService.EnglishName 而非注入 goDef
}

// H3：函数内正则上移为包级变量，编译一次复用，避免每次 Transpile 重新编译。
var (
	// 函数签名：名字 ( 参数列表 ) 返回类型
	reFuncSignature = regexp.MustCompile(`^(\S+)\s*\((.*)\)\s*(\S*)$`)
	// 方法签名：(接收者) 名字(参数) 返回类型
	reMethodSignature = regexp.MustCompile(`^\(([^)]+)\)\s*(\S+)\s*\((.*)\)\s*(\S*)$`)
	// 数组字面量：整数数组{ -> []int{
	reArrLiteral = regexp.MustCompile(`(\S+?)数组\s*\{`)
	// 映射字面量：映射 文本型 整数型{ -> map[string]int{
	reMapLiteral = regexp.MustCompile(`映射\s+(\S+)\s+(\S+)\s*\{`)
	// 类型转换：文本型(x) -> string(x)；按长度降序避免短名误匹配长名前缀。
	// ^ 锚定确保只在 rest 开头匹配，避免 FindStringSubmatch 返回非开头匹配
	// 导致 i 跳过错误字符数（曾导致 "结果 = 文本型(字符)" 丢失前缀且重复输出 string(）。
	reTypeCast = regexp.MustCompile(`^(无符号指针整数型|无符号64位整数型|无符号32位整数型|无符号16位整数型|无符号8位整数型|有符号32位整数型|有符号8位整数型|无符号长整数型|无符号短整数型|无符号整数型|双精度小数型|无符号字节型|长整数型|短整数型|整数型|字节型|字符型|小数型|文本型|逻辑型|变体型|字节集)\s*\(`)
	// for 循环 init 赋值：i = 0 -> i := 0
	reForInitAssign = regexp.MustCompile(`^(\S+)\s*＝\s*(.+)$`)
	// 复合类型 map：map[文本型]整数型
	reCompositeMap = regexp.MustCompile(`map\[\s*(\S+?)\s*\]\s*(\S+)`)
	// 复合类型 数组：[]整数型
	reCompositeArr = regexp.MustCompile(`\[\s*(\S+?)\s*\]`)
	// M7：标识符提取正则，一次性扫描源码中所有标识符（中文/英文/下划线/数字），
	// 替代 detectSupportCommands 中 N 次 strings.Contains 全文扫描。
	reIdent = regexp.MustCompile(`[\p{L}_][\p{L}\p{N}_]*`)
)

// M7：英文键 → 中文别名 反查表，初始化一次复用。
// detectSupportCommands 和 replaceSupportCallsInLine 通过它把英文键映射回 supportLibrary 的 key。
var englishToAlias = func() map[string]string {
	m := make(map[string]string, len(supportLibrary))
	for alias, cmd := range supportLibrary {
		m[cmd.english] = alias
	}
	return m
}()

// 已知组件事件后缀，用于自动生成 registerHandlers 注册表。
var knownEventSuffixes = []string{
	"被单击", "内容被改变", "状态被改变", "选项被改变", "值被改变",
	"获得焦点", "失去焦点",
}

// knownEventSuffixesMap 是 knownEventSuffixes 的 set 形式，用于 O(1) 查找。
var knownEventSuffixesMap = func() map[string]bool {
	m := make(map[string]bool, len(knownEventSuffixes))
	for _, s := range knownEventSuffixes {
		m[s] = true
	}
	return m
}()

// externalCreateCmds 存储外置组件的动态创建命令映射。
// key 是中文别名（如 "创建日期选择器"），value 是组件类型标识（如 "datepicker"）。
// 转译时 "创建日期选择器(名称, 文本, x, y, w, h)" 会被替换为
// "runtimeUIService.CreateComponent("datepicker", 名称, 文本, x, y, w, h)"。
// 由 runner 在 Transpile 前根据 IDE components/ 目录下已安装组件包动态注册。
var (
	externalCreateCmds = map[string]string{}
	externalCreateMu   sync.RWMutex
)

// RegisterExternalCreate 注册一个外置组件的创建命令别名。
// alias 形如 "创建日期选择器"，componentType 形如 "datepicker"。
func RegisterExternalCreate(alias, componentType string) {
	externalCreateMu.Lock()
	externalCreateCmds[alias] = componentType
	externalCreateMu.Unlock()
}

// ClearExternalCreates 清空已注册的外置组件创建命令。
// 每次编译前调用，避免上一次注册残留。
func ClearExternalCreates() {
	externalCreateMu.Lock()
	externalCreateCmds = map[string]string{}
	externalCreateMu.Unlock()
}

func getExternalCreate(alias string) (string, bool) {
	externalCreateMu.RLock()
	t, ok := externalCreateCmds[alias]
	externalCreateMu.RUnlock()
	return t, ok
}

// externalEventSuffixes 存储外置组件的自定义事件后缀（如 "节点被点击"），
// 供 parseEventHandlerName 自动识别事件处理函数名（如 "树形框1_节点被点击"）。
var (
	externalEventSuffixes = map[string]bool{}
	externalEventMu       sync.RWMutex
)

// RegisterExternalEventSuffix 注册一个外置组件事件后缀，使自动事件注册能识别该事件。
func RegisterExternalEventSuffix(suffix string) {
	externalEventMu.Lock()
	externalEventSuffixes[suffix] = true
	externalEventMu.Unlock()
}

// ClearExternalEventSuffixes 清空已注册的外置组件事件后缀。
func ClearExternalEventSuffixes() {
	externalEventMu.Lock()
	externalEventSuffixes = map[string]bool{}
	externalEventMu.Unlock()
}

func isExternalEventSuffix(suffix string) bool {
	externalEventMu.RLock()
	ok := externalEventSuffixes[suffix]
	externalEventMu.RUnlock()
	return ok
}

var supportLibrary = map[string]supportCmd{
	"信息框":       {english: "MessageBox", runtimeMethod: "MessageBox"},
	"确认框":       {english: "QuestionBox", runtimeMethod: "QuestionBox"},
	"输入框":       {english: "InputBox", runtimeMethod: "InputBox"},
	"打开文件":      {english: "OpenFileDialog", runtimeMethod: "OpenFileDialog"},
	"保存文件":      {english: "SaveFileDialog", runtimeMethod: "SaveFileDialog"},
	"到文本":       {english: "ToString", goDef: supportToString, imports: []string{"fmt"}},
	"打印":        {english: "Println", runtimeMethod: "Println"},
	"取长度":       {english: "Len", goDef: supportLen, imports: nil},
	"创建窗口":      {english: "NewWindow", runtimeMethod: "NewWindow"},
	"加载窗口":      {english: "LoadWindow", runtimeMethod: "LoadWindow"},
	"消息循环":      {english: "MessageLoop", runtimeMethod: "MessageLoop"},
	"设置窗口位置":    {english: "SetWindowPosition", runtimeMethod: "SetWindowPosition"},
	"设置窗口大小":    {english: "SetWindowSize", runtimeMethod: "SetWindowSize"},
	"设置窗口标题":    {english: "SetWindowTitle", runtimeMethod: "SetWindowTitle"},
	"设置窗口透明度":   {english: "SetWindowOpacity", runtimeMethod: "SetWindowOpacity"},
	"设置窗口置顶":    {english: "SetAlwaysOnTop", runtimeMethod: "SetAlwaysOnTop"},
	"窗口居中":      {english: "CenterWindow", runtimeMethod: "CenterWindow"},
	"最小化窗口":     {english: "MinimizeWindow", runtimeMethod: "MinimizeWindow"},
	"最大化窗口":     {english: "MaximizeWindow", runtimeMethod: "MaximizeWindow"},
	"恢复窗口":      {english: "RestoreWindow", runtimeMethod: "RestoreWindow"},
	"全屏窗口":      {english: "FullScreenWindow", runtimeMethod: "FullScreenWindow"},
	"关闭窗口":      {english: "CloseWindow", runtimeMethod: "CloseWindow"},
	"隐藏窗口":      {english: "HideWindow", runtimeMethod: "HideWindow"},
	"显示窗口":      {english: "ShowWindow", runtimeMethod: "ShowWindow"},
	"取屏幕宽度":     {english: "ScreenWidth", runtimeMethod: "ScreenWidth"},
	"取屏幕高度":     {english: "ScreenHeight", runtimeMethod: "ScreenHeight"},
	"取剪贴板文本":    {english: "ClipboardGetText", runtimeMethod: "ClipboardGetText"},
	"置剪贴板文本":    {english: "ClipboardSetText", runtimeMethod: "ClipboardSetText"},
	"创建按钮":      {english: "CreateButton", runtimeMethod: "CreateButton"},
	"创建编辑框":     {english: "CreateEdit", runtimeMethod: "CreateEdit"},
	"创建标签":      {english: "CreateLabel", runtimeMethod: "CreateLabel"},
	"创建复选框":     {english: "CreateCheckbox", runtimeMethod: "CreateCheckbox"},
	"创建单选框":     {english: "CreateRadio", runtimeMethod: "CreateRadio"},
	"创建列表框":     {english: "CreateListbox", runtimeMethod: "CreateListbox"},
	"创建组合框":     {english: "CreateCombobox", runtimeMethod: "CreateCombobox"},
	"创建开关":      {english: "CreateSwitch", runtimeMethod: "CreateSwitch"},
	"创建滑动条":     {english: "CreateSlider", runtimeMethod: "CreateSlider"},
	"创建进度条":     {english: "CreateProgress", runtimeMethod: "CreateProgress"},
	"创建图片":      {english: "CreateImage", runtimeMethod: "CreateImage"},
	"创建多行编辑框":   {english: "CreateTextarea", runtimeMethod: "CreateTextarea"},
	"创建标签页":     {english: "CreateTabs", runtimeMethod: "CreateTabs"},
	"创建卡片":      {english: "CreateCard", runtimeMethod: "CreateCard"},
	"创建分割线":     {english: "CreateDivider", runtimeMethod: "CreateDivider"},
	"HTTP GET":  {english: "HttpGet", goDef: supportHttpGet, imports: []string{"io", "net/http"}},
	"HTTP POST": {english: "HttpPost", goDef: supportHttpPost, imports: []string{"io", "net/http", "strings"}},
	"取文本长度":     {english: "TextLen", goDef: supportTextLen, imports: nil},
	"取文本左边":     {english: "TextLeft", goDef: supportTextLeft, imports: nil},
	"取文本右边":     {english: "TextRight", goDef: supportTextRight, imports: nil},
	"寻找文本":      {english: "FindText", goDef: supportFindText, imports: []string{"strings"}},
	"替换文本":      {english: "ReplaceAll", goDef: supportReplaceAll, imports: []string{"strings"}},
	"分割文本":      {english: "Split", goDef: supportSplit, imports: []string{"strings"}},
	"到整数":       {english: "ToInt", goDef: supportToInt, imports: []string{"strconv"}},
	"到小数":       {english: "ToFloat64", goDef: supportToFloat64, imports: []string{"strconv"}},
	"延时":        {english: "Sleep", goDef: supportSleep, imports: []string{"time"}},
	// P1：assets 资源嵌入相关命令。函数定义在 runtime/wails-template/assetstore.go，
	// transpiler 只做别名替换，不注入 goDef（embeddedFiles 由 runner.writeEmbeddedAssets 生成）。
	"载入资源":  {english: "LoadAsset", goDef: "", imports: nil},
	"读资源文本": {english: "ReadAssetText", goDef: "", imports: nil},
	// P4：资源清单 API。返回嵌入资源/窗口的路径列表，供用户程序枚举可用资源。
	"列举资源":   {english: "ListAssets", goDef: "", imports: nil},
	"列举窗口":   {english: "ListWindows", goDef: "", imports: nil},
	"资源是否存在": {english: "HasAsset", goDef: "", imports: nil},
}

const supportToString = `func ToString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}`

const supportLen = `func Len(v interface{}) int {
	switch x := v.(type) {
	case string:
		return len(x)
	case []int:
		return len(x)
	case []string:
		return len(x)
	case map[string]int:
		return len(x)
	case map[string]string:
		return len(x)
	default:
		return 0
	}
}`

const supportHttpGet = `func HttpGet(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}`

const supportHttpPost = `func HttpPost(url string, data string) string {
	resp, err := http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}`

const supportTextLen = `func TextLen(s string) int {
	return len([]rune(s))
}`

const supportTextLeft = `func TextLeft(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}
	return string(runes[:n])
}`

const supportTextRight = `func TextRight(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}
	return string(runes[len(runes)-n:])
}`

const supportFindText = `func FindText(s, substr string, start int) int {
	if start < 1 {
		start = 1
	}
	runes := []rune(s)
	if start-1 >= len(runes) {
		return 0
	}
	idx := strings.Index(string(runes[start-1:]), substr)
	if idx < 0 {
		return 0
	}
	return start + idx
}`

const supportReplaceAll = `func ReplaceAll(s, old, newStr string) string {
	return strings.ReplaceAll(s, old, newStr)
}`

const supportSplit = `func Split(s, sep string) []string {
	return strings.Split(s, sep)
}`

const supportToInt = `func ToInt(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}`

const supportToFloat64 = `func ToFloat64(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}`

const supportSleep = `func Sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}`

// extraAliases 存储项目 .elib 提供的命令别名映射（中文别名 → 英文键）。
// 这些命令的函数定义已在 .elib 的 source.eg 中（由 runner.mergeProjectLibs 拼接），
// transpiler 只需做别名替换，不需要注入 goDef。
// H2：用 sync.RWMutex 保护，避免并发构建时 RegisterExtraAliases/ClearExtraAliases
// 与 replaceSupportCallsInLine 的读取触发 "concurrent map read and map write" fatal error。
var (
	extraAliases   = map[string]string{}
	extraAliasesMu sync.RWMutex
)

// RegisterExtraAliases 注册项目 .elib 提供的命令别名映射。
// aliases: key = 中文别名, value = 英文键（Go 函数名）。
func RegisterExtraAliases(aliases map[string]string) {
	extraAliasesMu.Lock()
	defer extraAliasesMu.Unlock()
	extraAliases = aliases
}

// ClearExtraAliases 清空已注册的项目 .elib 命令别名。
func ClearExtraAliases() {
	extraAliasesMu.Lock()
	defer extraAliasesMu.Unlock()
	extraAliases = map[string]string{}
}

// getExtraAlias 线程安全地查询单个别名，返回英文键和是否命中。
func getExtraAlias(alias string) (string, bool) {
	extraAliasesMu.RLock()
	defer extraAliasesMu.RUnlock()
	eng, ok := extraAliases[alias]
	return eng, ok
}

// tryTranspileByAST 尝试用 AST 通道转译源码
// 返回 (out, true) 表示成功可采用；返回 ("", false) 表示需回退正则通道
// 策略（v0.7.0 放宽）：
//   - file == nil（致命解析失败）→ 回退
//   - 错误数 > maxASTErrors（垃圾输入门禁）→ 回退
//   - GenerateGo 失败 → 回退
//   - gofmt 失败（生成的代码语法不正确）→ 回退
//   - 否则采用 AST 输出（即使有少量非致命解析错误，错误恢复后仍可能生成可用代码）
//
// 依赖 gofmt 作为最终质量门禁：语法正确性由 Go 官方格式化器背书
const maxASTErrors = 20

func tryTranspileByAST(source string) (string, bool) {
	file, errs := Parse(source)
	if file == nil {
		return "", false
	}
	if len(errs) > maxASTErrors {
		return "", false // 错误过多，疑似垃圾输入，回退正则
	}
	out, err := GenerateGo(file)
	if err != nil {
		return "", false
	}
	// 后处理：支持库命令替换 + goDef 注入 + imports 补入
	out = translateSupportCalls(out)
	out = injectSupportDefsAndImports(out)
	// 最终 gofmt
	if formatted, ferr := format.Source([]byte(out)); ferr == nil {
		out = string(formatted)
	} else {
		return "", false // gofmt 失败说明生成的代码有问题，回退
	}
	return out, true
}

// isTopLevelDeclaration 判断一行 .eg 源码是否是顶层声明语句。
// 顶层声明可以在 toplevel 状态下直接输出（对应 Go 的 package/import/const/var/func/type 声明）。
// 非顶层声明（函数调用、赋值、控制流、局部变量等）属于可执行语句，
// 转译器会自动包装到 init() 函数中（Go 不允许函数外有可执行语句）。
func isTopLevelDeclaration(line string) bool {
	return strings.HasPrefix(line, "# 程序集") ||
		strings.HasPrefix(line, "导入 (") ||
		strings.HasPrefix(line, "常量 (") ||
		strings.HasPrefix(line, "变量 (") ||
		strings.HasPrefix(line, "函数 ") ||
		strings.HasPrefix(line, "结束函数") ||
		(strings.HasPrefix(line, "类型 ") && strings.HasSuffix(line, " 结构体")) ||
		line == "结束类型" ||
		strings.HasPrefix(line, "方法 ") ||
		strings.HasPrefix(line, "结束方法") ||
		strings.HasPrefix(line, "@嵌入")
}

// Transpile 将 .eg 源码内容转译为 .go 源码内容。
// v65：AST 优先策略——先尝试 AST 通道，仅当 Parse 返回零错误且 GenerateGo 成功时采用 AST 输出；
// 否则回退到正则通道（稳定兜底）。
func Transpile(source string) (string, error) {
	source = normalizeSource(source)

	// AST 优先：尝试用 AST 通道转译，仅当零错误时采用
	if out, ok := tryTranspileByAST(source); ok {
		return out, nil
	}

	// 回退：正则通道
	usedCmds := detectSupportCommands(source)
	supportImports := collectSupportImports(usedCmds)
	source = translateSupportCalls(source)

	lines := strings.Split(source, "\n")
	var out []string
	state := "toplevel"
	embedIndent := ""
	embedPrevState := "" // @嵌入 之前的 state，@结束 时恢复，避免函数体内的嵌入块结束后回到 toplevel
	packageName := ""
	hasImportBlock := false
	userImports := make(map[string]bool)
	var eventHandlers []string
	var doWhileConds []string // 判断循环(do-while)的条件栈
	topLevelInInit := false   // 是否在自动生成的 init() 函数中（用于包装顶层可执行语句）

	// ===== //line 指令支持：跟踪每行源码对应的输出位置，用于在生成的 Go 代码中
	// 插入 //line "文件名":行号 指令，让 Go 编译器错误指向 .eg 源码行而非 usercode.go。 =====
	var outLenBefore []int // outLenBefore[i] = 处理第 i 行源码前 len(out)
	var srcInfos []srcLineInfo
	currentFile := ""  // 当前源码文件名（由 #@eg-file 标记设置）
	fileLineStart := 0 // 当前文件在合并源码中的起始索引

	for i, raw := range lines {
		outLenBefore = append(outLenBefore, len(out))
		lineNum := i + 1
		line := strings.TrimSpace(raw)

		// #@eg-file 标记：更新当前文件上下文，不产生输出
		if strings.HasPrefix(line, "#@eg-file ") {
			currentFile = strings.TrimSpace(strings.TrimPrefix(line, "#@eg-file "))
			fileLineStart = i + 1
			srcInfos = append(srcInfos, srcLineInfo{currentFile, 0})
			continue
		}

		// 计算文件内行号（用于 //line 指令和错误消息）
		fileLocalLine := lineNum
		if fileLineStart > 0 {
			fileLocalLine = i - fileLineStart + 1
		}
		fileForInfo := currentFile
		if fileForInfo == "" {
			fileForInfo = "源码.eg"
		}
		srcInfos = append(srcInfos, srcLineInfo{fileForInfo, fileLocalLine})

		// 跳过空行和纯注释（仅支持 // 注释，单引号 ' 为易语言风格已废弃）
		if line == "" || strings.HasPrefix(line, "//") {
			out = append(out, "")
			continue
		}

		switch state {
		case "embed":
			if strings.HasPrefix(line, "@结束") {
				// 恢复 @嵌入 之前的 state（可能是 toplevel 或 function）
				if embedPrevState != "" {
					state = embedPrevState
				} else {
					state = "toplevel"
				}
				embedIndent = ""
				embedPrevState = ""
				continue
			}
			out = append(out, embedIndent+raw)
			continue
		}

		// toplevel 状态下允许声明语句和可执行语句。
		// Go 不允许函数外有可执行语句，转译器自动把顶层可执行语句包装到 init() 函数中。
		// init() 会在 mainImpl 之前自动执行，语义等同脚本式顶层语句（如 Python）。
		// 遇到声明语句时若仍在 init() 中，先关闭 init() 再处理声明。
		// 注意：@嵌入 是块标记，toplevel 和 function 都能用，不触发 init() 开关。
		if state == "function" && topLevelInInit && isTopLevelDeclaration(line) && !strings.HasPrefix(line, "@嵌入") {
			out = append(out, "}")
			topLevelInInit = false
			state = "toplevel"
		}
		if state == "toplevel" && !isTopLevelDeclaration(line) {
			// 非声明语句 → 可执行语句，自动开启 init() 包装
			if !topLevelInInit {
				out = append(out, "")
				out = append(out, "// 顶层可执行语句自动包装到 init() 函数（Go 不允许函数外可执行语句）")
				out = append(out, "// init() 在 mainImpl 之前自动执行，语义等同脚本式顶层语句")
				out = append(out, "func init() {")
				topLevelInInit = true
			}
			state = "function"
		}

		switch {
		case strings.HasPrefix(line, "# 程序集"):
			name := strings.TrimSpace(strings.TrimPrefix(line, "# 程序集"))
			if name == "" {
				name = "main"
			}
			packageName = name
			out = append(out, fmt.Sprintf("package %s", name))

		case strings.HasPrefix(line, "导入 ("):
			out = append(out, "import (")
			hasImportBlock = true
			state = "import_block"

		case strings.HasPrefix(line, "常量 ("):
			out = append(out, "const (")
			state = "const_block"

		case strings.HasPrefix(line, "变量 ("):
			out = append(out, "var (")
			state = "var_block"

		case state == "import_block" && line == ")":
			for _, imp := range supportImports {
				if !userImports[imp] {
					out = append(out, "\t\""+imp+"\"")
				}
			}
			out = append(out, ")")
			state = "toplevel"

		case state == "const_block" && line == ")":
			out = append(out, ")")
			state = "toplevel"

		case state == "var_block" && line == ")":
			out = append(out, ")")
			state = "toplevel"

		case state == "import_block":
			out = append(out, "\t"+line)
			if imp := extractImportPath(line); imp != "" {
				userImports[imp] = true
			}

		case state == "const_block":
			out = append(out, "\t"+translateExpression(line))

		case state == "var_block":
			decl, err := translateVarDecl(line)
			if err != nil {
				return "", fmt.Errorf("第 %d 行: %w", fileLocalLine, err)
			}
			out = append(out, "\t"+decl)

		case strings.HasPrefix(line, "函数 "):
			sig, err := translateFunctionSignature(line)
			if err != nil {
				return "", fmt.Errorf("第 %d 行: %w", fileLocalLine, err)
			}
			if comp, evt, ok := parseEventHandlerName(extractFuncName(sig)); ok {
				eventHandlers = append(eventHandlers, fmt.Sprintf(`runtimeUIService.RegisterEvent("%s", "%s", %s)`, comp, evt, extractFuncName(sig)))
			}
			out = append(out, sig+" {")
			state = "function"

		case strings.HasPrefix(line, "结束函数"):
			out = append(out, "}")
			state = "toplevel"

		case strings.HasPrefix(line, "类型 ") && strings.HasSuffix(line, " 结构体"):
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "类型 "), " 结构体"))
			out = append(out, fmt.Sprintf("type %s struct {", name))
			state = "struct_block"

		case state == "struct_block":
			if line == "结束类型" {
				out = append(out, "}")
				state = "toplevel"
				continue
			}
			decl, err := translateVarDecl(line)
			if err != nil {
				return "", fmt.Errorf("第 %d 行: %w", fileLocalLine, err)
			}
			// translateVarDecl 返回 "var 名字 类型"，结构体字段只需要 "名字 类型"
			out = append(out, "\t"+strings.TrimPrefix(decl, "var "))

		case strings.HasPrefix(line, "方法 "):
			sig, err := translateMethodSignature(line)
			if err != nil {
				return "", fmt.Errorf("第 %d 行: %w", fileLocalLine, err)
			}
			out = append(out, sig+" {")
			state = "function"

		case strings.HasPrefix(line, "结束方法"):
			out = append(out, "}")
			state = "toplevel"

		case strings.HasPrefix(line, "局部变量 "):
			decl, err := translateVarDecl(strings.TrimPrefix(line, "局部变量 "))
			if err != nil {
				return "", fmt.Errorf("第 %d 行: %w", fileLocalLine, err)
			}
			out = append(out, "\t"+decl)

		case strings.HasPrefix(line, "常量 ") && !strings.HasPrefix(line, "常量 ("):
			decl, err := translateConstDecl(strings.TrimPrefix(line, "常量 "))
			if err != nil {
				return "", fmt.Errorf("第 %d 行: %w", fileLocalLine, err)
			}
			out = append(out, "\t"+decl)

		case strings.HasPrefix(line, "返回 "):
			expr := strings.TrimPrefix(line, "返回 ")
			out = append(out, "\treturn "+translateExpression(expr))

		case strings.HasPrefix(line, "返回"):
			out = append(out, "\treturn")

		case strings.HasPrefix(line, "如果 "):
			cond := strings.TrimPrefix(line, "如果 ")
			cond = strings.TrimSpace(cond)
			// 去掉行尾的“则”
			cond = strings.TrimSuffix(cond, " 则")
			cond = strings.TrimSuffix(cond, "则")
			cond = strings.TrimSpace(cond)
			// 去掉外层括号
			if strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") {
				cond = cond[1 : len(cond)-1]
			}
			out = append(out, "\tif "+translateExpression(cond)+" {")

		case strings.HasPrefix(line, "否则如果"):
			// 否则如果 cond → } else if cond {
			cond := strings.TrimPrefix(line, "否则如果")
			cond = strings.TrimSpace(cond)
			if strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") {
				cond = cond[1 : len(cond)-1]
			}
			out = append(out, "\t} else if "+translateExpression(cond)+" {")

		case strings.HasPrefix(line, "否则"):
			out = append(out, "\t} else {")

		case strings.HasPrefix(line, "结束如果"):
			out = append(out, "\t}")

		case strings.HasPrefix(line, "循环 "):
			clause := strings.TrimPrefix(line, "循环 ")
			// range 循环：循环 值 ＝ 范围 列表
			if strings.Contains(clause, " ＝ 范围 ") || strings.Contains(clause, " = 范围 ") {
				loop, err := translateRangeLoop(clause)
				if err != nil {
					return "", fmt.Errorf("第 %d 行: %w", fileLocalLine, err)
				}
				out = append(out, "\t"+loop)
				continue
			}
			// for init; cond; post
			parts := strings.Split(clause, ";")
			if len(parts) == 3 {
				init := translateForInit(strings.TrimSpace(parts[0]))
				cond := translateExpression(strings.TrimSpace(parts[1]))
				post := translateExpression(strings.TrimSpace(parts[2]))
				out = append(out, "\tfor "+init+"; "+cond+"; "+post+" {")
			} else {
				out = append(out, "\tfor "+translateExpression(clause)+" {")
			}

		case strings.HasPrefix(line, "结束循环"):
			out = append(out, "\t}")

		case strings.HasPrefix(line, "判断循环 "):
			// 判断循环 cond → for { ... if !(cond) { break } }
			// Go 没有 do-while，用 for { body; if !(cond) { break } } 模式
			// 条件保存到 doWhileConds 栈，结束判断循环 时取出
			cond := strings.TrimPrefix(line, "判断循环 ")
			cond = strings.TrimSpace(cond)
			if strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") {
				cond = cond[1 : len(cond)-1]
			}
			doWhileConds = append(doWhileConds, translateExpression(cond))
			out = append(out, "\tfor {")

		case strings.HasPrefix(line, "结束判断循环"):
			// 从栈取出条件，生成 if !(cond) { break } }
			if len(doWhileConds) > 0 {
				cond := doWhileConds[len(doWhileConds)-1]
				doWhileConds = doWhileConds[:len(doWhileConds)-1]
				out = append(out, "\t\tif !("+cond+") {")
				out = append(out, "\t\t\tbreak")
				out = append(out, "\t\t}")
			}
			out = append(out, "\t}")

		case strings.HasPrefix(line, "选择 "):
			// 选择 (expr) → switch expr {
			// 选择 expr  → switch expr {
			cond := strings.TrimPrefix(line, "选择 ")
			cond = strings.TrimSpace(cond)
			if strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") {
				cond = cond[1 : len(cond)-1]
			}
			out = append(out, "\tswitch "+translateExpression(cond)+" {")

		case strings.HasPrefix(line, "情况 "):
			// 情况 val  → case val:
			val := strings.TrimPrefix(line, "情况 ")
			val = strings.TrimSpace(val)
			out = append(out, "\tcase "+translateExpression(val)+":")

		case line == "默认":
			out = append(out, "\tdefault:")

		case strings.HasPrefix(line, "结束选择"):
			out = append(out, "\t}")

		// 延迟 xxx → defer xxx（对应 Go defer，用于资源释放/收尾逻辑）
		case strings.HasPrefix(line, "延迟 "):
			expr := strings.TrimPrefix(line, "延迟 ")
			out = append(out, "\tdefer "+translateExpression(expr))

		// 协程 xxx → go xxx（对应 Go go 关键字，启动 goroutine）
		case strings.HasPrefix(line, "协程 "):
			expr := strings.TrimPrefix(line, "协程 ")
			out = append(out, "\tgo "+translateExpression(expr))

		case strings.HasPrefix(line, "@嵌入"):
			embedPrevState = state
			state = "embed"
			embedIndent = extractIndent(raw)

		default:
			// function 状态下（含自动 init()），普通语句直接翻译运算符后输出
			out = append(out, "\t"+translateExpression(line))
		}
	}

	// 插入 //line 指令：让 Go 编译器错误指向 .eg 源码行而非 usercode.go 行号
	out = insertLineDirectives(out, outLenBefore, srcInfos)

	if packageName == "" {
		packageName = "main"
		out = append([]string{"package main"}, out...)
	} else {
		// 已有 package 声明（来自主源码的 # 程序集），但合并扩展包后可能不在文件顶部。
		// Go 语法要求 package 声明必须在最顶部，否则报 "expected 'package', found 'func'"。
		// 找到 package 行，移到最前面（保留该行的 //line 指令一起移动）。
		for i := 0; i < len(out); i++ {
			line := strings.TrimSpace(out[i])
			if strings.HasPrefix(line, "package ") {
				if i == 0 {
					break // 已在最顶部，无需移动
				}
				// 收集 package 行及其前面的 //line 指令
				start := i
				if i > 0 && strings.HasPrefix(strings.TrimSpace(out[i-1]), "//line ") {
					start = i - 1
				}
				block := append([]string{}, out[start:i+1]...)
				rest := append([]string{}, out[:start]...)
				rest = append(rest, out[i+1:]...)
				out = append(block, rest...)
				break
			}
		}
	}

	// 如果没有用户导入块但支持库需要导入，则在 package 后注入 import 块
	if !hasImportBlock && len(supportImports) > 0 {
		out = injectImports(out, supportImports)
	}

	// 生成事件处理函数注册表；Wails 运行时模板中的 main.go 会在执行主函数前调用它。
	out = append(out, "")
	out = append(out, "func registerHandlersImpl() {")
	for _, h := range eventHandlers {
		out = append(out, "\t"+h)
	}
	out = append(out, "}")

	// 如果用户代码没有定义"主函数"（即没有生成 mainImpl），补充空占位符，
	// 避免 main.go 中的 runtimeUIService.SetMainFuncs(registerHandlersImpl, mainImpl) 编译失败。
	hasMainImpl := false
	for _, l := range out {
		if strings.Contains(l, "func mainImpl(") {
			hasMainImpl = true
			break
		}
	}
	if !hasMainImpl {
		out = append(out, "")
		out = append(out, "// 空主函数占位符：用户 .eg 代码未定义\"主函数\"时由转译器自动补充")
		out = append(out, "func mainImpl() {}")
	}

	// 如果顶层可执行语句的 init() 函数未关闭（用户代码以可执行语句结尾），补上闭合括号。
	if topLevelInInit {
		out = append(out, "}")
		topLevelInInit = false
	}

	// 注入已使用的非运行时支持库函数定义
	if len(usedCmds) > 0 {
		out = append(out, "")
		out = append(out, "// 支持库函数")
		for alias := range usedCmds {
			cmd := supportLibrary[alias]
			if cmd.goDef == "" {
				continue
			}
			for _, defLine := range strings.Split(cmd.goDef, "\n") {
				out = append(out, defLine)
			}
		}
	}

	raw := strings.Join(out, "\n")
	// 正则通道也经过 gofmt 格式化，确保缩进标准（与 AST 通道一致）
	// gofmt 失败时返回原始输出（保留功能可用，避免完全无法编译）
	if formatted, ferr := format.Source([]byte(raw)); ferr == nil {
		return string(formatted), nil
	}
	return raw, nil
}

// srcLineInfo 记录每行源码所属的文件名和文件内行号，用于生成 //line 指令。
type srcLineInfo struct {
	file string
	line int
}

// insertLineDirectives 在生成的 Go 代码中插入 //line 指令，让 Go 编译器把错误定位
// 到原始 .eg 源码行。outLenBefore[i] 是处理第 i 行源码前 out 的长度，srcInfos[i]
// 是第 i 行源码所属的文件名和行号。只对产生非空输出的源码行插入指令，避免空行杂乱。
func insertLineDirectives(out []string, outLenBefore []int, srcInfos []srcLineInfo) []string {
	if len(srcInfos) == 0 || len(outLenBefore) == 0 {
		return out
	}
	var result []string
	for i := 0; i < len(srcInfos); i++ {
		startOut := outLenBefore[i]
		var endOut int
		if i+1 < len(outLenBefore) {
			endOut = outLenBefore[i+1]
		} else {
			endOut = len(out)
		}
		// 仅在该源码行产生了非空输出时插入 //line 指令
		if endOut > startOut && startOut < len(out) && out[startOut] != "" && srcInfos[i].line > 0 {
			// 不带引号格式：与 gen.go 保持一致，避免 dlv 把引号当作文件名的一部分
			result = append(result, fmt.Sprintf("//line %s:%d", srcInfos[i].file, srcInfos[i].line))
		}
		for j := startOut; j < endOut && j < len(out); j++ {
			result = append(result, out[j])
		}
	}
	return result
}

// injectImports 在 package 声明行之后插入 import 块。
func injectImports(out, supportImports []string) []string {
	for i, line := range out {
		if strings.HasPrefix(line, "package ") {
			imports := append([]string{"", "import ("}, make([]string, len(supportImports))...)
			for j, imp := range supportImports {
				imports[j+2] = "\t\"" + imp + "\""
			}
			imports = append(imports, ")")
			return append(out[:i+1], append(imports, out[i+1:]...)...)
		}
	}
	return out
}

// translateFunctionSignature 把 "函数 名字(参数 a 整数型, 参数 b 文本型) 整数型" 转成 Go 函数签名。
func translateFunctionSignature(line string) (string, error) {
	// 去掉前缀
	body := strings.TrimPrefix(line, "函数 ")

	// 匹配：名字 ( 参数列表 ) 返回类型
	m := reFuncSignature.FindStringSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("无法解析函数签名: %s", body)
	}

	name := m[1]
	paramsStr := strings.TrimSpace(m[2])
	retType := strings.TrimSpace(m[3])

	// 运行时入口函数：避免与 main.go 中的 main 函数名冲突，映射为 mainImpl。
	goName := name
	if name == "主函数" {
		goName = "mainImpl"
	}

	var goParams []string
	if paramsStr != "" {
		// 解析 "名字 类型" 格式
		parts := splitParams(paramsStr)
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			fields := strings.Fields(p)
			if len(fields) != 2 {
				return "", fmt.Errorf("参数格式错误（期望 名字 类型）: %s", p)
			}
			goParams = append(goParams, fmt.Sprintf("%s %s", fields[0], mapType(fields[1])))
		}
	}

	sig := fmt.Sprintf("func %s(%s)", goName, strings.Join(goParams, ", "))
	if retType != "" {
		sig += " " + mapType(retType)
	}
	return sig, nil
}

// extractFuncName 从函数签名中提取函数名。
func extractFuncName(sig string) string {
	// sig 形如 "func name(...)" 或 "func name(...) type"
	if !strings.HasPrefix(sig, "func ") {
		return ""
	}
	body := strings.TrimPrefix(sig, "func ")
	idx := strings.Index(body, "(")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(body[:idx])
}

// parseEventHandlerName 判断函数名是否为组件事件处理函数，返回组件名、事件名与是否匹配。
func parseEventHandlerName(name string) (string, string, bool) {
	for _, suffix := range knownEventSuffixes {
		sep := "_" + suffix
		if strings.HasSuffix(name, sep) {
			return strings.TrimSuffix(name, sep), suffix, true
		}
	}
	// 外置组件自定义事件后缀（如 "节点被点击"）
	externalEventMu.RLock()
	for suffix := range externalEventSuffixes {
		// 跳过已知后缀，避免重复
		if _, ok := knownEventSuffixesMap[suffix]; ok {
			continue
		}
		sep := "_" + suffix
		if strings.HasSuffix(name, sep) {
			externalEventMu.RUnlock()
			return strings.TrimSuffix(name, sep), suffix, true
		}
	}
	externalEventMu.RUnlock()
	return "", "", false
}

// translateMethodSignature 把 "方法 (u 用户信息) 打招呼()" 转成 Go 方法签名。
func translateMethodSignature(line string) (string, error) {
	body := strings.TrimPrefix(line, "方法 ")
	// 匹配：(接收者) 名字(参数) 返回类型
	m := reMethodSignature.FindStringSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("无法解析方法签名: %s", body)
	}

	receiver := strings.TrimSpace(m[1])
	name := m[2]
	paramsStr := strings.TrimSpace(m[3])
	retType := strings.TrimSpace(m[4])

	// 解析接收者：名字 类型
	recvFields := strings.Fields(receiver)
	if len(recvFields) != 2 {
		return "", fmt.Errorf("方法接收者格式错误: %s", receiver)
	}
	goReceiver := fmt.Sprintf("(%s %s)", recvFields[0], mapType(recvFields[1]))

	var goParams []string
	if paramsStr != "" {
		parts := splitParams(paramsStr)
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			fields := strings.Fields(p)
			if len(fields) != 2 {
				return "", fmt.Errorf("参数格式错误（期望 名字 类型）: %s", p)
			}
			goParams = append(goParams, fmt.Sprintf("%s %s", fields[0], mapType(fields[1])))
		}
	}

	sig := fmt.Sprintf("func %s %s(%s)", goReceiver, name, strings.Join(goParams, ", "))
	if retType != "" {
		sig += " " + mapType(retType)
	}
	return sig, nil
}

// translateVarDecl 把 "名字 类型"、"名字, 类型" 或 "名字 ＝ 表达式" 转成 Go 变量声明。
func translateVarDecl(line string) (string, error) {
	line = strings.ReplaceAll(line, "，", ",")

	// 短变量声明：名字 ＝ 表达式
	if strings.Contains(line, "＝") {
		parts := strings.SplitN(line, "＝", 2)
		name := strings.TrimSpace(parts[0])
		expr := strings.TrimSpace(parts[1])
		if name == "" || expr == "" {
			return "", fmt.Errorf("变量声明格式错误: %s", line)
		}
		return fmt.Sprintf("%s := %s", name, translateExpression(expr)), nil
	}

	parts := strings.Split(line, ",")
	if len(parts) == 2 {
		name := strings.TrimSpace(parts[0])
		typ := mapType(strings.TrimSpace(parts[1]))
		return fmt.Sprintf("var %s %s", name, typ), nil
	}

	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", fmt.Errorf("变量声明格式错误: %s", line)
	}
	return fmt.Sprintf("var %s %s", fields[0], mapType(fields[1])), nil
}

// translateConstDecl 把 "名字 = 值" 或 "名字 类型 = 值" 转成 Go 常量声明。
func translateConstDecl(line string) (string, error) {
	line = strings.TrimSpace(line)
	// 按第一个等号/全角等号拆分
	idx := -1
	sep := ""
	for i, r := range line {
		if r == '=' || r == '＝' {
			idx = i
			sep = string(r)
			break
		}
	}
	if idx <= 0 {
		return "", fmt.Errorf("常量声明缺少等号: %s", line)
	}
	left := strings.TrimSpace(line[:idx])
	right := strings.TrimSpace(line[idx+len(sep):])

	fields := strings.Fields(left)
	if len(fields) == 0 {
		return "", fmt.Errorf("常量声明格式错误: %s", line)
	}
	name := fields[0]
	if len(fields) >= 2 {
		return fmt.Sprintf("const %s %s = %s", name, mapType(fields[1]), translateExpression(right)), nil
	}
	return fmt.Sprintf("const %s = %s", name, translateExpression(right)), nil
}

// translateExpression 把表达式中的中文运算符替换为 Go 运算符。
// 采用简单的 rune 级扫描，避免把标识符内部的字符（如“加法”中的“加”）误替换。
func translateExpression(expr string) string {
	expr = translateTypeLiterals(expr)

	operators := []struct{ from, to string }{
		{"不等于", "!="},
		{"＞＝", ">="},
		{"＜＝", "<="},
		{"＝＝", "=="},
		{"＝", "="},
		{"≠", "!="},
		{"＞", ">"},
		{"＜", "<"},
		{"≥", ">="},
		{"≤", "<="},
		{"且", "&&"},
		{"或", "||"},
		{"非", "!"},
		{"加", "+"},
		{"减", "-"},
		{"乘", "*"},
		{"除", "/"},
		{"取余", "%"},
	}

	runes := []rune(expr)
	var out strings.Builder
	i := 0
	for i < len(runes) {
		// 跳过字符串字面量
		if runes[i] == '"' || runes[i] == '\'' {
			quote := runes[i]
			out.WriteRune(runes[i])
			i++
			for i < len(runes) && runes[i] != quote {
				out.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				out.WriteRune(runes[i])
				i++
			}
			continue
		}

		matched := false
		for _, op := range operators {
			opRunes := []rune(op.from)
			if i+len(opRunes) <= len(runes) && runeSliceEqual(runes[i:i+len(opRunes)], opRunes) {
				before := i == 0 || !isIdentRune(runes[i-1])
				after := i+len(opRunes) >= len(runes) || !isIdentRune(runes[i+len(opRunes)])
				if before && after {
					out.WriteString(op.to)
					i += len(opRunes)
					matched = true
					break
				}
			}
		}
		if matched {
			continue
		}

		// 中文化布尔字面量
		if i+1 <= len(runes) && runes[i] == '真' {
			before := i == 0 || !isIdentRune(runes[i-1])
			after := i+1 >= len(runes) || !isIdentRune(runes[i+1])
			if before && after {
				out.WriteString("true")
				i++
				continue
			}
		}
		if i+1 <= len(runes) && runes[i] == '假' {
			before := i == 0 || !isIdentRune(runes[i-1])
			after := i+1 >= len(runes) || !isIdentRune(runes[i+1])
			if before && after {
				out.WriteString("false")
				i++
				continue
			}
		}

		out.WriteRune(runes[i])
		i++
	}
	return out.String()
}

// translateTypeLiterals 把中文化类型字面量替换为 Go 字面量。
// 例如：整数数组{1,2} -> []int{1,2}；映射 文本型 整数型{"a":1} -> map[string]int{"a":1}
//
//	文本型(字符) -> string(字符)（类型转换）
func translateTypeLiterals(expr string) string {
	// 数组字面量：整数数组{ -> []int{
	expr = reArrLiteral.ReplaceAllStringFunc(expr, func(s string) string {
		m := reArrLiteral.FindStringSubmatch(s)
		base := m[1]
		return "[]" + mapType(base+"型") + "{"
	})

	// 映射字面量：映射 文本型 整数型{ -> map[string]int{
	expr = reMapLiteral.ReplaceAllStringFunc(expr, func(s string) string {
		m := reMapLiteral.FindStringSubmatch(s)
		return fmt.Sprintf("map[%s]%s{", mapType(m[1]), mapType(m[2]))
	})

	// 类型转换：文本型(x) -> string(x)
	// 注意：跳过字符串字面量内的内容，避免误替换
	expr = replaceTypeCastSkippingStrings(expr)

	return expr
}

// replaceTypeCastSkippingStrings 替换类型转换，但跳过字符串字面量内的内容。
// 例如："文本型(" 在字符串里不替换，但 文本型(字符) 在表达式里替换为 string(字符)。
func replaceTypeCastSkippingStrings(expr string) string {
	var out strings.Builder
	runes := []rune(expr)
	i := 0
	for i < len(runes) {
		// 跳过字符串字面量
		if runes[i] == '"' || runes[i] == '\'' {
			quote := runes[i]
			out.WriteRune(runes[i])
			i++
			for i < len(runes) && runes[i] != quote {
				out.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				out.WriteRune(runes[i])
				i++
			}
			continue
		}
		// 尝试在当前位置匹配类型转换
		rest := string(runes[i:])
		m := reTypeCast.FindStringSubmatch(rest)
		if m != nil {
			// 确保前面不是标识符字符（避免把"我的文本型("误匹配）
			prevIsIdent := i > 0 && isIdentRune(runes[i-1])
			if !prevIsIdent {
				out.WriteString(mapType(m[1]))
				out.WriteString("(")
				i += len([]rune(m[0]))
				continue
			}
		}
		out.WriteRune(runes[i])
		i++
	}
	return out.String()
}

// translateForInit 把 for 循环的 init 子句中的第一个赋值改为短变量声明。
// 例如 "i = 0" 转换为 "i := 0"。
func translateForInit(init string) string {
	init = strings.TrimSpace(init)
	if m := reForInitAssign.FindStringSubmatch(init); m != nil {
		return m[1] + " := " + m[2]
	}
	return init
}

// translateRangeLoop 把 "索引, 值 ＝ 范围 列表" 或 "值 ＝ 范围 列表" 转成 Go range 循环头。
func translateRangeLoop(clause string) (string, error) {
	clause = strings.ReplaceAll(clause, " = 范围 ", " ＝ 范围 ")
	parts := strings.SplitN(clause, " ＝ 范围 ", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("range 循环格式错误: %s", clause)
	}
	varsPart := strings.TrimSpace(parts[0])
	target := strings.TrimSpace(parts[1])

	vars := splitParams(varsPart)
	for i := range vars {
		vars[i] = strings.TrimSpace(vars[i])
	}

	switch len(vars) {
	case 1:
		// 单变量表示值，索引用下划线忽略
		return fmt.Sprintf("for _, %s := range %s {", vars[0], target), nil
	case 2:
		return fmt.Sprintf("for %s, %s := range %s {", vars[0], vars[1], target), nil
	default:
		return "", fmt.Errorf("range 循环变量数量错误: %s", varsPart)
	}
}

func isIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func runeSliceEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// mapType 把中文类型映射为 Go 类型，支持数组、映射等复合类型。
func mapType(typ string) string {
	typ = strings.TrimSpace(typ)

	// 指针类型：*Type → *mapType(Type)
	if strings.HasPrefix(typ, "*") {
		return "*" + mapType(strings.TrimPrefix(typ, "*"))
	}

	// 数组声明类型：整数数组、文本数组
	if strings.HasSuffix(typ, "数组") {
		base := strings.TrimSuffix(typ, "数组")
		return "[]" + mapType(base+"型")
	}

	// 字面量复合类型：[]整数型、map[文本型]整数型
	typ = translateCompositeType(typ)

	switch typ {
	case "整数型":
		return "int"
	case "长整数型":
		return "int64"
	case "短整数型":
		return "int16"
	case "字节型":
		return "byte"
	case "无符号整数型":
		return "uint"
	case "无符号短整数型":
		return "uint16"
	case "无符号长整数型":
		return "uint64"
	case "无符号字节型":
		return "uint8"
	case "有符号8位整数型":
		return "int8"
	case "有符号32位整数型":
		return "int32"
	case "无符号8位整数型":
		return "uint8"
	case "无符号16位整数型":
		return "uint16"
	case "无符号32位整数型":
		return "uint32"
	case "无符号64位整数型":
		return "uint64"
	case "无符号指针整数型":
		return "uintptr"
	case "字符型":
		return "rune"
	case "小数型":
		return "float32"
	case "双精度小数型":
		return "float64"
	case "文本型":
		return "string"
	case "逻辑型":
		return "bool"
	case "变体型":
		return "interface{}"
	case "字节集":
		return "[]byte"
	default:
		// 通道类型：通道 整数型 → chan int
		// 注意：mapType 入参可能是 "通道 整数型" 或 "通道int"，统一处理
		if rest, ok := stripChanPrefix(typ); ok {
			return "chan " + mapType(rest)
		}
		return typ
	}
}

// stripChanPrefix 把 "通道 xxx" 拆分为 "xxx"，用于 chan 类型转译
// 返回 (元素类型, true) 表示是通道类型；否则 (原串, false)
func stripChanPrefix(typ string) (string, bool) {
	if rest, ok := strings.CutPrefix(typ, "通道 "); ok {
		return strings.TrimSpace(rest), true
	}
	if rest, ok := strings.CutPrefix(typ, "通道"); ok {
		// "通道int" 这种无空格形式
		return strings.TrimSpace(rest), true
	}
	return typ, false
}

// translateCompositeType 把 []中文类型 和 map[中文类型]中文类型 中的类型名替换为 Go 类型。
func translateCompositeType(typ string) string {
	// 先处理 map[文本型]整数型，避免被 [] 正则误处理
	if m := reCompositeMap.FindStringSubmatch(typ); m != nil {
		return fmt.Sprintf("map[%s]%s", mapType(m[1]), mapType(m[2]))
	}

	// 再处理 []整数型
	if m := reCompositeArr.FindStringSubmatch(typ); m != nil {
		return "[]" + mapType(m[1])
	}

	return typ
}

// splitParams 按逗号拆分参数列表，忽略字符串内的逗号。
func splitParams(s string) []string {
	var parts []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)

	for _, ch := range s {
		if inString {
			current.WriteRune(ch)
			if ch == stringChar {
				inString = false
			}
			continue
		}

		if ch == '"' || ch == '\'' {
			inString = true
			stringChar = ch
			current.WriteRune(ch)
			continue
		}

		if ch == ',' {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}

		current.WriteRune(ch)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// extractImportPath 从 import 语句行中提取包路径（简单形式）。
func extractImportPath(line string) string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "\"")
	return line
}

// extractIndent 取出行首的缩进空白。
func extractIndent(line string) string {
	var indent strings.Builder
	for _, ch := range line {
		if ch == ' ' || ch == '\t' {
			indent.WriteRune(ch)
		} else {
			break
		}
	}
	return indent.String()
}

// detectSupportCommands 预扫描源码中使用的支持库命令（中文别名或英文键）。
// M7：原实现对每个别名做 strings.Contains 全文扫描（O(N×L)），
// 改为单次正则提取所有标识符后查 map（O(N)），大幅降低扫描成本。
// .elib 额外命令（extraAliases）不影响 imports，此处不处理。
func detectSupportCommands(source string) map[string]bool {
	used := make(map[string]bool)
	for _, ident := range reIdent.FindAllString(source, -1) {
		// 中文别名直接命中
		if _, ok := supportLibrary[ident]; ok {
			used[ident] = true
			continue
		}
		// 英文键经反查表映射回中文别名
		if alias, ok := englishToAlias[ident]; ok {
			used[alias] = true
		}
	}
	return used
}

// collectSupportImports 收集已使用支持库命令需要的额外导入包。
func collectSupportImports(used map[string]bool) []string {
	seen := make(map[string]bool)
	var imports []string
	for alias := range used {
		for _, imp := range supportLibrary[alias].imports {
			if !seen[imp] {
				seen[imp] = true
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// translateSupportCalls 将源码中的中文命令别名替换为英文函数键。
func translateSupportCalls(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		lines[i] = replaceSupportCallsInLine(line)
	}
	return strings.Join(lines, "\n")
}

func replaceSupportCallsInLine(line string) string {
	var out strings.Builder
	runes := []rune(line)
	i := 0
	for i < len(runes) {
		// 跳过字符串字面量
		if runes[i] == '"' || runes[i] == '\'' {
			quote := runes[i]
			out.WriteRune(runes[i])
			i++
			for i < len(runes) && runes[i] != quote {
				out.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				out.WriteRune(runes[i])
				i++
			}
			continue
		}

		// M7：原实现对每个 rune 位置遍历全部别名（O(N×M×L)），
		// 改为提取当前位置最长标识符后查 map（O(N)），命中即替换。
		if isIdentRune(runes[i]) {
			j := i + 1
			for j < len(runes) && isIdentRune(runes[j]) {
				j++
			}
			ident := string(runes[i:j])
			// 仅当标识符后紧跟 '(' 才尝试命令替换
			if j < len(runes) && runes[j] == '(' {
				prev := rune(0)
				if i > 0 {
					prev = runes[i-1]
				}
				// runtimeMethod 命令不允许前缀为 '.'（避免 obj.信息框() 被替换为 obj.runtimeUIService.xxx）
				beforeRuntime := i == 0 || (!isIdentRune(prev) && prev != '.')
				beforePlain := i == 0 || !isIdentRune(prev)

				if beforeRuntime {
					// 外置组件创建命令：创建日期选择器(...) → runtimeUIService.CreateComponent("datepicker", ...)
					// 需要消耗 '(' 并注入组件类型作为第一个参数
					if compType, ok := getExternalCreate(ident); ok && j < len(runes) && runes[j] == '(' {
						out.WriteString("runtimeUIService.CreateComponent(\"" + compType + "\", ")
						i = j + 1 // 跳过 '('，让后续参数原样处理
						continue
					}
					// 中文别名 → runtimeUIService.English
					if cmd, ok := supportLibrary[ident]; ok && cmd.runtimeMethod != "" {
						out.WriteString("runtimeUIService." + cmd.english)
						i = j
						continue
					}
					// 英文键 → runtimeUIService.English
					if alias, ok := englishToAlias[ident]; ok {
						cmd := supportLibrary[alias]
						if cmd.runtimeMethod != "" {
							out.WriteString("runtimeUIService." + cmd.english)
							i = j
							continue
						}
					}
				}
				if beforePlain {
					// 普通命令：中文别名 → 英文键
					if cmd, ok := supportLibrary[ident]; ok && cmd.runtimeMethod == "" {
						out.WriteString(cmd.english)
						i = j
						continue
					}
					// .elib 额外命令：中文别名 → 英文键（函数定义已在 source.eg 中拼接）
					if english, ok := getExtraAlias(ident); ok {
						out.WriteString(english)
						i = j
						continue
					}
				}
			}
			// 未命中命令替换，原样输出标识符
			out.WriteString(ident)
			i = j
			continue
		}

		out.WriteRune(runes[i])
		i++
	}
	return out.String()
}

// normalizeSource 将中文标点符号统一替换为 ASCII，避免引号、括号等导致 Go 编译错误。
// 逗号、句号等分隔符仅在字符串外部替换，以保留字符串内容中的中文标点。
func normalizeSource(src string) string {
	// 去掉 UTF-8 BOM，避免第一行 # 程序集 等指令无法被正确识别。
	if strings.HasPrefix(src, "\xEF\xBB\xBF") {
		src = src[3:]
	}
	// 先把中文引号统一为 ASCII 引号，方便后续按 ASCII 字符串进行保护。
	src = strings.NewReplacer("“", "\"", "”", "\"", "‘", "'", "’", "'").Replace(src)

	bracketRepl := map[rune]rune{
		'（': '(', '）': ')',
		'【': '[', '】': ']',
		'｛': '{', '｝': '}',
		'《': '<', '》': '>',
	}
	sepRepl := map[rune]rune{
		'，': ',', '。': '.', '；': ';', '：': ':', '？': '?',
		'！': '!', '＊': '*', '／': '/', '＼': '\\', '｜': '|',
		'～': '~', '＾': '^', '＿': '_', '＆': '&', '＃': '#',
		'％': '%', '＠': '@', '＄': '$', '　': ' ', // 全角空格 → 半角空格
		// 注意：＝＜＞＋－ 不转换，因为转译器内部用全角做比较运算符替换
	}

	var out strings.Builder
	inString := false
	var quote rune
	for _, r := range src {
		if inString {
			if r == quote {
				inString = false
			}
			out.WriteRune(r)
			continue
		}
		if r == '"' || r == '\'' {
			inString = true
			quote = r
			out.WriteRune(r)
			continue
		}
		if v, ok := bracketRepl[r]; ok {
			out.WriteRune(v)
			continue
		}
		if v, ok := sepRepl[r]; ok {
			out.WriteRune(v)
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
