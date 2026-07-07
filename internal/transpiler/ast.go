// Package transpiler — AST 节点定义
//
// 设计目标：
//   - 为 EGOU 语言提供结构化的语法树表示
//   - 支持符号提取/查找引用/重命名/悬停提示等编辑器功能
//   - 支持未来基于 AST 的转译器（替代当前正则模式）
//   - 节点保留位置信息（行/列），便于编辑器跳转
//
// 节点层次：
//   File
//   ├─ PackageDecl
//   ├─ ImportDecl
//   └─ Decl (FuncDecl / MethodDecl / TypeDecl / ConstDecl / VarDecl / EmbedBlock)
//      └─ Stmt (IfStmt / ForStmt / WhileStmt / SwitchStmt / ReturnStmt / ...)
//         └─ Expr (Ident / Literal / BinaryExpr / CallExpr / ...)
package transpiler

// Pos 表示源码位置（1-based 行号和列号）
type Pos struct {
	Line int
	Col  int
}

// Node 是所有 AST 节点的公共接口
type Node interface {
	Position() Pos // 节点起始位置
}

// ===== 顶层节点 =====

// File 是整个源码文件的 AST 根节点
type File struct {
	Package       string        // 包名（来自 # 程序集 xxx）
	PkgPos        Pos           // 包声明位置
	Imports       []*ImportDecl // 导入声明
	Decls         []Decl        // 顶层声明（函数/方法/类型/常量/变量/嵌入块）
	TopLevelStmts []Stmt        // 顶层可执行语句（自动包装到 init() 函数，Go 不允许函数外可执行语句）
	FileMarkers   []FileMarker  // #@eg-file 标记（多文件合并时记录每个源文件的起始位置和名字）
}

// FileMarker 记录 #@eg-file 标记，用于多文件合并场景下的 //line 指令生成。
// runner.go mergeLibsFromDir 在每个 .elib 扩展包源码前插入 "#@eg-file global:libs/pkg/source.eg"，
// 转译器据此把后续声明的位置映射回原始 .eg 文件，生成的 Go 代码插入 //line 指令，
// 让 Go 编译器错误指向 .eg 源码行而非合并后的 usercode.go。
// 与正则通道 transpiler.go:339-364 对齐。
type FileMarker struct {
	GlobalLine int    // #@eg-file 标记在合并源码中的行号（1-based）
	FileName   string // 源文件名（如 "global:libs/stringx/source.eg"）
}

// Position 实现 Node 接口
func (f *File) Position() Pos { return f.PkgPos }

// ===== 声明节点 =====

// Decl 是所有顶层声明的接口
type Decl interface {
	Node
	declNode()
}

// ImportDecl 是导入声明（导入 "fmt"）
type ImportDecl struct {
	Path  string // 包路径，如 "fmt"
	Alias string // 别名（可选）
	Pos   Pos
}

func (d *ImportDecl) Position() Pos { return d.Pos }
func (*ImportDecl) declNode()       {}

// FuncDecl 是函数声明（函数 名字(参数) 返回类型 ... 结束函数）
type FuncDecl struct {
	Name        string
	Params      []*ParamDecl
	ReturnTypes []string // 空切片表示无返回值；单元素为单返回值；多元素为多返回值
	Body        *BlockStmt
	Pos         Pos
	EndPos      Pos // 结束函数 关键字位置
}

func (d *FuncDecl) Position() Pos { return d.Pos }
func (*FuncDecl) declNode()       {}

// MethodDecl 是方法声明（方法 (接收者) 名字(参数) 返回类型 ... 结束方法）
type MethodDecl struct {
	Receiver    *ParamDecl
	Name        string
	Params      []*ParamDecl
	ReturnTypes []string
	Body        *BlockStmt
	Pos         Pos
	EndPos      Pos
}

func (d *MethodDecl) Position() Pos { return d.Pos }
func (*MethodDecl) declNode()       {}

// FuncLit 是匿名函数字面量（函数 (参数) 返回类型 ... 结束函数）
// 作为表达式使用，常用于协程/延迟/立即调用/赋值给变量
//   - 协程 worker:  协程 函数() ... 结束函数
//   - 延迟 cleanup: 延迟 函数() ... 结束函数
//   - 立即调用:     函数() ... 结束函数()
//   - 赋值变量:     f ＝ 函数() ... 结束函数
type FuncLit struct {
	Params      []*ParamDecl
	ReturnTypes []string
	Body        *BlockStmt
	Pos         Pos
	EndPos      Pos
}

func (e *FuncLit) Position() Pos { return e.Pos }
func (*FuncLit) exprNode()       {}

// RecoverExpr 是恢复() 表达式，对应 Go recover()
// 只能在 defer 函数内有效，捕获 panic 抛出的值，返回 interface{}
type RecoverExpr struct {
	Pos Pos
}

func (e *RecoverExpr) Position() Pos { return e.Pos }
func (*RecoverExpr) exprNode()       {}

// IotaExpr 是序数 表达式，对应 Go iota 常量生成器
// 只能在 枚举 ... 结束枚举 块内有效，每行自动 +1
type IotaExpr struct {
	Pos Pos
}

func (e *IotaExpr) Position() Pos { return e.Pos }
func (*IotaExpr) exprNode()       {}

// TypeDecl 是类型声明（类型 名字 结构体 ... 结束类型）
type TypeDecl struct {
	Name              string
	Kind              string // "结构体" / "接口"
	Fields            []*FieldDecl        // 结构体字段（Kind == "结构体"）
	Methods           []*MethodSig        // 接口方法（Kind == "接口"）
	EmbeddedInterfaces []*EmbeddedInterface // 接口嵌入的其他接口（Kind == "接口"，Go 接口组合）
	Pos               Pos
	EndPos            Pos
}

func (d *TypeDecl) Position() Pos { return d.Pos }
func (*TypeDecl) declNode()       {}

// EmbeddedInterface 是接口内嵌入的其他接口（Go 接口组合）
// 语法：在 接口 ... 结束类型 块内，独占一行写接口类型名（无括号）
//   类型 读写器 接口
//       读接口
//       写接口
//       关闭()
//   结束类型
// 转译为 Go：type 读写器 interface { 读接口; 写接口; 关闭() }
type EmbeddedInterface struct {
	Name string // 嵌入的接口类型名
	Pos  Pos
}

func (d *EmbeddedInterface) Position() Pos { return d.Pos }

// TypeAliasDecl 是类型别名声明（类型 X ＝ Y → type X = Y）
// Go 1.9+ 特性：X 和 Y 是同一类型的不同名字，无需类型转换即可互相赋值
// 语法：类型 别名 ＝ 原类型
//   - 类型 整数 ＝ 整数型      → type 整数 = int
//   - 类型 IntPtr ＝ *整数型   → type IntPtr = *int
//   - 类型 IntChan ＝ 通道 整数型 → type IntChan = chan int
type TypeAliasDecl struct {
	Name       string // 别名名
	Underlying string // 被别名的原类型（中文，由 gen.go 通过 mapType 转换）
	Pos        Pos
}

func (d *TypeAliasDecl) Position() Pos { return d.Pos }
func (*TypeAliasDecl) declNode()       {}

// MethodSig 是接口方法签名（接口内的方法声明）
// 语法：方法名(参数) 返回类型
type MethodSig struct {
	Name        string
	Params      []*ParamDecl
	ReturnTypes []string
	Pos         Pos
}

func (d *MethodSig) Position() Pos { return d.Pos }

// FieldDecl 是结构体字段
//   - 普通字段：Name + Type（语法：名字, 类型）
//   - 嵌入字段：Embedded == true，Name 为空，Type 是嵌入的类型名（语法：类型，独占一行）
//     对应 Go 的嵌入字段（embedded field），实现字段/方法提升语义
//     支持 *T（指针嵌入）/ 包名.T（限定嵌入）/ 接口嵌入
type FieldDecl struct {
	Name     string
	Type     string
	Embedded bool // 是否为嵌入字段（无字段名，只有类型）
	Pos      Pos
}

func (d *FieldDecl) Position() Pos { return d.Pos }

// ConstDecl 是常量声明（常量 名字 ＝ 值）
type ConstDecl struct {
	Name  string
	Value Expr
	Pos   Pos
}

func (d *ConstDecl) Position() Pos { return d.Pos }
func (*ConstDecl) declNode()       {}

// ConstBlockDecl 是多常量块（常量 ( ... )），对应 Go const ( ... )
// 语法：
//   常量 (
//       名字1 ＝ 表达式
//       名字2 ＝ 表达式
//   )
// 转译为 Go：const ( 名字1 = 表达式; 名字2 = 表达式 )
type ConstBlockDecl struct {
	Items  []*ConstDecl
	Pos    Pos
	EndPos Pos
}

func (d *ConstBlockDecl) Position() Pos { return d.Pos }
func (*ConstBlockDecl) declNode()       {}

// EnumDecl 是枚举声明（枚举 ... 结束枚举），对应 Go 的 const 块 + iota
// 语法：
//   枚举
//       名字1 ＝ 表达式       // 首行带表达式，常含 序数（iota）
//       名字2                 // 省略表达式，自动延续 iota +1
//       名字3
//   结束枚举
// 转译为 Go：
//   const (
//       名字1 = iota          // 表达式中的 序数 替换为 iota
//       名字2                 // 省略表达式
//       名字3
//   )
type EnumDecl struct {
	Items []*EnumItem
	Pos   Pos
	EndPos Pos
}

// EnumItem 是枚举项（名字 ＝ 表达式 或仅 名字）
type EnumItem struct {
	Name  string // 枚举项名字
	Value Expr   // 表达式（可能为 nil，nil 时省略，自动 iota +1）
	HasValue bool // 是否显式写了 ＝ 表达式
	Pos   Pos
}

func (d *EnumDecl) Position() Pos { return d.Pos }
func (*EnumDecl) declNode()       {}

func (d *EnumItem) Position() Pos { return d.Pos }

// VarDecl 是包级变量声明（变量 名字 类型 或 变量 名字 ＝ 值）
type VarDecl struct {
	Name  string
	Type  string // 空串表示从初值推断
	Value Expr   // 可选
	Pos   Pos
}

func (d *VarDecl) Position() Pos { return d.Pos }
func (*VarDecl) declNode()       {}

// VarBlockDecl 是多变量块（变量 ( ... )），对应 Go var ( ... )
// 语法：
//   变量 (
//       名字1 类型
//       名字2 ＝ 表达式
//       名字3, 类型
//   )
// 转译为 Go：var ( 名字1 类型; 名字2 = 表达式; 名字3 类型 )
// 注意：Go 包级 var 块内不允许 := 短声明，必须用 = 或显式类型
type VarBlockDecl struct {
	Items  []*VarDecl
	Pos    Pos
	EndPos Pos
}

func (d *VarBlockDecl) Position() Pos { return d.Pos }
func (*VarBlockDecl) declNode()       {}

// EmbedBlock 是嵌入块（@嵌入 ... @结束），内容原样保留
type EmbedBlock struct {
	Content string // 原始 Go 代码
	Pos     Pos
	EndPos  Pos
}

func (d *EmbedBlock) Position() Pos { return d.Pos }
func (*EmbedBlock) declNode()       {}

// ===== 参数声明 =====

// ParamDecl 是函数参数或方法接收者（参数 名字 类型）
type ParamDecl struct {
	Name     string
	Type     string
	Variadic bool // 可变参数：args ...Type → Go 的 args ...Type
	Pos      Pos
}

func (d *ParamDecl) Position() Pos { return d.Pos }

// ===== 语句节点 =====

// Stmt 是所有语句的接口
type Stmt interface {
	Node
	stmtNode()
}

// BlockStmt 是语句块（包含一组语句）
type BlockStmt struct {
	Stmts []Stmt
	Pos   Pos
	EndPos Pos
}

func (s *BlockStmt) Position() Pos { return s.Pos }
func (*BlockStmt) stmtNode()       {}

// IfStmt 是如果语句（如果 cond ... 否则 ... 结束如果）
type IfStmt struct {
	Cond Expr
	Then *BlockStmt
	Else *BlockStmt // 可能为 nil
	Pos  Pos
}

func (s *IfStmt) Position() Pos { return s.Pos }
func (*IfStmt) stmtNode()       {}

// ForStmt 是循环语句（循环 init; cond; post ... 结束循环）
type ForStmt struct {
	Init Stmt // 可能为 nil
	Cond Expr // 可能为 nil
	Post Stmt // 可能为 nil
	Body *BlockStmt
	Pos  Pos
}

func (s *ForStmt) Position() Pos { return s.Pos }
func (*ForStmt) stmtNode()       {}

// RangeStmt 是范围循环（循环 k, v ＝ 范围 x ... 结束循环）
type RangeStmt struct {
	Key   string // 可能为空
	Value string
	Tok   string // "＝" 或 "="
	X     Expr
	Body  *BlockStmt
	Pos   Pos
}

func (s *RangeStmt) Position() Pos { return s.Pos }
func (*RangeStmt) stmtNode()       {}

// WhileStmt 是判断循环（判断循环 cond ... 结束判断循环，对应 Go 的 do-while）
type WhileStmt struct {
	Cond Expr
	Body *BlockStmt
	Pos  Pos
}

func (s *WhileStmt) Position() Pos { return s.Pos }
func (*WhileStmt) stmtNode()       {}

// SwitchStmt 是选择语句（选择 expr ... 结束选择）
// 也支持 type switch 形式：选择 x ＝ y.(类型) ... 结束选择
// 当 TypeVar 非空时为 type switch，对应 Go "switch x := y.(type) { ... }"
type SwitchStmt struct {
	X       Expr   // 可能为 nil（switch { ... }）；type switch 时是被断言的表达式 y
	TypeVar string // type switch 的变量名（v53 新增）；非空表示 type switch
	Cases   []*CaseClause
	Pos     Pos
}

func (s *SwitchStmt) Position() Pos { return s.Pos }
func (*SwitchStmt) stmtNode()       {}

// CaseClause 是情况分支（情况 val: ... 或 默认: ...）
type CaseClause struct {
	Values []Expr // nil 表示默认分支
	Body   *BlockStmt
	Pos    Pos
}

func (s *CaseClause) Position() Pos { return s.Pos }
func (*CaseClause) stmtNode()       {}

// SelectStmt 是通道选择语句（通道选择 ... 结束通道选择，对应 Go select）
// 每个 case 是一个 CommClause，Comm 为通信语句（接收赋值/接收表达式/发送表达式）
type SelectStmt struct {
	Cases []*CommClause
	Pos   Pos
}

func (s *SelectStmt) Position() Pos { return s.Pos }
func (*SelectStmt) stmtNode()       {}

// CommClause 是 select 的通信分支
//   - 情况 v := <-ch:   Comm 是 *AssignStmt（短声明接收）或 *MultiAssignStmt
//   - 情况 <-ch:        Comm 是 *ExprStmt（X 是 *ChanExpr 接收）
//   - 情况 ch <- v:     Comm 是 *ExprStmt（X 是 *ChanExpr 发送）
//   - 默认:             Comm 为 nil
type CommClause struct {
	Comm Stmt // 通信语句；nil 表示默认分支
	Body *BlockStmt
	Pos  Pos
}

func (s *CommClause) Position() Pos { return s.Pos }
func (*CommClause) stmtNode()       {}

// ReturnStmt 是返回语句（支持多返回值：返回 a, b）
type ReturnStmt struct {
	Values []Expr // 空切片表示无返回值
	Pos    Pos
}

func (s *ReturnStmt) Position() Pos { return s.Pos }
func (*ReturnStmt) stmtNode()       {}

// BreakStmt 是跳出语句（支持可选标签，用于跳出多层循环）
//   - 跳出           → break
//   - 跳出 标签名     → break Label
type BreakStmt struct {
	Label string // 可选标签（空串表示无标签）
	Pos   Pos
}

func (s *BreakStmt) Position() Pos { return s.Pos }
func (*BreakStmt) stmtNode()       {}

// ContinueStmt 是继续语句（支持可选标签，用于继续外层循环的下一次迭代）
//   - 继续           → continue
//   - 继续 标文名     → continue Label
type ContinueStmt struct {
	Label string // 可选标签（空串表示无标签）
	Pos   Pos
}

func (s *ContinueStmt) Position() Pos { return s.Pos }
func (*ContinueStmt) stmtNode()       {}

// LabeledStmt 是标签语句（标签 名字: 语句），用于 break/continue/goto 跳转目标
// Go 语法：Label: Stmt
// EGOU 语法：标签 名字 独占一行
//   - 若下一语句存在，标签修饰该语句（v48 行为，用于 break/continue）
//   - 若下一语句不存在（块结束/EOF），标签独立存在（v52 新增，用于 goto 跳转目标）
type LabeledStmt struct {
	Label string // 标签名
	Stmt  Stmt   // 标签修饰的语句（可能为 nil，nil 时为独立标签，用于 goto 跳转）
	Pos   Pos
}

func (s *LabeledStmt) Position() Pos { return s.Pos }
func (*LabeledStmt) stmtNode()       {}

// GotoStmt 是跳转语句（跳转 名字 → goto 名字，对应 Go goto）
// 跳转到同一函数内已定义的标签（用 标签 名字 声明）
// Go goto 限制：不能跳过变量声明，只能跳到同一函数内已定义的标签
type GotoStmt struct {
	Label string // 跳转目标的标签名
	Pos   Pos
}

func (s *GotoStmt) Position() Pos { return s.Pos }
func (*GotoStmt) stmtNode()       {}

// FallthroughStmt 是穿透语句（穿透 → fallthrough，对应 Go fallthrough）
// 用于 switch case 末尾，强制跳到下一个 case 的体（不评估下一个 case 的条件）
// Go 限制：fallthrough 必须是 case 块的最后一条语句；不能在 type switch 的 case 中使用；
// 不能在最后一个 case/default 中使用。
type FallthroughStmt struct {
	Pos Pos
}

func (s *FallthroughStmt) Position() Pos { return s.Pos }
func (*FallthroughStmt) stmtNode()       {}

// DeferStmt 是延迟语句（延迟 xxx → defer xxx，对应 Go defer）
type DeferStmt struct {
	Call Expr // 延迟执行的调用表达式
	Pos  Pos
}

func (s *DeferStmt) Position() Pos { return s.Pos }
func (*DeferStmt) stmtNode()       {}

// GoStmt 是协程语句（协程 xxx → go xxx，对应 Go go 关键字启动 goroutine）
type GoStmt struct {
	Call Expr // 协程启动的调用表达式
	Pos  Pos
}

func (s *GoStmt) Position() Pos { return s.Pos }
func (*GoStmt) stmtNode()       {}

// PanicStmt 是抛出语句（抛出 expr → panic(expr)，对应 Go panic）
// 用于显式触发运行时错误，配合 恢复() 实现 try-catch 语义
type PanicStmt struct {
	X   Expr // 抛出的值
	Pos Pos
}

func (s *PanicStmt) Position() Pos { return s.Pos }
func (*PanicStmt) stmtNode()       {}

// ExprStmt 是表达式语句（函数调用等）
type ExprStmt struct {
	X   Expr
	Pos Pos
}

func (s *ExprStmt) Position() Pos { return s.Pos }
func (*ExprStmt) stmtNode()       {}

// AssignStmt 是赋值语句（名字 ＝ 表达式）
type AssignStmt struct {
	Lhs Expr
	Op  string // "＝" / "=" / "+=" 等
	Rhs Expr
	Pos Pos
}

func (s *AssignStmt) Position() Pos { return s.Pos }
func (*AssignStmt) stmtNode()       {}

// IncDecStmt 是自增/自减语句（后缀 ++ / --）
type IncDecStmt struct {
	X   Expr
	Tok string // "++" 或 "--"
	Pos Pos
}

func (s *IncDecStmt) Position() Pos { return s.Pos }
func (*IncDecStmt) stmtNode()       {}

// MultiAssignStmt 是多变量赋值/短声明语句（a, b ＝ 1, 2 或 a, b := f()）
// 支持：
//   - a, b ＝ 1, 2 → a, b = 1, 2
//   - a, b := f() → a, b := f()（短声明，新增变量）
//   - a, b ＝ f() → a, b = f()（多返回值赋值给已声明变量）
type MultiAssignStmt struct {
	Lhs []Expr    // 左侧表达式列表（通常是 *Ident）
	Op  string    // "＝" / "=" / ":="
	Rhs []Expr    // 右侧表达式列表
	Pos Pos
}

func (s *MultiAssignStmt) Position() Pos { return s.Pos }
func (*MultiAssignStmt) stmtNode()       {}

// LocalVarDeclStmt 是局部变量声明（局部变量 名字, 类型）
type LocalVarDeclStmt struct {
	Name  string
	Type  string
	Value Expr // 可选（局部变量 名字 ＝ 值）
	Pos   Pos
}

func (s *LocalVarDeclStmt) Position() Pos { return s.Pos }
func (*LocalVarDeclStmt) stmtNode()       {}

// ===== 表达式节点 =====

// Expr 是所有表达式的接口
type Expr interface {
	Node
	exprNode()
}

// Ident 是标识符（变量名/函数名/类型名等）
type Ident struct {
	Name string
	Pos  Pos
}

func (e *Ident) Position() Pos { return e.Pos }
func (*Ident) exprNode()       {}

// Literal 是字面量（数字/字符串/字符/真/假/空）
type Literal struct {
	Value string // 原始字面量文本
	Kind  string // "number" / "string" / "char" / "bool" / "nil"
	Pos   Pos
}

func (e *Literal) Position() Pos { return e.Pos }
func (*Literal) exprNode()       {}

// BinaryExpr 是二元运算表达式（a + b / a ＞ b 等）
type BinaryExpr struct {
	Op  string
	Lhs Expr
	Rhs Expr
	Pos Pos
}

func (e *BinaryExpr) Position() Pos { return e.Pos }
func (*BinaryExpr) exprNode()       {}

// UnaryExpr 是一元运算表达式（!x / -x 等）
type UnaryExpr struct {
	Op  string
	X   Expr
	Pos Pos
}

func (e *UnaryExpr) Position() Pos { return e.Pos }
func (*UnaryExpr) exprNode()       {}

// CallExpr 是函数调用表达式（f(args...)）
type CallExpr struct {
	Func     Expr // 通常是 *Ident 或 *MemberExpr
	Args     []Expr
	Ellipsis bool // 可变参数展开：f(args...) → Go 的 f(args...)
	Pos      Pos
}

func (e *CallExpr) Position() Pos { return e.Pos }
func (*CallExpr) exprNode()       {}

// MemberExpr 是成员访问表达式（x.y）
type MemberExpr struct {
	X   Expr
	Sel string
	Pos Pos
}

func (e *MemberExpr) Position() Pos { return e.Pos }
func (*MemberExpr) exprNode()       {}

// IndexExpr 是索引表达式（x[i]）
type IndexExpr struct {
	X     Expr
	Index Expr
	Pos   Pos
}

func (e *IndexExpr) Position() Pos { return e.Pos }
func (*IndexExpr) exprNode()       {}

// SliceExpr 是切片表达式（x[low:high] / x[:high] / x[low:] / x[:] / x[low:high:max]）
// Low/High/Max 为 nil 表示省略该端
type SliceExpr struct {
	X    Expr
	Low  Expr // 可选（nil 表示省略）
	High Expr // 可选（nil 表示省略）
	Max  Expr // 可选（nil 表示省略，三元切片才用）
	Pos  Pos
}

func (e *SliceExpr) Position() Pos { return e.Pos }
func (*SliceExpr) exprNode()       {}

// ArrayLiteral 是数组字面量（整数数组{1,2,3}）
type ArrayLiteral struct {
	ElemType string
	Elements []Expr
	Pos      Pos
}

func (e *ArrayLiteral) Position() Pos { return e.Pos }
func (*ArrayLiteral) exprNode()       {}

// MapLiteral 是映射字面量（映射 文本型 整数型{"a":1}）
type MapLiteral struct {
	KeyType   string
	ValueType string
	Pairs     []KeyValueExpr
	Pos       Pos
}

func (e *MapLiteral) Position() Pos { return e.Pos }
func (*MapLiteral) exprNode()       {}

// KeyValueExpr 是映射键值对
type KeyValueExpr struct {
	Key   Expr
	Value Expr
	Pos   Pos
}

func (e *KeyValueExpr) Position() Pos { return e.Pos }
func (*KeyValueExpr) exprNode()       {}

// StructLiteral 是结构体字面量（Point{1, 2} 或 Point{x: 1, y: 2}）
// 用 KeyValueExpr.Pairs 表示字段名:值；Key 为 nil 时表示按位置赋值
type StructLiteral struct {
	TypeName string          // 结构体类型名
	Pairs    []KeyValueExpr  // 字段键值对；Key 为 nil 表示按位置
	Pos      Pos
}

func (e *StructLiteral) Position() Pos { return e.Pos }
func (*StructLiteral) exprNode()       {}

// ChanExpr 是通道操作表达式
//   - 接收：Op == "<-"，Value 为 nil（一元前缀 `<-ch`）
//   - 发送：Op == "<-"，Value 非 nil（二元 `ch <- value`）
type ChanExpr struct {
	Op    string // 固定 "<-"
	Chan  Expr   // 通道变量
	Value Expr   // 发送的值；接收时为 nil
	Pos   Pos
}

func (e *ChanExpr) Position() Pos { return e.Pos }
func (*ChanExpr) exprNode()       {}

// TypeConvertExpr 是类型转换表达式 整数型(x) → int(x)
// 中文类型名通过 mapType 映射为 Go 类型名后，按函数调用形式输出
type TypeConvertExpr struct {
	Type string // 目标类型（中文，如 "整数型"）
	Arg  Expr   // 要转换的表达式
	Pos  Pos
}

func (e *TypeConvertExpr) Position() Pos { return e.Pos }
func (*TypeConvertExpr) exprNode()       {}

// NewExpr 是新建(T) 表达式，对应 Go 的 new(T)，返回 *T 零值指针
// 语法：新建 ( 类型 )
type NewExpr struct {
	Type string // 类型名（中文，如 "整数型"/"用户信息"，由 gen.go 通过 mapType 转换）
	Pos  Pos
}

func (e *NewExpr) Position() Pos { return e.Pos }
func (*NewExpr) exprNode()       {}

// ChanMakeExpr 是通道初始化表达式，对应 Go 的 make(chan T)
// 语法：新建 通道 元素类型（在通道变量声明中作为初始值）
// 与 NewExpr 区分：通道必须用 make 而非 new 初始化
type ChanMakeExpr struct {
	ElemType string // 元素类型（中文，如 "整数型"，由 gen.go 通过 mapType 转换）
	Pos      Pos
}

func (e *ChanMakeExpr) Position() Pos { return e.Pos }
func (*ChanMakeExpr) exprNode()       {}

// ParenExpr 是括号表达式（(x)）
type ParenExpr struct {
	X   Expr
	Pos Pos
}

func (e *ParenExpr) Position() Pos { return e.Pos }
func (*ParenExpr) exprNode()       {}

// TypeAssertExpr 是类型断言表达式（x.(Type)）
// Type 为空字符串表示 comma-ok 形式的 type switch：switch v := x.(type)
type TypeAssertExpr struct {
	X    Expr
	Type string // 断言的目标类型；空字符串表示 x.(type) 形式
	Pos  Pos
}

func (e *TypeAssertExpr) Position() Pos { return e.Pos }
func (*TypeAssertExpr) exprNode()       {}

// ===== 符号提取辅助 =====

// SymbolKind 表示符号种类
type SymbolKind int

const (
	SymbolFunc    SymbolKind = iota // 函数
	SymbolMethod                    // 方法
	SymbolType                      // 类型
	SymbolConst                     // 常量
	SymbolVar                       // 变量（包级）
	SymbolLocal                     // 局部变量
	SymbolParam                     // 参数
	SymbolField                     // 字段
)

// Symbol 是符号信息（用于编辑器大纲/跳转/查找引用）
type Symbol struct {
	Name     string
	Kind     SymbolKind
	Pos      Pos
	EndPos   Pos
	// 函数/方法专属
	Params     []*ParamDecl
	ReturnType string
	// 类型专属
	Fields []*FieldDecl
	// 所属文件（用于跨文件查找引用）
	File string
}

// String 返回符号的可读表示
func (s Symbol) String() string {
	switch s.Kind {
	case SymbolFunc:
		return "函数 " + s.Name
	case SymbolMethod:
		return "方法 " + s.Name
	case SymbolType:
		return "类型 " + s.Name
	case SymbolConst:
		return "常量 " + s.Name
	case SymbolVar:
		return "变量 " + s.Name
	case SymbolLocal:
		return "局部变量 " + s.Name
	case SymbolParam:
		return "参数 " + s.Name
	case SymbolField:
		return "字段 " + s.Name
	}
	return s.Name
}

// joinReturnTypes 将返回类型列表拼接为前端可读字符串
// 空切片返回 ""，单元素返回 "整数型"，多元素返回 "整数型, 文本型"
// 用于 Symbol.ReturnType 字段，保持与前端 returnType JSON 字段兼容
func joinReturnTypes(types []string) string {
	out := ""
	for i, t := range types {
		if i > 0 {
			out += ", "
		}
		out += t
	}
	return out
}

// CollectSymbols 从 AST 提取所有符号定义（函数/方法/类型/常量/包级变量）
func CollectSymbols(file *File) []*Symbol {
	if file == nil {
		return nil
	}
	var symbols []*Symbol
	for _, d := range file.Decls {
		switch decl := d.(type) {
		case *FuncDecl:
			symbols = append(symbols, &Symbol{
				Name:       decl.Name,
				Kind:       SymbolFunc,
				Pos:        decl.Pos,
				EndPos:     decl.EndPos,
				Params:     decl.Params,
				ReturnType: joinReturnTypes(decl.ReturnTypes),
			})
		case *MethodDecl:
			symbols = append(symbols, &Symbol{
				Name:       decl.Name,
				Kind:       SymbolMethod,
				Pos:        decl.Pos,
				EndPos:     decl.EndPos,
				Params:     decl.Params,
				ReturnType: joinReturnTypes(decl.ReturnTypes),
			})
		case *TypeDecl:
			symbols = append(symbols, &Symbol{
				Name:   decl.Name,
				Kind:   SymbolType,
				Pos:    decl.Pos,
				EndPos: decl.EndPos,
				Fields: decl.Fields,
			})
		case *TypeAliasDecl:
			symbols = append(symbols, &Symbol{
				Name:   decl.Name,
				Kind:   SymbolType,
				Pos:    decl.Pos,
			})
		case *ConstDecl:
			symbols = append(symbols, &Symbol{
				Name: decl.Name,
				Kind: SymbolConst,
				Pos:  decl.Pos,
			})
		case *EnumDecl:
			// 枚举项作为常量符号
			for _, item := range decl.Items {
				symbols = append(symbols, &Symbol{
					Name: item.Name,
					Kind: SymbolConst,
					Pos:  item.Pos,
				})
			}
		case *VarDecl:
			symbols = append(symbols, &Symbol{
				Name: decl.Name,
				Kind: SymbolVar,
				Pos:  decl.Pos,
			})
		}
	}
	return symbols
}
