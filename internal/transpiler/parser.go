// Package transpiler — 递归下降解析器
//
// 设计目标：
//   - 基于 lexer.go 的 Token 流构建 AST
//   - 支持符号提取/查找引用/重命名等编辑器功能
//   - 错误恢复：一处错误不立即停止，记录后跳过继续解析
//   - 当前作为正则 transpiler 的并行实验，未来可替代之
//
// 解析策略：
//   - 顶层声明：函数/方法/类型/常量/变量/导入/程序集/嵌入块
//   - 函数体语句：如果/循环/判断循环/选择/返回/继续/跳出/局部变量/赋值/表达式
//   - 表达式：二元/一元/字面量/标识符/调用/成员/索引/数组字面量/映射字面量
package transpiler

import (
	"fmt"
	"strings"
)

// ParseError 是解析错误（包含位置信息）
type ParseError struct {
	Pos Pos
	Msg string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("第 %d 行第 %d 列: %s", e.Pos.Line, e.Pos.Col, e.Msg)
}

// Parser 是递归下降解析器
type Parser struct {
	tokens []Token // 输入 Token 流（含所有类型，含空白）
	pos    int     // 当前 Token 索引
	errors []error // 错误列表（支持错误恢复）
	src    string  // 原始源码（用于提取嵌入块内容）
	lines  []string // 源码按行切分（用于嵌入块内容提取）
}

// NewParser 创建解析器
func NewParser(src string) *Parser {
	return &Parser{
		tokens: Tokenize(src),
		src:    src,
		lines:  strings.Split(src, "\n"),
	}
}

// Parse 解析整个文件，返回 AST 根节点和错误列表
func Parse(src string) (*File, []error) {
	p := NewParser(src)
	return p.parseFile(), p.errors
}

// ===== Token 导航辅助 =====

// peek 返回当前 Token（不消费）
func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

// peekFiltered 返回当前非空白 Token（不消费）
func (p *Parser) peekFiltered() Token {
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		if t.Type != TokenWhitespace && t.Type != TokenNewline {
			return t
		}
		p.pos++
	}
	return Token{Type: TokenEOF}
}

// next 返回当前 Token 并前进
func (p *Parser) next() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	t := p.tokens[p.pos]
	p.pos++
	return t
}

// nextFiltered 返回当前非空白 Token 并前进
func (p *Parser) nextFiltered() Token {
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		p.pos++
		if t.Type != TokenWhitespace && t.Type != TokenNewline {
			return t
		}
	}
	return Token{Type: TokenEOF}
}

// errorf 记录错误并继续解析
func (p *Parser) errorf(pos Pos, format string, args ...interface{}) {
	p.errors = append(p.errors, &ParseError{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// v59 新增：错误恢复辅助函数
// syncToNextStatement 跳过 Token 直到遇到语句开始的标记或 EOF
// 这让解析器在遇到严重错误后能同步到下一个合理的语句开始位置，继续解析后续代码
func (p *Parser) syncToNextStatement() {
	statementStartKeywords := map[string]bool{
		// 块开始关键字
		"函数": true, "方法": true, "类型": true, "枚举": true, "常量": true, "变量": true, "导入": true, "程序集": true,
		// 控制流关键字
		"如果": true, "循环": true, "判断循环": true, "选择": true, "通道选择": true,
		"返回": true, "继续": true, "跳出": true, "延迟": true, "协程": true, "抛出": true, "穿透": true,
		// 块结束关键字（作为同步点）
		"结束函数": true, "结束方法": true, "结束类型": true, "结束枚举": true,
		"结束如果": true, "结束循环": true, "结束判断循环": true, "结束选择": true, "结束通道选择": true,
		"否则": true, "否则如果": true, "情况": true, "默认": true,
		// 其他
		"局部变量": true,
	}

	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && statementStartKeywords[t.Value] {
			break
		}
		// 跳过当前 Token，继续检查下一个
		p.nextFiltered()
	}
}

// expectKeyword 消费一个关键字 Token，不匹配则记录错误
func (p *Parser) expectKeyword(kw string) (Token, bool) {
	t := p.peekFiltered()
	if t.Type == TokenKeyword && t.Value == kw {
		return p.nextFiltered(), true
	}
	p.errorf(Pos{t.Line, t.Col}, "期望关键字 %q，实际为 %q", kw, t.Value)
	return t, false
}

// consumeKeyword 消费一个关键字 Token（如匹配返回 true，不匹配返回 false 不报错）
func (p *Parser) consumeKeyword(kw string) bool {
	t := p.peekFiltered()
	if t.Type == TokenKeyword && t.Value == kw {
		p.nextFiltered()
		return true
	}
	return false
}

// consumeDelimiter 消费一个分隔符 Token
func (p *Parser) consumeDelimiter(d string) bool {
	t := p.peekFiltered()
	if t.Type == TokenDelimiter && t.Value == d {
		p.nextFiltered()
		return true
	}
	return false
}

// ===== 顶层解析 =====

// parseFile 解析整个文件
func (p *Parser) parseFile() *File {
	file := &File{}

	// 跳过开头的注释和空白
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			return file
		}
		// # 程序集 main —— 注意 lexer 把 # 程序集 识别为 TokenComment（因为 # 开头）
		if t.Type == TokenComment && strings.HasPrefix(t.Value, "# 程序集") {
			pkgToken := p.nextFiltered()
			file.Package = strings.TrimSpace(strings.TrimPrefix(pkgToken.Value, "# 程序集"))
			if file.Package == "" {
				file.Package = "main"
			}
			file.PkgPos = Pos{pkgToken.Line, pkgToken.Col}
			break
		}
		// 不是 # 程序集 开头也允许（默认 main）
		break
	}

	// 解析顶层声明
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}

		// 跳过注释（ #@eg-file 标记除外：多文件合并时记录源文件边界，供 gen.go 生成 //line 指令）
		if t.Type == TokenComment {
			if strings.HasPrefix(t.Value, "#@eg-file ") {
				fileName := strings.TrimSpace(strings.TrimPrefix(t.Value, "#@eg-file "))
				file.FileMarkers = append(file.FileMarkers, FileMarker{
					GlobalLine: t.Line,
					FileName:   fileName,
				})
			}
			p.nextFiltered()
			continue
		}

		// @嵌入 / @结束（顶层嵌入块）
		if t.Type == TokenEmbed {
			decl := p.parseEmbedBlock()
			if decl != nil {
				file.Decls = append(file.Decls, decl)
			}
			continue
		}

		// 关键字驱动的声明
		if t.Type == TokenKeyword {
			switch t.Value {
			case "导入":
				decl := p.parseImportDecl()
				if decl != nil {
					file.Imports = append(file.Imports, decl)
				}
			case "函数", "主函数":
				decl := p.parseFuncDecl()
				if decl != nil {
					file.Decls = append(file.Decls, decl)
				}
			case "初始化":
				// 初始化 关键字 → Go init 函数（无参数无返回值，包初始化时自动调用）
				decl := p.parseInitDecl()
				if decl != nil {
					file.Decls = append(file.Decls, decl)
				}
			case "方法":
				decl := p.parseMethodDecl()
				if decl != nil {
					file.Decls = append(file.Decls, decl)
				}
			case "类型":
				decl := p.parseTypeDecl()
				if decl != nil {
					file.Decls = append(file.Decls, decl)
				}
			case "常量":
				decl := p.parseConstDecl()
				if decl != nil {
					file.Decls = append(file.Decls, decl)
				}
			case "枚举":
				decl := p.parseEnumDecl()
				if decl != nil {
					file.Decls = append(file.Decls, decl)
				}
			case "变量":
				decl := p.parseVarDecl()
				if decl != nil {
					file.Decls = append(file.Decls, decl)
				}
			default:
				// v59 改进：跳过未知关键字并同步到下一个语句开始位置（错误恢复）
				p.errorf(Pos{t.Line, t.Col}, "函数外部不允许关键字 %q", t.Value)
				// 先消费当前未知关键字，再同步到下一个语句开始位置。
				// 否则 syncToNextStatement 会 peek 到当前关键字（是语句开始关键字）直接 break，导致死循环。
				p.nextFiltered()
				p.syncToNextStatement()
			}
			continue
		}

		// 尝试解析为顶层可执行语句（赋值/函数调用等）
		// Go 不允许函数外有可执行语句，转译器自动包装到 init() 函数
		// （与正则通道 transpiler.go:399-409 对齐）
		if stmt := p.parseExprOrAssignStmt(); stmt != nil {
			file.TopLevelStmts = append(file.TopLevelStmts, stmt)
			continue
		}

		// 跳过未知 Token（错误恢复）
		p.nextFiltered()
	}

	return file
}

// parseImportDecl 解析导入声明（导入 (\n "fmt"\n "egou/ui"\n)）
func (p *Parser) parseImportDecl() *ImportDecl {
	t := p.peekFiltered()
	decl := &ImportDecl{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("导入")

	// 导入 ( 多行形式
	if p.consumeDelimiter("(") {
		for {
			t := p.peekFiltered()
			if t.Type == TokenEOF {
				break
			}
			if t.Type == TokenDelimiter && t.Value == ")" {
				p.nextFiltered()
				break
			}
			// 每行是一个字符串字面量（包路径），可能有别名
			if t.Type == TokenString {
				strTok := p.nextFiltered()
				path := strings.Trim(strTok.Value, `"`)
				decl = &ImportDecl{Path: path, Pos: Pos{strTok.Line, strTok.Col}}
				// 这里应该 append 到一个列表，但简化处理只保留最后一个
				// 实际上需要重构 parseImportDecl 返回 []*ImportDecl
			} else {
				p.nextFiltered()
			}
		}
		return decl
	}

	// 导入 "fmt" 单行形式
	if t.Type == TokenString {
		strTok := p.nextFiltered()
		decl.Path = strings.Trim(strTok.Value, `"`)
	}

	return decl
}

// parseFuncDecl 解析函数声明（函数 名字(参数) 返回类型 ... 结束函数）
func (p *Parser) parseFuncDecl() *FuncDecl {
	t := p.peekFiltered()
	decl := &FuncDecl{Pos: Pos{t.Line, t.Col}}

	// 消费 "函数" 或 "主函数"
	p.nextFiltered()

	// 函数名
	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText && nameTok.Type != TokenKeyword {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "期望函数名，实际为 %q", nameTok.Value)
		return nil
	}
	decl.Name = nameTok.Value
	p.nextFiltered()

	// 参数列表 (参数 a 整数型, 参数 b 整数型)
	if !p.consumeDelimiter("(") {
		p.errorf(decl.Pos, "函数 %s 缺少左括号", decl.Name)
		return decl
	}
	decl.Params = p.parseParamList()
	p.consumeDelimiter(")")

	// 返回类型（可选）：支持单类型 "整数型" 和多类型 "(整数型, 文本型)"
	decl.ReturnTypes = p.parseReturnTypes("结束函数")

	// 函数体
	decl.Body = p.parseBlockStmt("结束函数", &decl.EndPos)
	return decl
}

// parseInitDecl 解析 init 函数声明（初始化 ... 结束函数 → func init() { ... }）
// Go init 函数特性：无参数、无返回值、包初始化时自动调用、可定义多个 init
// EGOU 用 初始化 关键字糖衣，转译为 Go func init()
func (p *Parser) parseInitDecl() *FuncDecl {
	t := p.peekFiltered()
	decl := &FuncDecl{Name: "init", Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("初始化")

	// 可选的 () — init 函数无参数，但允许用户写 () 提升可读性
	p.consumeDelimiter("(")
	p.consumeDelimiter(")")

	// init 函数无返回类型（即使写了也会被忽略，Go 语法禁止）

	// 函数体
	decl.Body = p.parseBlockStmt("结束函数", &decl.EndPos)
	return decl
}

// parseMethodDecl 解析方法声明（方法 (接收者) 名字(参数) 返回类型 ... 结束方法）
func (p *Parser) parseMethodDecl() *MethodDecl {
	t := p.peekFiltered()
	decl := &MethodDecl{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("方法")

	// 接收者 (名字 类型)
	if !p.consumeDelimiter("(") {
		p.errorf(decl.Pos, "方法缺少接收者左括号")
		return decl
	}
	recvNameTok := p.peekFiltered()
	if recvNameTok.Type != TokenIdentifier && recvNameTok.Type != TokenChineseText {
		p.errorf(Pos{recvNameTok.Line, recvNameTok.Col}, "方法接收者缺少名字")
		return decl
	}
	recvName := recvNameTok.Value
	p.nextFiltered()
	// 通用类型解析：支持 *Type / 通道 Type / 组合
	recvType := p.parseType()
	decl.Receiver = &ParamDecl{Name: recvName, Type: recvType, Pos: Pos{recvNameTok.Line, recvNameTok.Col}}
	p.consumeDelimiter(")")

	// 方法名
	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "方法缺少名字")
		return decl
	}
	decl.Name = nameTok.Value
	p.nextFiltered()

	// 参数列表
	if !p.consumeDelimiter("(") {
		p.errorf(decl.Pos, "方法 %s 缺少左括号", decl.Name)
		return decl
	}
	decl.Params = p.parseParamList()
	p.consumeDelimiter(")")

	// 返回类型（可选）：支持单类型和多类型
	decl.ReturnTypes = p.parseReturnTypes("结束方法")

	// 方法体
	decl.Body = p.parseBlockStmt("结束方法", &decl.EndPos)
	return decl
}

// parseTypeDecl 解析类型声明（类型 名字 结构体 ... 结束类型 / 类型 名字 接口 ... 结束类型）
// 也处理类型别名（类型 X ＝ Y），返回 Decl 接口以支持两种返回类型
func (p *Parser) parseTypeDecl() Decl {
	t := p.peekFiltered()
	pos := Pos{t.Line, t.Col}
	p.consumeKeyword("类型")

	// 类型名
	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "类型缺少名字")
		return nil
	}
	name := nameTok.Value
	p.nextFiltered()

	// 检测类型别名：类型 X ＝ Y
	if nextTok := p.peekFiltered(); nextTok.Type == TokenOperator && (nextTok.Value == "＝" || nextTok.Value == "=") {
		p.nextFiltered() // 消费 ＝
		underlying := p.parseType()
		return &TypeAliasDecl{Name: name, Underlying: underlying, Pos: pos}
	}

	// 普通类型声明（结构体 / 接口）
	decl := &TypeDecl{Name: name, Pos: pos}

	// Kind（结构体 / 接口）
	kindTok := p.peekFiltered()
	if kindTok.Type == TokenKeyword && kindTok.Value == "结构体" {
		decl.Kind = "结构体"
		p.nextFiltered()
	} else if kindTok.Type == TokenKeyword && kindTok.Value == "接口" {
		decl.Kind = "接口"
		p.nextFiltered()
	} else {
		p.errorf(Pos{kindTok.Line, kindTok.Col}, "类型 %s 缺少种类（结构体/接口）", decl.Name)
	}

	// 字段列表 / 方法列表
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && t.Value == "结束类型" {
			decl.EndPos = Pos{t.Line, t.Col}
			p.nextFiltered()
			break
		}
		// 名字
		nameTok := p.peekFiltered()

		if decl.Kind == "接口" {
			// 接口成员：方法签名 或 嵌入接口
			if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText && nameTok.Type != TokenKeywordType {
				p.nextFiltered()
				continue
			}
			name := nameTok.Value
			pos := Pos{nameTok.Line, nameTok.Col}
			p.nextFiltered()
			// 区分：方法签名（后跟 `(`）/ 限定嵌入接口（后跟 `.`）/ 普通嵌入接口（后跟其他）
			if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "(" {
				// 接口方法签名：方法名(参数) 返回类型
				p.consumeDelimiter("(")
				params := p.parseParamList()
				p.consumeDelimiter(")")
				returnTypes := p.parseReturnTypes("结束类型")
				decl.Methods = append(decl.Methods, &MethodSig{Name: name, Params: params, ReturnTypes: returnTypes, Pos: pos})
			} else if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "." {
				// 限定嵌入接口：包名.T（name 是包名，后跟 .T）
				p.nextFiltered() // 消费 .
				nt := p.peekFiltered()
				if nt.Type != TokenIdentifier && nt.Type != TokenChineseText && nt.Type != TokenKeywordType {
					p.errorf(pos, "限定嵌入接口 %q 后缺少类型名", name)
					decl.EmbeddedInterfaces = append(decl.EmbeddedInterfaces, &EmbeddedInterface{Name: name, Pos: pos})
				} else {
					p.nextFiltered()
					qualified := name + "." + nt.Value
					// 支持多段限定：包名.子包.T
					for p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "." {
						p.nextFiltered()
						mt := p.peekFiltered()
						if mt.Type != TokenIdentifier && mt.Type != TokenChineseText && mt.Type != TokenKeywordType {
							p.errorf(pos, "限定嵌入接口 %q 后缺少类型名", qualified)
							break
						}
						p.nextFiltered()
						qualified = qualified + "." + mt.Value
					}
					decl.EmbeddedInterfaces = append(decl.EmbeddedInterfaces, &EmbeddedInterface{Name: qualified, Pos: pos})
				}
			} else {
				// 嵌入接口：name 即嵌入的接口类型名（Go 接口组合）
				decl.EmbeddedInterfaces = append(decl.EmbeddedInterfaces, &EmbeddedInterface{Name: name, Pos: pos})
			}
		} else {
			// 结构体字段：区分普通字段和嵌入字段
			//   - 嵌入字段（*T 指针嵌入）：以 * 开头，调用 parseType 解析整个类型（含 *包名.T）
			//   - 嵌入字段（T 类型嵌入）：name 后无逗号，name 即类型名
			//   - 嵌入字段（包名.T 限定嵌入）：name 后是 `.`，组合成限定类型名
			//   - 普通字段：name, 类型
			pos := Pos{nameTok.Line, nameTok.Col}
			if nameTok.Type == TokenOperator && strings.Trim(nameTok.Value, "*") == "" && nameTok.Value != "" {
				// 指针嵌入字段：*T（parseType 处理连续 * 序列 + 包名.T 限定）
				embeddedType := p.parseType()
				decl.Fields = append(decl.Fields, &FieldDecl{Name: "", Type: embeddedType, Embedded: true, Pos: pos})
			} else if nameTok.Type == TokenIdentifier || nameTok.Type == TokenChineseText || nameTok.Type == TokenKeywordType {
				// 消费 name，再 peek 是否是逗号
				name := nameTok.Value
				p.nextFiltered()
				if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "," {
					// 普通字段：名字, 类型
					p.consumeDelimiter(",")
					fieldType := p.parseType()
					decl.Fields = append(decl.Fields, &FieldDecl{Name: name, Type: fieldType, Pos: pos})
				} else if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "." {
					// 限定嵌入字段：包名.T（name 是包名，后跟 .T）
					// 复用 parseType 的限定类型逻辑：把 name 放回，调用 parseType
					// 但 parseType 会重新 peek 并消费 name，这里已消费 name，需手动组合
					p.nextFiltered() // 消费 .
					nt := p.peekFiltered()
					if nt.Type != TokenIdentifier && nt.Type != TokenChineseText && nt.Type != TokenKeywordType {
						p.errorf(pos, "限定嵌入字段 %q 后缺少类型名", name)
						decl.Fields = append(decl.Fields, &FieldDecl{Name: "", Type: name, Embedded: true, Pos: pos})
					} else {
						p.nextFiltered()
						qualified := name + "." + nt.Value
						// 支持多段限定：包名.子包.T
						for p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "." {
							p.nextFiltered()
							mt := p.peekFiltered()
							if mt.Type != TokenIdentifier && mt.Type != TokenChineseText && mt.Type != TokenKeywordType {
								p.errorf(pos, "限定嵌入字段 %q 后缺少类型名", qualified)
								break
							}
							p.nextFiltered()
							qualified = qualified + "." + mt.Value
						}
						decl.Fields = append(decl.Fields, &FieldDecl{Name: "", Type: qualified, Embedded: true, Pos: pos})
					}
				} else {
					// 嵌入字段：name 即类型名
					decl.Fields = append(decl.Fields, &FieldDecl{Name: "", Type: name, Embedded: true, Pos: pos})
				}
			} else {
				p.nextFiltered()
				continue
			}
		}
	}

	return decl
}

// parseConstDecl 解析常量声明。返回 Decl 接口以支持两种形式：
//   - 单常量：常量 名字 ＝ 值          → *ConstDecl
//   - 多常量块：常量 ( ... )          → *ConstBlockDecl
func (p *Parser) parseConstDecl() Decl {
	t := p.peekFiltered()
	pos := Pos{t.Line, t.Col}
	p.consumeKeyword("常量")

	// 多常量块：常量 ( 名字1 ＝ 值; 名字2 ＝ 值 )
	if p.consumeDelimiter("(") {
		block := &ConstBlockDecl{Pos: pos}
		for {
			nt := p.peekFiltered()
			if nt.Type == TokenEOF {
				p.errorf(Pos{nt.Line, nt.Col}, "多常量块缺少右括号 )")
				break
			}
			if nt.Type == TokenDelimiter && nt.Value == ")" {
				block.EndPos = Pos{nt.Line, nt.Col}
				p.nextFiltered()
				break
			}
			// 块内项：名字 ＝ 值
			item := p.parseConstItem()
			if item != nil {
				block.Items = append(block.Items, item)
			}
		}
		return block
	}

	// 单常量：常量 名字 ＝ 值
	decl := &ConstDecl{Pos: pos}
	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "常量缺少名字")
		return decl
	}
	decl.Name = nameTok.Value
	p.nextFiltered()

	// 等号 ＝ 或 =
	eqTok := p.peekFiltered()
	if eqTok.Type == TokenOperator && (eqTok.Value == "＝" || eqTok.Value == "=") {
		p.nextFiltered()
	} else {
		p.errorf(Pos{eqTok.Line, eqTok.Col}, "常量 %s 缺少等号", decl.Name)
		return decl
	}

	decl.Value = p.parseExpr()
	return decl
}

// parseConstItem 解析多常量块内的单项（名字 ＝ 值），不含"常量"关键字和括号
func (p *Parser) parseConstItem() *ConstDecl {
	t := p.peekFiltered()
	decl := &ConstDecl{Pos: Pos{t.Line, t.Col}}

	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "常量项缺少名字")
		p.nextFiltered()
		return nil
	}
	decl.Name = nameTok.Value
	p.nextFiltered()

	eqTok := p.peekFiltered()
	if eqTok.Type == TokenOperator && (eqTok.Value == "＝" || eqTok.Value == "=") {
		p.nextFiltered()
	} else {
		p.errorf(Pos{eqTok.Line, eqTok.Col}, "常量 %s 缺少等号", decl.Name)
		return decl
	}

	decl.Value = p.parseExpr()
	return decl
}

// parseEnumDecl 解析枚举声明（枚举 ... 结束枚束），对应 Go const 块 + iota
// 语法：
//   枚举
//       名字1 ＝ 表达式       // 首行带表达式（常含 序数）
//       名字2                 // 省略表达式，自动 iota +1
//   结束枚举
func (p *Parser) parseEnumDecl() *EnumDecl {
	t := p.peekFiltered()
	decl := &EnumDecl{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("枚举")

	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && t.Value == "结束枚举" {
			decl.EndPos = Pos{t.Line, t.Col}
			p.nextFiltered()
			break
		}
		// 枚举项名字
		nameTok := p.peekFiltered()
		if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
			p.nextFiltered()
			continue
		}
		name := nameTok.Value
		pos := Pos{nameTok.Line, nameTok.Col}
		p.nextFiltered()

		item := &EnumItem{Name: name, Pos: pos}
		// 可选 ＝ 表达式
		if nt := p.peekFiltered(); nt.Type == TokenOperator && (nt.Value == "＝" || nt.Value == "=") {
			p.nextFiltered()
			item.HasValue = true
			item.Value = p.parseExpr()
		}
		decl.Items = append(decl.Items, item)
	}

	return decl
}

// parseVarDecl 解析包级变量声明。返回 Decl 接口以支持两种形式：
//   - 单变量：变量 名字 类型 / 变量 名字 ＝ 值   → *VarDecl
//   - 多变量块：变量 ( ... )                   → *VarBlockDecl
func (p *Parser) parseVarDecl() Decl {
	t := p.peekFiltered()
	pos := Pos{t.Line, t.Col}
	p.consumeKeyword("变量")

	// 多变量块：变量 ( 名字1 类型; 名字2 ＝ 值 )
	if p.consumeDelimiter("(") {
		block := &VarBlockDecl{Pos: pos}
		for {
			nt := p.peekFiltered()
			if nt.Type == TokenEOF {
				p.errorf(Pos{nt.Line, nt.Col}, "多变量块缺少右括号 )")
				break
			}
			if nt.Type == TokenDelimiter && nt.Value == ")" {
				block.EndPos = Pos{nt.Line, nt.Col}
				p.nextFiltered()
				break
			}
			// 块内项：名字 类型 / 名字, 类型 / 名字 ＝ 值
			item := p.parseVarItem()
			if item != nil {
				block.Items = append(block.Items, item)
			}
		}
		return block
	}

	// 单变量：变量 名字 类型 / 变量 名字, 类型 / 变量 名字 ＝ 值
	decl := &VarDecl{Pos: pos}
	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "变量缺少名字")
		return decl
	}
	decl.Name = nameTok.Value
	p.nextFiltered()

	// 可能是 , 类型 或 类型 或 ＝ 值
	// 与 LocalVarDeclStmt 一致：支持 "变量 名字, 类型" 和 "变量 名字 类型" 两种形式
	if p.consumeDelimiter(",") {
		// 通用类型解析：支持 *Type / 通道 Type / 组合
		decl.Type = p.parseType()
	} else {
		nextTok := p.peekFiltered()
		if nextTok.Type == TokenOperator && (nextTok.Value == "＝" || nextTok.Value == "=") {
			p.nextFiltered()
			decl.Value = p.parseExpr()
		} else {
			// 通用类型解析：支持 *Type / 通道 Type / 组合
			decl.Type = p.parseType()
		}
	}

	return decl
}

// parseVarItem 解析多变量块内的单项，不含"变量"关键字和括号
// 支持三种形式：名字 类型 / 名字, 类型 / 名字 ＝ 值
func (p *Parser) parseVarItem() *VarDecl {
	t := p.peekFiltered()
	decl := &VarDecl{Pos: Pos{t.Line, t.Col}}

	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "变量项缺少名字")
		p.nextFiltered()
		return nil
	}
	decl.Name = nameTok.Value
	p.nextFiltered()

	if p.consumeDelimiter(",") {
		decl.Type = p.parseType()
	} else {
		nextTok := p.peekFiltered()
		if nextTok.Type == TokenOperator && (nextTok.Value == "＝" || nextTok.Value == "=") {
			p.nextFiltered()
			decl.Value = p.parseExpr()
		} else {
			decl.Type = p.parseType()
		}
	}

	return decl
}

// parseEmbedBlock 解析嵌入块（@嵌入 ... @结束）
func (p *Parser) parseEmbedBlock() *EmbedBlock {
	startTok := p.peekFiltered()
	decl := &EmbedBlock{Pos: Pos{startTok.Line, startTok.Col}}

	// 收集从 @嵌入 行到 @结束 行之间的所有原始内容
	startLine := startTok.Line
	p.nextFiltered() // 消费 @嵌入

	var contentLines []string
	currentLine := startLine + 1
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenEndEmbed {
			decl.EndPos = Pos{t.Line, t.Col}
			p.nextFiltered()
			break
		}
		// 提取当前 Token 所在行的原始内容
		if t.Line >= currentLine && t.Line-1 < len(p.lines) {
			contentLines = append(contentLines, p.lines[t.Line-1])
			// 跳到下一行
			currentLine = t.Line + 1
			// 跳过本行所有 Token
			for p.pos < len(p.tokens) && p.tokens[p.pos].Line == t.Line {
				p.pos++
			}
			continue
		}
		p.nextFiltered()
	}

	decl.Content = strings.Join(contentLines, "\n")
	return decl
}

// parseReturnTypes 解析函数/方法的返回类型声明
// 支持三种形式：
//   - 无返回值：) 后直接换行（跨行检测）→ 返回 nil
//   - 单返回值：整数型 → ["整数型"]
//   - 多返回值：(整数型, 文本型) → ["整数型", "文本型"]
//
// endKw 是结束关键字（"结束函数" / "结束方法"），用于判断无返回值场景
//
// 跨行检测：peekFiltered 跳过 NEWLINE，会把下一行的代码误判为返回类型
// 例如 `函数 主函数()\n\ta, b := f()`，peekFiltered 看到 `a` 会误判为返回类型
// 修复：用 raw 索引扫描，如果 ) 后（跳过 WS）直接是 NEWLINE/EOF，则无返回类型
func (p *Parser) parseReturnTypes(endKw string) []string {
	// 用临时索引判断是否跨行：如果 ) 后（跳过 WS 和注释）是 NEWLINE/EOF，则没有返回类型
	// 注意：必须跳过注释，否则 "func foo()\n// 注释\n语句" 会把注释后的语句误判为返回类型
	i := p.pos
	for i < len(p.tokens) && (p.tokens[i].Type == TokenWhitespace || p.tokens[i].Type == TokenComment) {
		i++
	}
	if i >= len(p.tokens) {
		return nil
	}
	raw := p.tokens[i]
	if raw.Type == TokenNewline || raw.Type == TokenEOF {
		return nil
	}

	// 同一行内，peekFiltered 获取返回类型或 endKw
	t := p.peekFiltered()
	// 无返回值（EOF）
	if t.Type == TokenEOF {
		return nil
	}

	// 多返回值 (t1, t2, ...)
	if t.Type == TokenDelimiter && t.Value == "(" {
		p.nextFiltered() // 消费 "("
		var types []string
		for {
			tt := p.peekFiltered()
			if tt.Type == TokenEOF {
				break
			}
			if tt.Type == TokenDelimiter && tt.Value == ")" {
				p.nextFiltered()
				break
			}
			// 跳过逗号
			if tt.Type == TokenDelimiter && tt.Value == "," {
				p.nextFiltered()
				continue
			}
			// 通用类型解析：支持 *Type / 通道 Type / 组合
			// 注意：parseType 不消费 token 时返回空串，需避免死循环
			before := p.pos
			ty := p.parseType()
			if ty != "" {
				types = append(types, ty)
			}
			if p.pos == before {
				// 未消费任何 token，跳过避免死循环
				p.nextFiltered()
			}
		}
		return types
	}

	// 单返回值（通用类型解析）
	// parseType 不识别 endKw（"结束函数"等 TokenKeyword）会返回空串，不消费 token
	ty := p.parseType()
	if ty != "" {
		return []string{ty}
	}

	return nil
}

// parseParamList 解析参数列表，支持 "名字 类型" 语法
// 特殊情况支持：
//   - 可变参数：name ...类型（如 items ...整数型）
//   - 通道类型：name 通道 类型（如 ch 通道 整数型）
// 示例：函数 f(a 整数型, b 文本型) 或 函数 f(items ...整数型) 或 函数 f(ch 通道 整数型)
func (p *Parser) parseParamList() []*ParamDecl {
	var params []*ParamDecl

	isNameTok := func(tok Token) bool {
		return tok.Type == TokenIdentifier || tok.Type == TokenChineseText || tok.Type == TokenKeywordType
	}

	// parseParam 解析单个参数
	parseParam := func() {
		nameTok := p.peekFiltered()
		if !isNameTok(nameTok) {
			p.errorf(Pos{nameTok.Line, nameTok.Col}, "参数缺少名字，实际为 %q", nameTok.Value)
			p.nextFiltered()
			return
		}
		paramName := nameTok.Value
		paramPos := Pos{nameTok.Line, nameTok.Col}
		p.nextFiltered()

		// 检查是否为 "..." 前缀（可变参数）
		variadic := false
		if nt := p.peekFiltered(); nt.Type == TokenOperator && nt.Value == "..." {
			variadic = true
			p.nextFiltered()
		}

		// 检查是否为 "通道" 关键字（通道类型：名字 通道 类型）
		if nt := p.peekFiltered(); nt.Type == TokenKeyword && nt.Value == "通道" {
			p.nextFiltered() // 消费 "通道"
			elemType := p.parseType()
			paramType := "通道 " + elemType
			params = append(params, &ParamDecl{Name: paramName, Type: paramType, Variadic: variadic, Pos: paramPos})
			return
		}

		// 普通类型（如果下一���是类型关键字，也要处理）
		nt := p.peekFiltered()
		if nt.Type == TokenKeywordType {
			p.nextFiltered()
			params = append(params, &ParamDecl{Name: paramName, Type: nt.Value, Variadic: variadic, Pos: paramPos})
			return
		}

		if nt.Type == TokenDelimiter && (nt.Value == ")" || nt.Value == ",") {
			params = append(params, &ParamDecl{Name: paramName, Type: "", Variadic: variadic, Pos: paramPos})
			return
		}

		paramType := p.parseType()
		params = append(params, &ParamDecl{Name: paramName, Type: paramType, Variadic: variadic, Pos: paramPos})
	}

	for {
		t := p.peekFiltered()
		if t.Type == TokenDelimiter && t.Value == ")" {
			break
		}
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenDelimiter && t.Value == "," {
			p.nextFiltered()
			continue
		}

		parseParam()

		p.consumeDelimiter(",")
	}
	return params
}

// parseBlockStmt 解析语句块直到遇到 endKw（结束xxx 关键字）
func (p *Parser) parseBlockStmt(endKw string, endPos *Pos) *BlockStmt {
	t := p.peekFiltered()
	block := &BlockStmt{Pos: Pos{t.Line, t.Col}}

	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && t.Value == endKw {
			if endPos != nil {
				*endPos = Pos{t.Line, t.Col}
			}
			p.nextFiltered()
			break
		}

		// 跳过函数体内的注释（与顶层解析循环行为一致）
		// 不跳过注释会导致 parseStmt 无法处理 TokenComment，触发死循环保护，
		// syncToNextStatement 可能跳过后续语句，导致函数体被解析为空。
		if t.Type == TokenComment {
			p.nextFiltered()
			continue
		}

		// 死循环保护：记录 parseStmt 前的位置，如果 parseStmt 没消费 token 则同步
		posBefore := p.pos
		stmt := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
		// 如果 parseStmt 没消费任何 token，说明遇到无法解析的内容，同步避免死循环
		if p.pos == posBefore {
			p.errorf(Pos{t.Line, t.Col}, "无法解析的语句（token %q）", t.Value)
			p.syncToNextStatement()
			// sync 后如果位置仍没变，强制消费一个 token 避免无限循环
			if p.pos == posBefore {
				p.nextFiltered()
			}
		}
	}

	block.EndPos = Pos{t.Line, t.Col}
	return block
}

// parseStmt 解析单条语句
func (p *Parser) parseStmt() Stmt {
	t := p.peekFiltered()

	// v58 关键修复：如果遇到 AST 解析器完全不支持的语句级关键字，快速失败避免死循环
	// 已知语句级关键字列表（switch 中有对应 case） + 表达式级关键字（需要继续解析）
	knownStmtKeywords := map[string]bool{
		"如果": true, "循环": true, "判断循环": true, "选择": true, "通道选择": true,
		"返回": true, "继续": true, "跳出": true, "标签": true, "跳转": true, "穿透": true,
		"延迟": true, "协程": true, "抛出": true, "局部变量": true, "情况": true, "默认": true,
		// 表达式级关键字（可能在表达式中出现，如匿名函数）
		"函数": true, "方法": true, "类型": true, "结构体": true, "接口": true,
		"枚举": true, "常量": true, "变量": true, "导入": true, "程序集": true,
		"通道": true, "新建": true, "映射": true, "数组": true,
		"真": true, "假": true, "空": true, "序数": true,
		"结束函数": true, "结束方法": true, "结束类型": true, "结束枚举": true,
		"结束如果": true, "结束循环": true, "结束判断循环": true, "结束选择": true, "结束通道选择": true,
		"否则": true, "否则如果": true,
	}
	if t.Type == TokenKeyword && !knownStmtKeywords[t.Value] {
		p.errorf(Pos{t.Line, t.Col}, "不支持的关键字 %q（AST 解析器未实现）", t.Value)
		p.nextFiltered()
		return &ExprStmt{X: &Ident{Name: t.Value, Pos: Pos{t.Line, t.Col}}}
	}

	if t.Type == TokenKeyword {
		switch t.Value {
		case "如果":
			return p.parseIfStmt()
		case "循环":
			return p.parseForStmt()
		case "判断循环":
			return p.parseWhileStmt()
		case "选择":
			return p.parseSwitchStmt()
		case "通道选择":
			return p.parseSelectStmt()
		case "返回":
			return p.parseReturnStmt()
		case "继续":
			stmt := &ContinueStmt{Pos: Pos{t.Line, t.Col}}
			p.nextFiltered()
			// 可选标签：继续 标签名
			if nt := p.peekFiltered(); nt.Type == TokenIdentifier || nt.Type == TokenChineseText {
				stmt.Label = nt.Value
				p.nextFiltered()
			}
			return stmt
		case "跳出":
			stmt := &BreakStmt{Pos: Pos{t.Line, t.Col}}
			p.nextFiltered()
			// 可选标签：跳出 标签名
			if nt := p.peekFiltered(); nt.Type == TokenIdentifier || nt.Type == TokenChineseText {
				stmt.Label = nt.Value
				p.nextFiltered()
			}
			return stmt
		case "标签":
			// 标签 名字（独占一行，下一语句是标签修饰的目标）
			return p.parseLabeledStmt()
		case "跳转":
			// 跳转 名字 → goto 名字
			stmt := &GotoStmt{Pos: Pos{t.Line, t.Col}}
			p.nextFiltered()
			labelTok := p.peekFiltered()
			if labelTok.Type != TokenIdentifier && labelTok.Type != TokenChineseText {
				p.errorf(Pos{labelTok.Line, labelTok.Col}, "跳转缺少标签名")
				return stmt
			}
			stmt.Label = labelTok.Value
			p.nextFiltered()
			return stmt
		case "穿透":
			// 穿透 → fallthrough（switch case 末尾穿透到下一个 case 体）
			stmt := &FallthroughStmt{Pos: Pos{t.Line, t.Col}}
			p.nextFiltered()
			return stmt
		case "延迟":
			// 延迟 xxx → defer xxx
			stmt := &DeferStmt{Pos: Pos{t.Line, t.Col}}
			p.nextFiltered()
			stmt.Call = p.parseExpr()
			return stmt
		case "协程":
			// 协程 xxx → go xxx
			stmt := &GoStmt{Pos: Pos{t.Line, t.Col}}
			p.nextFiltered()
			stmt.Call = p.parseExpr()
			return stmt
		case "抛出":
			// 抛出 expr → panic(expr)
			stmt := &PanicStmt{Pos: Pos{t.Line, t.Col}}
			p.nextFiltered()
			stmt.X = p.parseExpr()
			return stmt
		case "局部变量":
			return p.parseLocalVarDeclStmt()
		case "通道":
			// 通道变量声明：通道 整数型 ch ＝ 新建 通道 整数型
			// 转译为：var ch chan int = make(chan int)
			return p.parseChanVarDeclStmt()
		case "情况":
			return p.parseCaseClause(nil) // case 在 switch 中处理
		case "默认":
			return p.parseCaseClause(nil)
		}
	}

	// 否则是表达式语句或赋值
	return p.parseExprOrAssignStmt()
}

// parseIfStmt 解析如果语句
// 支持"否则如果 cond"简写（else if），解析为嵌套 IfStmt 放入 Else 块，
// gen.go 会识别此结构生成 "} else if ... {" 而非 "} else { if ... }"
func (p *Parser) parseIfStmt() *IfStmt {
	t := p.peekFiltered()
	stmt := &IfStmt{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("如果")

	// 条件表达式（可能带括号）
	if p.consumeDelimiter("(") {
		stmt.Cond = p.parseExpr()
		p.consumeDelimiter(")")
	} else {
		stmt.Cond = p.parseExpr()
	}

	// Then 块
	stmt.Then = p.parseUntilElseOrEnd("结束如果")

	// Else 块（可选，识别"否则如果"简写）
	stmt.Else = p.parseElseClause()

	return stmt
}

// parseElseClause 解析 Else 块，识别"否则如果"简写
// - "否则如果" → 递归解析为嵌套 IfStmt，放入 Else 块作为唯一语句
// - "否则" → 解析为普通 BlockStmt 直到"结束如果"
// - "结束如果" → 消费并返回 nil（无 else）
func (p *Parser) parseElseClause() *BlockStmt {
	elseTok := p.peekFiltered()
	if elseTok.Type != TokenKeyword {
		return nil
	}
	switch elseTok.Value {
	case "否则如果":
		// 否则如果简写：递归解析嵌套 IfStmt
		nestedPos := Pos{elseTok.Line, elseTok.Col}
		p.nextFiltered() // 消费"否则如果"
		nestedIf := &IfStmt{Pos: nestedPos}
		// 解析 cond（可能带括号）
		if p.consumeDelimiter("(") {
			nestedIf.Cond = p.parseExpr()
			p.consumeDelimiter(")")
		} else {
			nestedIf.Cond = p.parseExpr()
		}
		// Then 块
		nestedIf.Then = p.parseUntilElseOrEnd("结束如果")
		// 递归解析 Else（可能是另一个"否则如果"或"否则"）
		nestedIf.Else = p.parseElseClause()
		return &BlockStmt{Pos: nestedPos, Stmts: []Stmt{nestedIf}}
	case "否则":
		p.nextFiltered() // 消费"否则"
		return p.parseBlockStmt("结束如果", nil)
	case "结束如果":
		p.nextFiltered() // 消费"结束如果"
		return nil
	}
	return nil
}

// parseUntilElseOrEnd 解析直到遇到"否则"/"否则如果"/endKw
func (p *Parser) parseUntilElseOrEnd(endKw string) *BlockStmt {
	t := p.peekFiltered()
	block := &BlockStmt{Pos: Pos{t.Line, t.Col}}
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && (t.Value == "否则" || t.Value == "否则如果" || t.Value == endKw) {
			break
		}
		stmt := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
	}
	return block
}

// parseForStmt 解析循环语句
// 支持三种形式：
//  1. 循环 k, v ＝ 范围 x   → RangeStmt（双变量 range）
//  2. 循环 v ＝ 范围 x      → RangeStmt（单变量 range，Key 为空）
//  3. 循环 init; cond; post → ForStmt（三段式 for，对应 Go 三段式 for）
//  4. 循环 (init; cond; post) → ForStmt（带括号的三段式，糖衣）
//  5. 循环 cond             → ForStmt（简化 for cond）
//  6. 循环 (cond)           → ForStmt（带括号的 cond）
//
// 识别策略：peek 后如果是 ident 列表 + "＝"/"=" + "范围"，走 RangeStmt；
// 否则按 ForStmt 解析：先解析一个 simple stmt（init 或 cond 表达式语句），
// 若下一个 token 是 ";" 则识别为三段式，分别填充 Init/Cond/Post；
// 否则把 simple stmt 提取为 Cond 表达式（若是 ExprStmt 则取 X，否则回滚重新 parseExpr）。
func (p *Parser) parseForStmt() Stmt {
	t := p.peekFiltered()
	startPos := Pos{t.Line, t.Col}
	p.consumeKeyword("循环")

	// 尝试识别 range 形式
	if p.tryPeekRangePattern() {
		return p.parseRangeStmt(startPos)
	}

	stmt := &ForStmt{Pos: startPos}

	// 带 () 的形式：可能是 (cond) 或 (init; cond; post)
	hasParen := p.consumeDelimiter("(")

	// 解析第一段（可能是 init 或 cond 表达式）
	// 若下一个 token 是块体结束/控制流关键字（如 跳出/继续/结束循环），
	// 说明是无限循环形式（循环 ... 结束循环），跳过 first 解析避免 parseExpr 报错
	var first Stmt
	if nt := p.peekFiltered(); !isBlockStartKeyword(nt) {
		first = p.parseExprOrAssignStmt()
	}

	// 检测三段式：第一个 stmt 后紧跟 ";"
	if first != nil && p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == ";" {
		// 三段式：init; cond; post
		stmt.Init = first
		p.nextFiltered() // 消费第一个 ";"

		// cond（可为空，紧跟第二个 ";"）
		if nt := p.peekFiltered(); !(nt.Type == TokenDelimiter && nt.Value == ";") {
			stmt.Cond = p.parseExpr()
		}

		// 消费第二个 ";"
		if p.consumeDelimiter(";") {
			// post（可为空，紧跟 ")" 或语句结束）
			if nt := p.peekFiltered(); !(nt.Type == TokenDelimiter && nt.Value == ")") {
				if post := p.parseExprOrAssignStmt(); post != nil {
					stmt.Post = post
				}
			}
		}
	} else {
		// 单 cond 形式：把 first 提取为 cond 表达式
		// 仅 ExprStmt 能安全提取表达式；其他类型（AssignStmt/IncDecStmt/MultiAssignStmt）
		// 不应作为 cond，那是用户语法错误，这里容错：cond 为 nil，无限循环
		if e, ok := first.(*ExprStmt); ok {
			stmt.Cond = e.X
		}
		// first 为 nil 时 cond 也为 nil（无限循环 for { ... }）
	}

	if hasParen {
		p.consumeDelimiter(")")
	}

	stmt.Body = p.parseBlockStmt("结束循环", nil)
	return stmt
}

// isBlockStartKeyword 判断 token 是否为"块体内/控制流"关键字
// 这些关键字不会作为 for 的 init/cond 表达式开头出现
// 遇到它们应跳过 first 解析，避免 parseExpr 报"表达式位置不允许关键字"错误
// 典型场景：无限循环 `循环 ... 结束循环`，循环体首条语句是 `跳出`/`继续`/`返回` 等
func isBlockStartKeyword(t Token) bool {
	if t.Type != TokenKeyword {
		return false
	}
	switch t.Value {
	case "跳出", "继续", "返回", "跳转", "穿透",
		"结束循环", "结束函数", "结束方法", "结束如果", "结束判断循环", "结束选择", "结束通道选择", "结束类型", "结束枚举",
		"否则", "情况", "默认":
		return true
	}
	return false
}

// tryPeekQualifiedStructLit 前瞻检测 `ident . ident (. ident)* {` 模式
// 用于识别限定类型 struct literal：包名.T{...} 或 包名.子包.T{...}
// 不消费任何 token，仅用独立索引扫描
// 返回：(完整限定类型名, 段数)。段数为 0 表示不匹配。
// 严格匹配：最后必须是 `ident {`，避免与成员访问 `包名.字段` 冲突
func (p *Parser) tryPeekQualifiedStructLit(firstName string) (string, int) {
	i := p.pos
	scanFiltered := func() Token {
		for i < len(p.tokens) {
			t := p.tokens[i]
			i++
			if t.Type != TokenWhitespace && t.Type != TokenNewline {
				return t
			}
		}
		return Token{Type: TokenEOF}
	}

	// 已经消费了 firstName，从当前位置开始扫描
	// 期望模式：. ident (. ident)* {
	qualifiedName := firstName
	segCount := 0
	for {
		// 期望 .
		t := scanFiltered()
		if t.Type != TokenDelimiter || t.Value != "." {
			return "", 0
		}
		// 期望 ident
		t = scanFiltered()
		if t.Type != TokenIdentifier && t.Type != TokenChineseText && t.Type != TokenKeywordType {
			return "", 0
		}
		qualifiedName = qualifiedName + "." + t.Value
		segCount++
		// 期望 { 或 . （继续限定）
		t = scanFiltered()
		if t.Type == TokenDelimiter && t.Value == "{" {
			return qualifiedName, segCount
		}
		if t.Type == TokenDelimiter && t.Value == "." {
			// 继续前瞻下一段，回退一个 token 让循环重新消费 .
			i--
			continue
		}
		return "", 0
	}
}

// tryPeekRangePattern 探测当前 token 流是否符合 "ident (, ident)? (＝|=) 范围 expr" 模式
// 不消费 token，仅做前瞻判断（用独立索引，不修改 p.pos）
func (p *Parser) tryPeekRangePattern() bool {
	// 用独立索引扫描 token 流，跳过空白和换行
	i := p.pos
	scanFiltered := func() Token {
		for i < len(p.tokens) {
			t := p.tokens[i]
			i++
			if t.Type != TokenWhitespace && t.Type != TokenNewline {
				return t
			}
		}
		return Token{Type: TokenEOF}
	}

	// 最多前瞻 20 个 token 寻找"范围"关键字
	for j := 0; j < 20; j++ {
		t := scanFiltered()
		if t.Type == TokenEOF {
			return false
		}
		if t.Type == TokenKeyword && t.Value == "范围" {
			return true
		}
		// 遇到语句结束/新语句关键字，肯定不是 range
		if t.Type == TokenKeyword && (t.Value == "结束循环" || t.Value == "循环" || t.Value == "结束如果" || t.Value == "结束函数" || t.Value == "结束方法") {
			return false
		}
		// 遇到右括号/分号等，可能是 for(cond) 形式，但里面不应该有"范围"
		// 继续扫描
	}
	return false
}

// parseRangeStmt 解析 range 循环（已确认是 range 形式）
// 形式：k, v ＝ 范围 x 或 v ＝ 范围 x
// 左侧变量列表用 token 级别解析（避免 parseExpr 贪婪吞掉"范围"关键字）
func (p *Parser) parseRangeStmt(startPos Pos) *RangeStmt {
	stmt := &RangeStmt{Pos: startPos}

	// 用 token 级别解析左侧变量列表：ident (, ident)?
	// 变量名允许 TokenIdentifier（英文）和 TokenChineseText（中文），与 parseFuncDecl 一致
	var vars []string
	t := p.peekFiltered()
	if t.Type != TokenIdentifier && t.Type != TokenChineseText {
		p.errorf(startPos, "range 循环左侧应为变量名, 实际 %q", t.Value)
		return stmt
	}
	vars = append(vars, t.Value)
	p.nextFiltered()

	// 检查逗号
	if p.consumeDelimiter(",") {
		t2 := p.peekFiltered()
		if t2.Type != TokenIdentifier && t2.Type != TokenChineseText {
			p.errorf(startPos, "range 循环第二个变量应为变量名, 实际 %q", t2.Value)
			return stmt
		}
		vars = append(vars, t2.Value)
		p.nextFiltered()
	}

	// 消费 "＝" 或 "="
	if tok := p.peekFiltered(); tok.Type == TokenOperator && (tok.Value == "＝" || tok.Value == "=") {
		stmt.Tok = tok.Value
		p.nextFiltered()
	}

	// 消费 "范围" 关键字
	p.consumeKeyword("范围")

	// 解析 range 表达式
	stmt.X = p.parseExpr()

	// 赋值 Key/Value
	switch len(vars) {
	case 1:
		stmt.Key = ""
		stmt.Value = vars[0]
	case 2:
		stmt.Key = vars[0]
		stmt.Value = vars[1]
	}

	// 循环体
	stmt.Body = p.parseBlockStmt("结束循环", nil)
	return stmt
}

// parseWhileStmt 解析判断循环
func (p *Parser) parseWhileStmt() *WhileStmt {
	t := p.peekFiltered()
	stmt := &WhileStmt{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("判断循环")

	if p.consumeDelimiter("(") {
		stmt.Cond = p.parseExpr()
		p.consumeDelimiter(")")
	} else {
		stmt.Cond = p.parseExpr()
	}

	stmt.Body = p.parseBlockStmt("结束判断循环", nil)
	return stmt
}

// parseSwitchStmt 解析选择语句
// 支持三种形式（v53 扩展）：
//   1. 选择 expr ... 结束选择            → switch expr { ... }
//   2. 选择 { ... 结束选择               → switch { ... }（X 为 nil）
//   3. 选择 x ＝ y.(类型) ... 结束选择   → switch x := y.(type) { ... }（type switch，TypeVar 非空）
func (p *Parser) parseSwitchStmt() *SwitchStmt {
	t := p.peekFiltered()
	stmt := &SwitchStmt{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("选择")

	// 检测 type switch 形式：选择 x ＝ y.(类型) 或 选择 x := y.(type)
	// 前瞻：ident/chineseText 后跟 ＝ / := 则为 type switch
	if p.isTypeSwitchPrefix() {
		varTok := p.peekFiltered()
		stmt.TypeVar = varTok.Value
		p.nextFiltered() // 消费变量名
		opTok := p.peekFiltered()
		p.nextFiltered() // 消费 ＝ / :=
		_ = opTok
		// 解析被断言的表达式（应为 TypeAssertExpr{Type: ""}）
		stmt.X = p.parseExpr()
		if tae, ok := stmt.X.(*TypeAssertExpr); !ok || tae.Type != "" {
			p.errorf(stmt.Pos, "类型选择 %q 后必须是 .(类型) 形式", stmt.TypeVar)
		}
	} else if p.consumeDelimiter("(") {
		stmt.X = p.parseExpr()
		p.consumeDelimiter(")")
	} else if p.peekFiltered().Type == TokenKeyword && p.peekFiltered().Value == "结束选择" {
		// 选择 ... 结束选择 直接跟结束，X 为 nil（switch { ... }）
		// 但仍可能有 case，先不消费 结束选择，由下面的循环处理
		// 这里不解析 X
	} else if p.peekFiltered().Type == TokenKeyword && p.peekFiltered().Value == "情况" {
		// 选择 情况 ... 形式：X 为 nil（switch { case ... }）
	} else {
		stmt.X = p.parseExpr()
	}

	// 解析 case 列表
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && t.Value == "结束选择" {
			p.nextFiltered()
			break
		}
		if t.Type == TokenKeyword && t.Value == "情况" {
			caseClause := p.parseCaseClause(stmt)
			if caseClause != nil {
				stmt.Cases = append(stmt.Cases, caseClause)
			}
		} else if t.Type == TokenKeyword && t.Value == "默认" {
			caseClause := p.parseDefaultClause(stmt)
			if caseClause != nil {
				stmt.Cases = append(stmt.Cases, caseClause)
			}
		} else {
			p.nextFiltered()
		}
	}

	return stmt
}

// isTypeSwitchPrefix 前瞻检测 type switch 形式
// 匹配模式：ident/chineseText 后跟 ＝（全角）/ =（半角）/ :=（短声明）
// 用于 "选择 x ＝ y.(类型)" 形式识别
func (p *Parser) isTypeSwitchPrefix() bool {
	i := p.pos
	// 跳过空白/换行
	for i < len(p.tokens) && (p.tokens[i].Type == TokenWhitespace || p.tokens[i].Type == TokenNewline) {
		i++
	}
	if i >= len(p.tokens) {
		return false
	}
	t1 := p.tokens[i]
	// 变量名：ident / 中文文本（不能是关键字，避免误判 选择 选择 这种）
	if t1.Type != TokenIdentifier && t1.Type != TokenChineseText {
		return false
	}
	i++
	for i < len(p.tokens) && (p.tokens[i].Type == TokenWhitespace || p.tokens[i].Type == TokenNewline) {
		i++
	}
	if i >= len(p.tokens) {
		return false
	}
	t2 := p.tokens[i]
	// ＝ / = / :=
	if t2.Type == TokenOperator && (t2.Value == "＝" || t2.Value == "=" || t2.Value == ":=") {
		return true
	}
	return false
}

// parseSelectStmt 解析通道选择语句
// 语法：
//   通道选择
//       情况 v := <-ch1:
//           ...
//       情况 ch2 <- y:
//           ...
//       默认:
//           ...
//   结束通道选择
//
// 对应 Go 的 select { case ... : ... default: ... }
func (p *Parser) parseSelectStmt() *SelectStmt {
	t := p.peekFiltered()
	stmt := &SelectStmt{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("通道选择")

	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && t.Value == "结束通道选择" {
			p.nextFiltered()
			break
		}
		if t.Type == TokenKeyword && t.Value == "情况" {
			cc := p.parseCommClause()
			if cc != nil {
				stmt.Cases = append(stmt.Cases, cc)
			}
		} else if t.Type == TokenKeyword && t.Value == "默认" {
			cc := p.parseDefaultCommClause()
			if cc != nil {
				stmt.Cases = append(stmt.Cases, cc)
			}
		} else {
			p.nextFiltered()
		}
	}

	return stmt
}

// parseCommClause 解析 select 的 case 通信分支
// 语法：情况 <comm>: body
// comm 可以是：
//   - v := <-ch      （短声明接收，AssignStmt Op=":="）
//   - v, ok := <-ch  （多变量短声明接收，MultiAssignStmt Op=":="）
//   - <-ch           （接收表达式，ExprStmt(ChanExpr)）
//   - ch <- v        （发送表达式，ExprStmt(ChanExpr)）
func (p *Parser) parseCommClause() *CommClause {
	t := p.peekFiltered()
	cc := &CommClause{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("情况")

	// 解析通信语句：尝试按表达式/赋值语句解析
	// parseExprOrAssignStmt 能识别 v := <-ch（短声明）、ch <- v（发送）、<-ch（接收）
	cc.Comm = p.parseExprOrAssignStmt()
	p.consumeDelimiter(":")

	// 解析 case body
	cc.Body = &BlockStmt{Pos: Pos{t.Line, t.Col}}
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && (t.Value == "情况" || t.Value == "默认" || t.Value == "结束通道选择") {
			break
		}
		stmt := p.parseStmt()
		if stmt != nil {
			cc.Body.Stmts = append(cc.Body.Stmts, stmt)
		}
	}

	return cc
}

// parseDefaultCommClause 解析 select 的 default 分支
func (p *Parser) parseDefaultCommClause() *CommClause {
	t := p.peekFiltered()
	cc := &CommClause{Pos: Pos{t.Line, t.Col}} // Comm 为 nil 表示默认
	p.consumeKeyword("默认")
	p.consumeDelimiter(":")

	cc.Body = &BlockStmt{Pos: Pos{t.Line, t.Col}}
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && (t.Value == "情况" || t.Value == "结束通道选择") {
			break
		}
		stmt := p.parseStmt()
		if stmt != nil {
			cc.Body.Stmts = append(cc.Body.Stmts, stmt)
		}
	}

	return cc
}

// parseCaseClause 解析情况分支
// typeSwitch 为 true 时（type switch 的 case），值用 parseType 解析类型名而非 parseExpr
// 支持多值：情况 v1, v2, v3: 或 情况 T1, T2, T3:
func (p *Parser) parseCaseClause(owner *SwitchStmt) *CaseClause {
	t := p.peekFiltered()
	cc := &CaseClause{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("情况")

	typeSwitch := owner != nil && owner.TypeVar != ""

	if typeSwitch {
		// type switch case：值为类型名（用 parseType 解析）
		// 情况 整数型: / 情况 *整数型: / 情况 T1, T2, T3:
		for {
			typeName := p.parseType()
			if typeName == "" {
				p.errorf(Pos{t.Line, t.Col}, "类型选择情况分支缺少类型名")
				break
			}
			cc.Values = append(cc.Values, &Ident{Name: typeName, Pos: Pos{t.Line, t.Col}})
			if !p.consumeDelimiter(",") {
				break
			}
		}
	} else {
		// 普通 switch case：值为表达式，支持多值 v1, v2, v3
		for {
			cc.Values = append(cc.Values, p.parseExpr())
			if !p.consumeDelimiter(",") {
				break
			}
		}
	}
	p.consumeDelimiter(":")

	// 解析 case body（直到下一个 情况/默认/结束选择）
	cc.Body = &BlockStmt{Pos: Pos{t.Line, t.Col}}
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && (t.Value == "情况" || t.Value == "默认" || t.Value == "结束选择") {
			break
		}
		stmt := p.parseStmt()
		if stmt != nil {
			cc.Body.Stmts = append(cc.Body.Stmts, stmt)
		}
	}

	return cc
}

// parseDefaultClause 解析默认分支
func (p *Parser) parseDefaultClause(_ *SwitchStmt) *CaseClause {
	t := p.peekFiltered()
	cc := &CaseClause{Pos: Pos{t.Line, t.Col}} // Values 为 nil 表示默认
	p.consumeKeyword("默认")
	p.consumeDelimiter(":")

	cc.Body = &BlockStmt{Pos: Pos{t.Line, t.Col}}
	for {
		t := p.peekFiltered()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && (t.Value == "情况" || t.Value == "结束选择") {
			break
		}
		stmt := p.parseStmt()
		if stmt != nil {
			cc.Body.Stmts = append(cc.Body.Stmts, stmt)
		}
	}

	return cc
}

// parseReturnStmt 解析返回语句（支持多返回值：返回 a, b, c）
func (p *Parser) parseReturnStmt() *ReturnStmt {
	t := p.peekFiltered()
	stmt := &ReturnStmt{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("返回")

	// 检查是否有返回值（下一个 Token 是 EOF 表示无返回值）
	// 注意：空/真/假 是 TokenKeyword 但作为字面量可以作为返回值，不能短路
	next := p.peekFiltered()
	if next.Type == TokenEOF {
		return stmt
	}
	if next.Type == TokenKeyword {
		switch next.Value {
		case "空", "真", "假":
			// 字面量，继续解析表达式
		default:
			// 其他关键字（如结束函数/否则等）表示无返回值
			return stmt
		}
	}

	// 循环解析逗号分隔的多个返回值
	for {
		e := p.parseExpr()
		if e == nil {
			break
		}
		stmt.Values = append(stmt.Values, e)
		if !p.consumeDelimiter(",") {
			break
		}
	}

	return stmt
}

// parseLabeledStmt 解析标签语句（标签 名字 独占一行，下一语句是标签修饰的目标）
// 语法：
//   标签 外层
//   循环 ...
//       循环 ...
//           跳出 外层      // break 外层
//       结束循环
//   结束循环
// 转译为 Go：
//   外层:
//   for ... {
//       for ... {
//           break 外层
//       }
//   }
func (p *Parser) parseLabeledStmt() *LabeledStmt {
	t := p.peekFiltered()
	stmt := &LabeledStmt{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("标签")

	// 标签名
	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "标签缺少名字")
		return stmt
	}
	stmt.Label = nameTok.Value
	p.nextFiltered()

	// 检查下一 token：若是块结束/EOF/控制流关键字，则标签独立存在（用于 goto 跳转目标）
	// 否则解析下一语句作为标签修饰的目标（v48 行为，用于 break/continue）
	if nt := p.peekFiltered(); nt.Type == TokenEOF || isBlockStartKeyword(nt) {
		stmt.Stmt = nil
		return stmt
	}

	// 解析下一语句作为标签修饰的目标
	stmt.Stmt = p.parseStmt()
	return stmt
}

// parseType 解析类型声明（通用，支持指针和通道组合）
// 支持：
//   - 基础类型：整数型 / 文本型 / MyType
//   - 指针类型：*Type（递归，如 *整数型 → *int）
//   - 通道类型：通道 Type（递归，如 通道 整数型 → chan int）
//   - 组合：*通道 整数型 → *chan int
//
// 返回类型字符串（中文形式，由 mapType 转换为 Go 类型）
// 如果无法识别，消费 Token 并报错（避免调用方死循环），返回空字符串
func (p *Parser) parseType() string {
	// 指针前缀（支持 ** / *** 等连续 * 序列，lexer 会把 ** 切成单一 Token）
	if t := p.peekFiltered(); t.Type == TokenOperator && t.Value != "" && strings.Trim(t.Value, "*") == "" {
		p.nextFiltered()
		return t.Value + p.parseType()
	}
	// 通道类型：通道 <Type>
	if t := p.peekFiltered(); t.Type == TokenKeyword && t.Value == "通道" {
		p.nextFiltered()
		return "通道 " + p.parseType()
	}
	// 基础类型 / 标识符 / 中文文本
	t := p.peekFiltered()
	if t.Type == TokenKeywordType || t.Type == TokenIdentifier || t.Type == TokenChineseText {
		p.nextFiltered()
		typ := t.Value
		// 限定类型：包名.T（支持多段 .T，如 包名.子包.T）
		// 循环 peek 是否是 `.`，是则消费 `.` 和后续标识符，组合成限定类型名
		for p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "." {
			p.nextFiltered() // 消费 .
			nt := p.peekFiltered()
			if nt.Type != TokenIdentifier && nt.Type != TokenChineseText && nt.Type != TokenKeywordType {
				p.errorf(Pos{nt.Line, nt.Col}, "限定类型 %q 后缺少类型名", typ)
				return typ
			}
			p.nextFiltered()
			typ = typ + "." + nt.Value
		}
		return typ
	}
	// 不匹配：消费 Token 避免调用方死循环（如 parseLocalVarDeclStmt 不检查返回值）
	if t.Type != TokenEOF {
		p.errorf(Pos{t.Line, t.Col}, "类型声明位置出现 %q，期望 *Type / 通道 Type / 基础类型", t.Value)
		p.nextFiltered()
	}
	return ""
}

// parseNewExpr 解析新建(T) 表达式 → new(T)
// 语法：新建 ( 类型 )，类型支持 *Type / 通道 Type / 组合（复用 parseType）
func (p *Parser) parseNewExpr() *NewExpr {
	t := p.peekFiltered()
	expr := &NewExpr{Pos: Pos{t.Line, t.Col}}
	p.nextFiltered() // 消费 新建

	if !p.consumeDelimiter("(") {
		p.errorf(expr.Pos, "新建缺少左括号")
		return nil
	}
	ty := p.parseType()
	if ty == "" {
		p.errorf(expr.Pos, "新建缺少类型参数")
		return nil
	}
	expr.Type = ty
	if !p.consumeDelimiter(")") {
		p.errorf(expr.Pos, "新建缺少右括号")
		return nil
	}

	// new(T) 后可能跟成员访问 / 调用，交给 parseSuffix 处理
	// 但 parseNewExpr 已消费完参数，这里返回后由调用方 parsePrimary 接管后缀
	// 由于 parsePrimary 的 case "新建" 直接 return p.parseNewExpr()，
	// 需要在此调用 parseSuffix 以支持 new(T).Field / new(T)[0] 等
	return expr
}

// parseFuncLit 解析匿名函数字面量
// 语法：函数 (参数) 返回类型 ... 结束函数
// 与 parseFuncDecl 区别：无函数名，作为表达式使用
func (p *Parser) parseFuncLit() *FuncLit {
	t := p.peekFiltered()
	fl := &FuncLit{Pos: Pos{t.Line, t.Col}}
	p.nextFiltered() // 消费 函数

	// 参数列表（注意：匿名函数无函数名，紧跟 (）
	if !p.consumeDelimiter("(") {
		p.errorf(fl.Pos, "匿名函数缺少左括号")
		return nil
	}
	fl.Params = p.parseParamList()
	p.consumeDelimiter(")")

	// 返回类型（可选）
	fl.ReturnTypes = p.parseReturnTypes("结束函数")

	// 函数体
	fl.Body = p.parseBlockStmt("结束函数", &fl.EndPos)
	return fl
}

// parseLocalVarDeclStmt 解析局部变量声明
func (p *Parser) parseLocalVarDeclStmt() *LocalVarDeclStmt {
	t := p.peekFiltered()
	stmt := &LocalVarDeclStmt{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("局部变量")

	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "局部变量缺少名字")
		return stmt
	}
	stmt.Name = nameTok.Value
	p.nextFiltered()

	// 可能是 , 类型 或 类型 或 ＝ 值
	// 与 parseVarDecl 保持一致：支持 `局部变量 i, 整数型` 和 `局部变量 i 整数型` 两种语法
	if p.consumeDelimiter(",") {
		// 通用类型解析：支持 *Type / 通道 Type / 组合
		stmt.Type = p.parseType()
	} else {
		nextTok := p.peekFiltered()
		if nextTok.Type == TokenOperator && (nextTok.Value == "＝" || nextTok.Value == "=") {
			p.nextFiltered()
			stmt.Value = p.parseExpr()
		} else {
			// 名字后直接跟类型（无逗号）：局部变量 i 整数型
			stmt.Type = p.parseType()
		}
	}

	return stmt
}

// parseChanVarDeclStmt 解析通道变量声明
// 语法：通道 元素类型 变量名 ＝ 新建 通道 元素类型
//   例：通道 整数型 ch ＝ 新建 通道 整数型
//   转译：var ch chan int = make(chan int)
// 也支持无初始化值形式：通道 整数型 ch → var ch chan int
// 元素类型由 parseType 解析（支持 *Type / 通道 Type 等组合）
func (p *Parser) parseChanVarDeclStmt() *LocalVarDeclStmt {
	t := p.peekFiltered()
	stmt := &LocalVarDeclStmt{Pos: Pos{t.Line, t.Col}}
	p.consumeKeyword("通道")

	// 元素类型（parseType 在遇到 "通道" 时会递归，但这里 "通道" 已被消费，
	// 所以会直接解析元素类型如 "整数型"）
	elemType := p.parseType()
	if elemType == "" {
		p.errorf(stmt.Pos, "通道变量声明缺少元素类型")
		return stmt
	}
	// 组合为 "通道 元素类型"，与 mapType 配合生成 "chan T"
	stmt.Type = "通道 " + elemType

	// 变量名
	nameTok := p.peekFiltered()
	if nameTok.Type != TokenIdentifier && nameTok.Type != TokenChineseText {
		p.errorf(Pos{nameTok.Line, nameTok.Col}, "通道变量声明缺少变量名")
		return stmt
	}
	stmt.Name = nameTok.Value
	p.nextFiltered()

	// 可选初始化：＝ 新建 通道 元素类型
	if nt := p.peekFiltered(); nt.Type == TokenOperator && (nt.Value == "＝" || nt.Value == "=") {
		p.nextFiltered() // 消费 ＝
		// 期望 "新建 通道 元素类型" 或 "新建 (通道 元素类型)"
		newTok := p.peekFiltered()
		if newTok.Type == TokenKeyword && newTok.Value == "新建" {
			p.nextFiltered() // 消费 新建
			// 可选括号
			hasParen := p.consumeDelimiter("(")
			chanType := p.parseType()
			if chanType == "" {
				p.errorf(stmt.Pos, "新建通道缺少类型参数")
				return stmt
			}
			if hasParen {
				p.consumeDelimiter(")")
			}
			// 用 ChanMakeExpr 节点，writeExpr 时生成 make(chan mapType(ElemType))
			stmt.Value = &ChanMakeExpr{
				ElemType: elemType,
				Pos:      Pos{newTok.Line, newTok.Col},
			}
		} else {
			// 其他初始化表达式，按普通表达式解析
			stmt.Value = p.parseExpr()
		}
	}

	return stmt
}

// parseExprOrAssignStmt 解析表达式语句或赋值语句
// 识别四种形式：
//   - expr ++ / expr --  → IncDecStmt（后缀自增/自减）
//   - lhs ＝/=/+=/-=/*=//= rhs → AssignStmt（单变量赋值）
//   - a, b ＝/=/:= rhs-list → MultiAssignStmt（多变量赋值/短声明）
//   - expr              → ExprStmt
func (p *Parser) parseExprOrAssignStmt() Stmt {
	t := p.peekFiltered()
	startPos := Pos{t.Line, t.Col}

	// 解析一个表达式
	expr := p.parseExpr()
	if expr == nil {
		return nil
	}

	next := p.peekFiltered()

	// 通道发送：ch <- value（二元形式，类似赋值语句）
	if next.Type == TokenOperator && next.Value == "<-" {
		p.nextFiltered() // 消费 <-
		val := p.parseExpr()
		return &ExprStmt{X: &ChanExpr{Op: "<-", Chan: expr, Value: val, Pos: startPos}, Pos: startPos}
	}

	// 检查是否是自增/自减（后缀 ++ / --）
	if next.Type == TokenOperator && (next.Value == "++" || next.Value == "--") {
		op := next.Value
		p.nextFiltered()
		return &IncDecStmt{X: expr, Tok: op, Pos: startPos}
	}

	// 检查是否是全角自增自减兼容
	if next.Type == TokenOperator && (next.Value == "＋＋" || next.Value == "－－") {
		// 全角自增自减（不常见但兼容）
		op := "++"
		if next.Value == "－－" {
			op = "--"
		}
		p.nextFiltered()
		return &IncDecStmt{X: expr, Tok: op, Pos: startPos}
	}

	// 多变量赋值/短声明：lhs1, lhs2, ... op rhs1, rhs2, ...
	// 检测：左侧第一个 expr 后紧跟逗号
	if next.Type == TokenDelimiter && next.Value == "," {
		lhs := []Expr{expr}
		// 收集剩余的左侧表达式
		for p.consumeDelimiter(",") {
			e := p.parseExpr()
			if e == nil {
				break
			}
			lhs = append(lhs, e)
		}

		// 期望赋值运算符
		opTok := p.peekFiltered()
		if opTok.Type != TokenOperator || (opTok.Value != "＝" && opTok.Value != "=" && opTok.Value != ":=") {
			p.errorf(Pos{opTok.Line, opTok.Col}, "多变量赋值缺少等号，实际为 %q", opTok.Value)
			return &ExprStmt{X: expr, Pos: startPos}
		}
		op := opTok.Value
		p.nextFiltered()

		// 解析右侧表达式列表
		var rhs []Expr
		first := p.parseExpr()
		if first != nil {
			rhs = append(rhs, first)
			for p.consumeDelimiter(",") {
				e := p.parseExpr()
				if e == nil {
					break
				}
				rhs = append(rhs, e)
			}
		}

		return &MultiAssignStmt{Lhs: lhs, Op: op, Rhs: rhs, Pos: startPos}
	}

	// 单变量赋值/短声明
	// 支持 ＝/= （普通赋值）、:= （短声明）、+=/-=/*=//= （复合赋值）
	if next.Type == TokenOperator && (next.Value == "＝" || next.Value == "=" || next.Value == ":=" || next.Value == "+=" || next.Value == "-=" || next.Value == "*=" || next.Value == "/=") {
		op := next.Value
		p.nextFiltered()
		rhs := p.parseExpr()
		return &AssignStmt{Lhs: expr, Op: op, Rhs: rhs, Pos: startPos}
	}

	return &ExprStmt{X: expr, Pos: startPos}
}

// ===== 表达式解析（Pratt parsing，简化版）=====

// parseExpr 解析表达式
func (p *Parser) parseExpr() Expr {
	return p.parseBinaryExpr(0)
}

// 优先级表
var operatorPrecedence = map[string]int{
	"||": 1, "或": 1,
	"&&": 2, "且": 2,
	"==": 3, "!=": 3, "≠": 3, "＝＝": 3, "≠≠": 3,
	"<": 4, ">": 4, "<=": 4, ">=": 4, "＜": 4, "＞": 4, "≤": 4, "≥": 4, "＜＝": 4, "＞＝": 4,
	"+": 5, "-": 5, "加": 5, "减": 5,
	"*": 6, "/": 6, "%": 6, "乘": 6, "除": 6, "取余": 6,
	"<<": 7, ">>": 7, "＜＜": 7, "＞＞": 7,
	"&": 8, "|": 8, "^": 8,
}

// parseBinaryExpr 解析二元表达式
func (p *Parser) parseBinaryExpr(minPrec int) Expr {
	left := p.parseUnaryExpr()
	if left == nil {
		return nil
	}
	for {
		t := p.peekFiltered()
		// 且/或 是 TokenKeyword 但用作操作符，需要特殊处理
		isLogicalKw := t.Type == TokenKeyword && (t.Value == "且" || t.Value == "或")
		if t.Type != TokenOperator && !isLogicalKw {
			break
		}
		prec, ok := operatorPrecedence[t.Value]
		if !ok || prec < minPrec {
			break
		}
		p.nextFiltered()
		right := p.parseBinaryExpr(prec + 1)
		if right == nil {
			break
		}
		left = &BinaryExpr{Op: t.Value, Lhs: left, Rhs: right, Pos: Pos{t.Line, t.Col}}
	}
	return left
}

// parseUnaryExpr 解析一元表达式
func (p *Parser) parseUnaryExpr() Expr {
	t := p.peekFiltered()
	if t.Type == TokenOperator && (t.Value == "!" || t.Value == "-" || t.Value == "+" || t.Value == "&" || t.Value == "*") {
		p.nextFiltered()
		x := p.parseUnaryExpr()
		return &UnaryExpr{Op: t.Value, X: x, Pos: Pos{t.Line, t.Col}}
	}
	// 通道接收：<-ch（一元前缀）
	if t.Type == TokenOperator && t.Value == "<-" {
		p.nextFiltered()
		x := p.parseUnaryExpr()
		return &ChanExpr{Op: "<-", Chan: x, Value: nil, Pos: Pos{t.Line, t.Col}}
	}
	if t.Type == TokenKeyword && t.Value == "非" {
		p.nextFiltered()
		x := p.parseUnaryExpr()
		return &UnaryExpr{Op: "!", X: x, Pos: Pos{t.Line, t.Col}}
	}
	return p.parsePrimary()
}

// parsePrimary 解析基本表达式
func (p *Parser) parsePrimary() Expr {
	t := p.peekFiltered()

	switch t.Type {
	case TokenNumber:
		p.nextFiltered()
		return &Literal{Value: t.Value, Kind: "number", Pos: Pos{t.Line, t.Col}}
	case TokenString:
		p.nextFiltered()
		return &Literal{Value: t.Value, Kind: "string", Pos: Pos{t.Line, t.Col}}
	case TokenChar:
		p.nextFiltered()
		return &Literal{Value: t.Value, Kind: "char", Pos: Pos{t.Line, t.Col}}
	case TokenKeyword:
		switch t.Value {
		case "真":
			p.nextFiltered()
			return &Literal{Value: "true", Kind: "bool", Pos: Pos{t.Line, t.Col}}
		case "假":
			p.nextFiltered()
			return &Literal{Value: "false", Kind: "bool", Pos: Pos{t.Line, t.Col}}
		case "空":
			p.nextFiltered()
			return &Literal{Value: "nil", Kind: "nil", Pos: Pos{t.Line, t.Col}}
		case "序数":
			// 序数 → iota，仅在 枚举 块内有效
			p.nextFiltered()
			return &IotaExpr{Pos: Pos{t.Line, t.Col}}
		case "映射":
			return p.parseMapLiteral()
		case "新建":
			e := p.parseNewExpr()
			if e == nil {
				return nil
			}
			return p.parseSuffix(e)
		case "函数":
			// 匿名函数字面量：函数 (参数) 返回类型 ... 结束函数
			fl := p.parseFuncLit()
			if fl == nil {
				return nil
			}
			return p.parseSuffix(fl)
		case "恢复":
			// 恢复() → recover()，无参数的内建函数
			p.nextFiltered()
			p.consumeDelimiter("(")
			p.consumeDelimiter(")")
			return p.parseSuffix(&RecoverExpr{Pos: Pos{t.Line, t.Col}})
		}
		// 其他关键字不允许在表达式中，消费并报错（避免死循环）
		p.errorf(Pos{t.Line, t.Col}, "表达式位置不允许关键字 %q", t.Value)
		p.nextFiltered()
		return nil
	case TokenKeywordType:
		// 类型转换：整数型(x) → int(x)
		// 类型关键字后必须跟 ( 才合法
		p.nextFiltered()
		if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "(" {
			p.nextFiltered() // 消费 (
			arg := p.parseExpr()
			if !p.consumeDelimiter(")") {
				p.errorf(Pos{t.Line, t.Col}, "类型转换 %q 缺少右括号", t.Value)
				return nil
			}
			if arg == nil {
				return nil
			}
			return p.parseSuffix(&TypeConvertExpr{Type: t.Value, Arg: arg, Pos: Pos{t.Line, t.Col}})
		}
		// 类型名出现在表达式位置但不跟 (，报错
		p.errorf(Pos{t.Line, t.Col}, "类型 %q 出现在表达式位置，需要 (args) 形式的类型转换", t.Value)
		return nil
	case TokenIdentifier, TokenChineseText:
		p.nextFiltered()
		// 检测数组字面量：XXX数组{...}（XXX 是类型前缀，如 "整数"/"文本"/"整数型"）
		if strings.HasSuffix(t.Value, "数组") && p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "{" {
			arr := p.parseArrayLiteral(t.Value, t.Line, t.Col)
			if arr == nil {
				return nil
			}
			return p.parseSuffix(arr)
		}
		// 检测限定类型 struct literal：包名.T{...} 或 包名.子包.T{...}
		// 用前瞻确认整体模式，匹配则消费并返回完整限定类型名
		if qualifiedName, segCount := p.tryPeekQualifiedStructLit(t.Value); segCount > 0 {
			// 消费已前瞻的 .T 段（segCount 段，每段是 . + ident）
			for i := 0; i < segCount; i++ {
				p.nextFiltered() // 消费 .
				p.nextFiltered() // 消费 ident
			}
			sl := p.parseStructLiteral(qualifiedName, t.Line, t.Col)
			if sl == nil {
				return nil
			}
			return p.parseSuffix(sl)
		}
		// 检测结构体字面量：TypeName{...}（本地类型名）
		// 排除关键字场景：如果 t.Value 是控制流关键字不应到这里（已是 TokenKeyword）
		// 但仍需排除可能的中文类型关键字（TokenKeywordType 单独走分支）
		if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "{" {
			sl := p.parseStructLiteral(t.Value, t.Line, t.Col)
			if sl == nil {
				return nil
			}
			return p.parseSuffix(sl)
		}
		ident := &Ident{Name: t.Value, Pos: Pos{t.Line, t.Col}}
		return p.parseSuffix(ident)
	case TokenDelimiter:
		if t.Value == "(" {
			p.nextFiltered()
			inner := p.parseExpr()
			p.consumeDelimiter(")")
			if inner == nil {
				return nil
			}
			return p.parseSuffix(&ParenExpr{X: inner, Pos: Pos{t.Line, t.Col}})
		}
	}

	return nil
}

// parseMapLiteral 解析映射字面量
// 语法：映射 <KeyType> <ValueType> { k1: v1, k2: v2, ... }
// 例如：映射 文本型 整数型 { "a": 1, "b": 2 }
// KeyType / ValueType 复用 parseType，支持 *Type / 通道 Type / 包名.T 等复合类型
func (p *Parser) parseMapLiteral() Expr {
	startTok := p.peekFiltered()
	startPos := Pos{startTok.Line, startTok.Col}
	p.nextFiltered() // 消费 "映射"

	// KeyType（复用 parseType，支持 *Type / 通道 Type / 包名.T 等）
	keyType := p.parseType()
	if keyType == "" {
		p.errorf(startPos, "映射缺少键类型")
		return nil
	}

	// ValueType
	valType := p.parseType()
	if valType == "" {
		p.errorf(startPos, "映射缺少值类型")
		return nil
	}

	// {
	if !p.consumeDelimiter("{") {
		p.errorf(startPos, "映射字面量缺少左花括号")
		return nil
	}

	lit := &MapLiteral{
		KeyType:   keyType,
		ValueType: valType,
		Pos:       startPos,
	}

	// 空映射：{ }
	if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "}" {
		p.nextFiltered()
		return lit
	}

	// 解析键值对列表 k: v (, k: v)*
	for {
		key := p.parseExpr()
		if key == nil {
			break
		}
		if !p.consumeDelimiter(":") {
			p.errorf(startPos, "映射键值对缺少冒号 :")
			break
		}
		val := p.parseExpr()
		if val == nil {
			break
		}
		lit.Pairs = append(lit.Pairs, KeyValueExpr{Key: key, Value: val})

		// 逗号则继续，否则结束
		if !p.consumeDelimiter(",") {
			break
		}
	}

	p.consumeDelimiter("}")
	return lit
}

// parseArrayLiteral 解析数组字面量
// 语法：XXX数组{e1, e2, ...}（XXX 是类型前缀，如 "整数"/"文本"/"整数型"）
// 例如：整数数组{1,2,3} → []int{1, 2, 3}
// 调用前：已消费类型 token（如 "整数数组"），下一个 token 应为 "{"
func (p *Parser) parseArrayLiteral(typeText string, line, col int) Expr {
	// 提取 ElemType：去掉 "数组" 后缀，加 "型"（如果未有）
	elemType := strings.TrimSuffix(typeText, "数组")
	if !strings.HasSuffix(elemType, "型") {
		elemType += "型"
	}

	startPos := Pos{line, col}

	// 消费 {
	if !p.consumeDelimiter("{") {
		p.errorf(startPos, "数组字面量缺少左花括号")
		return nil
	}

	lit := &ArrayLiteral{
		ElemType: elemType,
		Pos:      startPos,
	}

	// 空数组 { }
	if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "}" {
		p.nextFiltered()
		return lit
	}

	// 解析元素列表 e1, e2, ...
	for {
		e := p.parseExpr()
		if e == nil {
			break
		}
		lit.Elements = append(lit.Elements, e)
		if !p.consumeDelimiter(",") {
			break
		}
	}

	p.consumeDelimiter("}")
	return lit
}

// parseStructLiteral 解析结构体字面量
// 语法：TypeName{e1, e2, ...}（按位置）或 TypeName{f1: v1, f2: v2, ...}（按字段名）
// 例如：Point{1, 2} 或 Point{x: 1, y: 2}
// 调用前：已消费类型 token（如 "Point"），下一个 token 应为 "{"
func (p *Parser) parseStructLiteral(typeName string, line, col int) Expr {
	startPos := Pos{line, col}

	// 消费 {
	if !p.consumeDelimiter("{") {
		p.errorf(startPos, "结构体字面量缺少左花括号")
		return nil
	}

	lit := &StructLiteral{
		TypeName: typeName,
		Pos:      startPos,
	}

	// 空结构体 { }
	if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "}" {
		p.nextFiltered()
		return lit
	}

	// 解析元素列表 e1, e2, ... 或 f1: v1, f2: v2, ...
	for {
		// 先尝试解析一个表达式（可能是字段名或值）
		first := p.parseExpr()
		if first == nil {
			break
		}

		var pair KeyValueExpr
		// 检测是否是字段名:值形式（first 后跟 :）
		if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == ":" {
			p.nextFiltered() // 消费 :
			val := p.parseExpr()
			if val == nil {
				break
			}
			pair.Key = first
			pair.Value = val
		} else {
			// 按位置赋值（Key 为 nil）
			pair.Key = nil
			pair.Value = first
		}
		lit.Pairs = append(lit.Pairs, pair)

		if !p.consumeDelimiter(",") {
			break
		}
	}

	p.consumeDelimiter("}")
	return lit
}

// parseSuffix 解析后缀（函数调用 . 成员访问 [ 索引）
func (p *Parser) parseSuffix(expr Expr) Expr {
	for {
		t := p.peekFiltered()
		if t.Type == TokenDelimiter {
			if t.Value == "(" {
			// 函数调用
			p.nextFiltered()
			args := p.parseArgList()
			// 可变参数展开：f(args...)
			ellipsis := false
			if nt := p.peekFiltered(); nt.Type == TokenOperator && nt.Value == "..." {
				ellipsis = true
				p.nextFiltered()
			}
			p.consumeDelimiter(")")
			expr = &CallExpr{Func: expr, Args: args, Ellipsis: ellipsis, Pos: Pos{t.Line, t.Col}}
			continue
		}
			if t.Value == "." {
			// 消费 "."，然后判断是成员访问还是类型断言 x.(Type)
			p.nextFiltered()
			// 类型断言：.( 后跟类型或 type 关键字
			if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "(" {
				startPos := Pos{t.Line, t.Col}
				p.nextFiltered() // 消费 (

				nextTok := p.peekFiltered()
				// x.(type) / x.(类型) 形式（用于 type switch，v53 支持中文 类型）
				if (nextTok.Type == TokenIdentifier && nextTok.Value == "type") ||
					(nextTok.Type == TokenKeyword && nextTok.Value == "类型") {
					p.nextFiltered() // 消费 type / 类型
					if !p.consumeDelimiter(")") {
						p.errorf(startPos, "类型断言 .(type) 缺少右括号")
						break
					}
					expr = &TypeAssertExpr{X: expr, Type: "", Pos: startPos}
					continue
				}

				// x.(Type) 形式：Type 是类型名（中文类型关键字/标识符/中文文本）
				if nextTok.Type == TokenKeywordType || nextTok.Type == TokenIdentifier || nextTok.Type == TokenChineseText {
					typeName := nextTok.Value
					p.nextFiltered()
					// 支持指针类型 x.(*Type)
					if p.peekFiltered().Type == TokenOperator && p.peekFiltered().Value == "*" {
						p.nextFiltered()
						typeName = "*" + typeName
						// 再读一个类型名
						t2 := p.peekFiltered()
						if t2.Type == TokenKeywordType || t2.Type == TokenIdentifier || t2.Type == TokenChineseText {
							typeName += t2.Value
							p.nextFiltered()
						}
					}
					if !p.consumeDelimiter(")") {
						p.errorf(startPos, "类型断言 .(Type) 缺少右括号")
						break
					}
					expr = &TypeAssertExpr{X: expr, Type: typeName, Pos: startPos}
					continue
				}

				// .( 后不是类型也不是 type，无法识别，回退到成员访问逻辑
				p.errorf(Pos{t.Line, t.Col}, "类型断言 .( 后必须是类型或 type")
				break
			}

			// 成员访问
			selTok := p.peekFiltered()
			if selTok.Type != TokenIdentifier && selTok.Type != TokenChineseText {
				break
			}
			p.nextFiltered()
			expr = &MemberExpr{X: expr, Sel: selTok.Value, Pos: Pos{t.Line, t.Col}}
			continue
		}
			if t.Value == "[" {
			// 索引 x[i] 或切片 x[low:high] / x[:high] / x[low:] / x[:] / x[low:high:max]
			startPos := Pos{t.Line, t.Col}
			p.nextFiltered() // 消费 [

			// 情况 1：x[:high] 或 x[:] 或 x[:high:max] —— [ 后直接是 :
			if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == ":" {
				p.nextFiltered() // 消费第一个 :
				var high Expr
				if !(p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "]") {
					high = p.parseExpr()
				}
				// 检查第二个 : （三元切片 x[:high:max]）
				var max Expr
				if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == ":" {
					p.nextFiltered() // 消费第二个 :
					if !(p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "]") {
						max = p.parseExpr()
					}
				}
				p.consumeDelimiter("]")
				expr = &SliceExpr{X: expr, Low: nil, High: high, Max: max, Pos: startPos}
				continue
			}

			// 解析第一个表达式（low 或 index）
			first := p.parseExpr()
			if first == nil {
				p.consumeDelimiter("]")
				break
			}

			// 情况 2：x[low:high] / x[low:] / x[low:high:max] —— first 后是 :
			if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == ":" {
				p.nextFiltered() // 消费第一个 :
				var high Expr
				if !(p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "]") {
					high = p.parseExpr()
				}
				// 检查第二个 : （三元切片 x[low:high:max]）
				var max Expr
				if p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == ":" {
					p.nextFiltered() // 消费第二个 :
					if !(p.peekFiltered().Type == TokenDelimiter && p.peekFiltered().Value == "]") {
						max = p.parseExpr()
					}
				}
				p.consumeDelimiter("]")
				expr = &SliceExpr{X: expr, Low: first, High: high, Max: max, Pos: startPos}
				continue
			}

			// 情况 3：x[index] —— 索引访问
			p.consumeDelimiter("]")
			expr = &IndexExpr{X: expr, Index: first, Pos: startPos}
			continue
		}
		}
		break
	}
	return expr
}

// parseArgList 解析函数参数列表
func (p *Parser) parseArgList() []Expr {
	var args []Expr
	for {
		t := p.peekFiltered()
		if t.Type == TokenDelimiter && t.Value == ")" {
			break
		}
		if t.Type == TokenEOF {
			break
		}
		arg := p.parseExpr()
		if arg != nil {
			args = append(args, arg)
		}
		// 消费可能的逗号
		if !p.consumeDelimiter(",") {
			break
		}
	}
	return args
}
