// Package transpiler — 符号遍历与引用查找
//
// 设计目标：
//   - 提供通用 AST 遍历能力（Walk），供符号查找/引用查找/重命名等编辑器功能复用
//   - FindIdentRefs 收集所有匹配名称的 Ident 节点位置（含定义和引用）
//   - FindDefinition 在 AST 中查找给定名称的定义位置（函数/方法/类型/常量/包级变量/参数/局部变量）
//   - 不依赖源码字符串，所有位置均来自 AST Token 位置信息
//
// 与 CollectSymbols 的关系：
//   - CollectSymbols 只提取顶层符号（函数/方法/类型/常量/包级变量），用于大纲视图
//   - 本文件的 FindDefinition 会下钻到函数体内部，查找参数/局部变量定义
package transpiler

// WalkFunc 是 AST 遍历回调函数类型。
// 参数 node 是当前节点，返回 true 表示继续遍历子节点，false 表示跳过子节点。
type WalkFunc func(node Node) bool

// Walk 遍历 AST，对每个节点调用 fn。
// 起始节点可以是 *File、Decl、Stmt、Expr 任意类型。
func Walk(node Node, fn WalkFunc) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}
	switch n := node.(type) {
	case *File:
		for _, imp := range n.Imports {
			Walk(imp, fn)
		}
		for _, d := range n.Decls {
			Walk(d, fn)
		}
	case *ImportDecl:
		// 叶子节点
	case *FuncDecl:
		for _, p := range n.Params {
			Walk(p, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}
	case *MethodDecl:
		if n.Receiver != nil {
			Walk(n.Receiver, fn)
		}
		for _, p := range n.Params {
			Walk(p, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}
	case *TypeDecl:
		for _, f := range n.Fields {
			Walk(f, fn)
		}
		for _, ei := range n.EmbeddedInterfaces {
			Walk(ei, fn)
		}
	case *EmbeddedInterface:
		// 叶子节点
	case *TypeAliasDecl:
		// 叶子节点（Underlying 是字符串，无子节点）
	case *FieldDecl:
		// 叶子节点
	case *ConstDecl:
		if n.Value != nil {
			Walk(n.Value, fn)
		}
	case *EnumDecl:
		for _, item := range n.Items {
			Walk(item, fn)
		}
	case *EnumItem:
		if n.Value != nil {
			Walk(n.Value, fn)
		}
	case *VarDecl:
		if n.Value != nil {
			Walk(n.Value, fn)
		}
	case *EmbedBlock:
		// 嵌入原生 Go 代码块不深入分析
	case *ParamDecl:
		// 叶子节点
	case *BlockStmt:
		for _, s := range n.Stmts {
			Walk(s, fn)
		}
	case *IfStmt:
		if n.Cond != nil {
			Walk(n.Cond, fn)
		}
		if n.Then != nil {
			Walk(n.Then, fn)
		}
		if n.Else != nil {
			Walk(n.Else, fn)
		}
	case *ForStmt:
		if n.Init != nil {
			Walk(n.Init, fn)
		}
		if n.Cond != nil {
			Walk(n.Cond, fn)
		}
		if n.Post != nil {
			Walk(n.Post, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}
	case *RangeStmt:
		// Key/Value 是字符串，不是 Expr，单独处理
		if n.X != nil {
			Walk(n.X, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}
	case *WhileStmt:
		if n.Cond != nil {
			Walk(n.Cond, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}
	case *SwitchStmt:
		if n.X != nil {
			Walk(n.X, fn)
		}
		for _, c := range n.Cases {
			Walk(c, fn)
		}
	case *CaseClause:
		for _, v := range n.Values {
			Walk(v, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}
	case *SelectStmt:
		for _, c := range n.Cases {
			Walk(c, fn)
		}
	case *CommClause:
		if n.Comm != nil {
			Walk(n.Comm, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}
	case *ReturnStmt:
		for _, v := range n.Values {
			Walk(v, fn)
		}
	case *BreakStmt:
		// 叶子
	case *ContinueStmt:
		// 叶子
	case *LabeledStmt:
		if n.Stmt != nil {
			Walk(n.Stmt, fn)
		}
	case *FallthroughStmt:
		// 叶子
	case *DeferStmt:
		if n.Call != nil {
			Walk(n.Call, fn)
		}
	case *GoStmt:
		if n.Call != nil {
			Walk(n.Call, fn)
		}
	case *PanicStmt:
		if n.X != nil {
			Walk(n.X, fn)
		}
	case *ExprStmt:
		if n.X != nil {
			Walk(n.X, fn)
		}
	case *AssignStmt:
		if n.Lhs != nil {
			Walk(n.Lhs, fn)
		}
		if n.Rhs != nil {
			Walk(n.Rhs, fn)
		}
	case *IncDecStmt:
		if n.X != nil {
			Walk(n.X, fn)
		}
	case *MultiAssignStmt:
		for _, e := range n.Lhs {
			Walk(e, fn)
		}
		for _, e := range n.Rhs {
			Walk(e, fn)
		}
	case *LocalVarDeclStmt:
		if n.Value != nil {
			Walk(n.Value, fn)
		}
	case *Ident:
		// 叶子
	case *Literal:
		// 叶子
	case *BinaryExpr:
		if n.Lhs != nil {
			Walk(n.Lhs, fn)
		}
		if n.Rhs != nil {
			Walk(n.Rhs, fn)
		}
	case *UnaryExpr:
		if n.X != nil {
			Walk(n.X, fn)
		}
	case *ChanExpr:
		if n.Chan != nil {
			Walk(n.Chan, fn)
		}
		if n.Value != nil {
			Walk(n.Value, fn)
		}
	case *TypeConvertExpr:
		if n.Arg != nil {
			Walk(n.Arg, fn)
		}
	case *NewExpr:
		// NewExpr 无子表达式，Type 是字符串无需遍历
	case *FuncLit:
		// 匿名函数字面量：遍历参数和函数体
		for _, p := range n.Params {
			Walk(p, fn)
		}
		if n.Body != nil {
			Walk(n.Body, fn)
		}
	case *RecoverExpr:
		// recover() 无子表达式
	case *IotaExpr:
		// iota 无子表达式
	case *CallExpr:
		if n.Func != nil {
			Walk(n.Func, fn)
		}
		for _, a := range n.Args {
			Walk(a, fn)
		}
	case *MemberExpr:
		if n.X != nil {
			Walk(n.X, fn)
		}
		// Sel 是字符串，无需遍历
	case *IndexExpr:
		if n.X != nil {
			Walk(n.X, fn)
		}
		if n.Index != nil {
			Walk(n.Index, fn)
		}
	case *SliceExpr:
		if n.X != nil {
			Walk(n.X, fn)
		}
		if n.Low != nil {
			Walk(n.Low, fn)
		}
		if n.High != nil {
			Walk(n.High, fn)
		}
		if n.Max != nil {
			Walk(n.Max, fn)
		}
	case *ArrayLiteral:
		for _, e := range n.Elements {
			Walk(e, fn)
		}
	case *MapLiteral:
		for i := range n.Pairs {
			Walk(&n.Pairs[i], fn)
		}
	case *StructLiteral:
		for i := range n.Pairs {
			Walk(&n.Pairs[i], fn)
		}
	case *KeyValueExpr:
		if n.Key != nil {
			Walk(n.Key, fn)
		}
		if n.Value != nil {
			Walk(n.Value, fn)
		}
	case *ParenExpr:
		if n.X != nil {
			Walk(n.X, fn)
		}
	case *TypeAssertExpr:
		if n.X != nil {
			Walk(n.X, fn)
		}
	}
}

// IdentRef 是标识符引用位置（用于查找引用/重命名）
type IdentRef struct {
	Name string
	Pos  Pos
	// Kind 标识引用类型：
	//   "definition" - 定义处（函数名/参数名/变量名等）
	//   "reference"  - 引用处（调用/读取/赋值）
	Kind string
}

// FindIdentRefs 在 AST 中查找所有名为 name 的 Ident 出现位置。
// 同时收集定义位置（函数名/方法名/类型名/常量名/变量名/参数名/局部变量名）和引用位置。
// RangeStmt 的 Key/Value 不是 Ident 节点，单独识别为 definition。
func FindIdentRefs(file *File, name string) []IdentRef {
	if file == nil || name == "" {
		return nil
	}
	var refs []IdentRef

	// 1. 顶层声明名（定义）
	for _, d := range file.Decls {
		switch decl := d.(type) {
		case *FuncDecl:
			if decl.Name == name {
				refs = append(refs, IdentRef{Name: name, Pos: decl.Pos, Kind: "definition"})
			}
		case *MethodDecl:
			if decl.Name == name {
				refs = append(refs, IdentRef{Name: name, Pos: decl.Pos, Kind: "definition"})
			}
		case *TypeDecl:
			if decl.Name == name {
				refs = append(refs, IdentRef{Name: name, Pos: decl.Pos, Kind: "definition"})
			}
		case *TypeAliasDecl:
			if decl.Name == name {
				refs = append(refs, IdentRef{Name: name, Pos: decl.Pos, Kind: "definition"})
			}
		case *ConstDecl:
			if decl.Name == name {
				refs = append(refs, IdentRef{Name: name, Pos: decl.Pos, Kind: "definition"})
			}
		case *EnumDecl:
			// 枚举项作为常量定义
			for _, item := range decl.Items {
				if item.Name == name {
					refs = append(refs, IdentRef{Name: name, Pos: item.Pos, Kind: "definition"})
				}
			}
		case *VarDecl:
			if decl.Name == name {
				refs = append(refs, IdentRef{Name: name, Pos: decl.Pos, Kind: "definition"})
			}
		}
	}

	// 2. 遍历 AST 收集 Ident 引用 + 参数/局部变量定义
	Walk(file, func(node Node) bool {
		switch n := node.(type) {
		case *ParamDecl:
			if n.Name == name {
				refs = append(refs, IdentRef{Name: name, Pos: n.Pos, Kind: "definition"})
			}
		case *LocalVarDeclStmt:
			if n.Name == name {
				refs = append(refs, IdentRef{Name: name, Pos: n.Pos, Kind: "definition"})
			}
		case *RangeStmt:
			if n.Key == name {
				refs = append(refs, IdentRef{Name: name, Pos: n.Pos, Kind: "definition"})
			}
			if n.Value == name {
				refs = append(refs, IdentRef{Name: name, Pos: n.Pos, Kind: "definition"})
			}
		case *Ident:
			if n.Name == name {
				// 如果已经是定义处（顶部已添加），则不重复添加
				// 这里全部标为 reference，由调用方根据位置去重
				refs = append(refs, IdentRef{Name: name, Pos: n.Pos, Kind: "reference"})
			}
		}
		return true
	})

	return refs
}

// FindDefinition 在 AST 中查找给定名称的定义位置。
// 优先级：函数/方法/类型/常量/包级变量 > 参数 > 局部变量
// 返回首个匹配的定义位置；找不到返回零值 Pos{0,0}（line=0 表示未找到）。
func FindDefinition(file *File, name string) (Pos, bool) {
	if file == nil || name == "" {
		return Pos{}, false
	}

	// 1. 优先查找顶层声明
	for _, d := range file.Decls {
		switch decl := d.(type) {
		case *FuncDecl:
			if decl.Name == name {
				return decl.Pos, true
			}
		case *MethodDecl:
			if decl.Name == name {
				return decl.Pos, true
			}
		case *TypeDecl:
			if decl.Name == name {
				return decl.Pos, true
			}
		case *TypeAliasDecl:
			if decl.Name == name {
				return decl.Pos, true
			}
		case *ConstDecl:
			if decl.Name == name {
				return decl.Pos, true
			}
		case *EnumDecl:
			// 枚举项作为常量定义
			for _, item := range decl.Items {
				if item.Name == name {
					return item.Pos, true
				}
			}
		case *VarDecl:
			if decl.Name == name {
				return decl.Pos, true
			}
		}
	}

	// 2. 查找参数和局部变量（任意位置匹配）
	var found Pos
	foundIt := false
	Walk(file, func(node Node) bool {
		if foundIt {
			return false
		}
		switch n := node.(type) {
		case *ParamDecl:
			if n.Name == name {
				found = n.Pos
				foundIt = true
				return false
			}
		case *LocalVarDeclStmt:
			if n.Name == name {
				found = n.Pos
				foundIt = true
				return false
			}
		case *RangeStmt:
			if n.Key == name || n.Value == name {
				found = n.Pos
				foundIt = true
				return false
			}
		}
		return true
	})

	return found, foundIt
}

// FindSymbolAtPosition 返回包含给定位置的符号定义。
// 用于编辑器 hover/call hierarchy：点击某处找出当前在哪个函数/方法内。
// 注意：本函数只查找顶层声明（函数/方法/类型/常量/变量），不查内部符号。
func FindSymbolAtPosition(file *File, line, col int) *Symbol {
	if file == nil {
		return nil
	}
	symbols := CollectSymbols(file)
	for _, sym := range symbols {
		// 简单包含判断：位置在 [Pos, EndPos] 矩形区域内
		if posInRange(sym.Pos, sym.EndPos, line, col) {
			return sym
		}
	}
	return nil
}

// posInRange 判断 (line,col) 是否在 [start,end] 范围内
func posInRange(start, end Pos, line, col int) bool {
	if end.Line == 0 {
		// 没有结束位置，只判断起始行
		return line == start.Line
	}
	if line < start.Line || line > end.Line {
		return false
	}
	if line == start.Line && col < start.Col {
		return false
	}
	if line == end.Line && col > end.Col {
		return false
	}
	return true
}

// SymbolNameKind 返回符号类型的可读字符串（用于 JSON 序列化）
func SymbolNameKind(k SymbolKind) string {
	switch k {
	case SymbolFunc:
		return "function"
	case SymbolMethod:
		return "method"
	case SymbolType:
		return "type"
	case SymbolConst:
		return "const"
	case SymbolVar:
		return "var"
	case SymbolLocal:
		return "local"
	case SymbolParam:
		return "param"
	case SymbolField:
		return "field"
	}
	return "unknown"
}
