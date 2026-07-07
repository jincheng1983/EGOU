// Package transpiler — Token 分层实验（P3-2，吸取 NxEGO6）
//
// 设计目标：
//   - 把 @嵌入/@结束 升级为独立 Token 类型，为未来语法高亮/补全/错误恢复打基础
//   - TokenChineseText 作为独立类型（中文标识符）
//   - TokenNewline 区分逻辑行
//   - 作为并行实验，不替换现有 transpiler.go，仅供未来迁移参考
//
// 与现有 transpiler.go 的关系：
//   - 现有 transpiler 用正则 + 字符串替换处理，简单但缺乏结构
//   - 本 lexer 是真正的词法分析器，产出 Token 流
//   - 未来可在 Token 流基础上构建 AST，支持更精确的错误恢复
//
// 使用示例：
//
//	tokens := transpiler.Tokenize(source)
//	for _, tok := range tokens {
//	    fmt.Printf("%v: %q\n", tok.Type, tok.Value)
//	}
package transpiler

import (
	"strings"
	"unicode"
)

// TokenType 标识 Token 类型
type TokenType int

const (
	TokenEOF         TokenType = iota // 文件结束
	TokenNewline                      // 换行
	TokenWhitespace                   // 空白（非换行）
	TokenComment                      // // 注释
	TokenKeyword                      // 关键字（如果/否则/循环/函数等）
	TokenKeywordType                  // 类型关键字（整数型/文本型等，与 TokenKeyword 区分以便 AST 解析）
	TokenChineseText                  // 中文标识符（变量名/函数名）
	TokenIdentifier                   // 英文标识符
	TokenNumber                       // 数字字面量
	TokenString                       // 字符串字面量
	TokenChar                         // 字符字面量
	TokenOperator                     // 运算符 + - * / % = 等
	TokenDelimiter                    // 分隔符 ( ) [ ] { } , ;
	TokenEmbed                        // @嵌入（独立 Token，P3-2 核心）
	TokenEndEmbed                     // @结束（独立 Token，P3-2 核心）
	TokenAssembly                     // @汇编（保留）
	TokenUnknown                      // 未识别字符
)

// Token 是词法分析的产出单元
type Token struct {
	Type  TokenType
	Value string
	Line  int // 1-based 行号
	Col   int // 1-based 列号
}

// String 实现 Stringer 接口
func (t Token) String() string {
	return formatTokenType(t.Type) + "(" + t.Value + ")@" + itoa(t.Line) + ":" + itoa(t.Col)
}

func formatTokenType(t TokenType) string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenNewline:
		return "NEWLINE"
	case TokenWhitespace:
		return "WS"
	case TokenComment:
		return "COMMENT"
	case TokenKeyword:
		return "KW"
	case TokenKeywordType:
		return "KWT"
	case TokenChineseText:
		return "ZH"
	case TokenIdentifier:
		return "ID"
	case TokenNumber:
		return "NUM"
	case TokenString:
		return "STR"
	case TokenChar:
		return "CHAR"
	case TokenOperator:
		return "OP"
	case TokenDelimiter:
		return "DELIM"
	case TokenEmbed:
		return "EMBED"
	case TokenEndEmbed:
		return "ENDEMBED"
	case TokenAssembly:
		return "ASM"
	case TokenUnknown:
		return "UNKNOWN"
	}
	return "?"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// keywords 是 EGOU 关键字集合（与 transpiler.go 实际支持一致）
// 同步来源：docs/syntax_design.md §2 + frontend/src/utils/nxgKeywords.js
//
// 注意：包含"结束xxx"系列关键字。这是易语言风格块结构标记，不是冗余。
// 源码不写 Go 的 {}，块结构靠"结束xxx"显式闭合，转译器自动添加 Go 的 {}。
var keywords = map[string]bool{
	// 声明类
	"程序集": true, "导入": true,
	"常量": true, "变量": true, "局部变量": true,
	"类型": true, "结构体": true, "接口": true, "结束类型": true,
	"枚举": true, "结束枚举": true, "序数": true,
	"函数": true, "主函数": true, "结束函数": true,
	"初始化": true,
	"标签": true,
	"方法": true, "结束方法": true,
	// 控制流
	"如果": true, "否则": true, "否则如果": true, "结束如果": true,
	"循环": true, "结束循环": true,
	"判断循环": true, "结束判断循环": true,
	"选择": true, "情况": true, "默认": true, "结束选择": true,
	"通道选择": true, "结束通道选择": true,
	"返回": true, "继续": true, "跳出": true, "抛出": true, "恢复": true, "跳转": true, "穿透": true,
	// 修饰符
	"参数": true, "范围": true, "映射": true, "数组": true,
	// 字面量
	"真": true, "假": true, "空": true,
	// 待实现（先列入高亮，transpiler.go 暂不支持）
	"延迟": true, "协程": true, "通道": true, "新建": true,
	"且": true, "或": true, "非": true,
}

// typeKeywords 是中文类型关键字集合（与 transpiler.go mapType 函数一致）
// 这些不是控制流关键字，而是类型标识符，单独列表以便 AST 解析器识别
var typeKeywords = map[string]bool{
	"整数型": true, "长整数型": true, "短整数型": true, "字节型": true,
	"小数型": true, "双精度小数型": true,
	"文本型": true, "逻辑型": true, "变体型": true, "字节集": true,
	// v54 新增无符号系列 + 固定位宽整数 + rune
	"无符号整数型": true, "无符号短整数型": true, "无符号长整数型": true, "无符号字节型": true,
	"有符号8位整数型": true, "有符号32位整数型": true,
	"无符号8位整数型": true, "无符号16位整数型": true, "无符号32位整数型": true, "无符号64位整数型": true,
	"无符号指针整数型": true,
	"字符型": true,
}

// blockPairs 是块结构开始→结束关键字配对表（用于 AST 解析器和代码折叠）
var blockPairs = map[string]string{
	"函数":   "结束函数",
	"方法":   "结束方法",
	"类型":   "结束类型",
	"如果":   "结束如果",
	"循环":   "结束循环",
	"判断循环": "结束判断循环",
	"选择":   "结束选择",
	"通道选择": "结束通道选择",
}

// IsBlockStart 判断是否是块开始关键字
func IsBlockStart(s string) bool {
	_, ok := blockPairs[s]
	return ok
}

// IsBlockEnd 判断是否是块结束关键字
func IsBlockEnd(s string) bool {
	for _, end := range blockPairs {
		if end == s {
			return true
		}
	}
	return false
}

// GetBlockEnd 返回块开始关键字对应的结束关键字，不是块开始则返回空串
func GetBlockEnd(start string) string {
	return blockPairs[start]
}

// isChineseChar 判断是否中文字符
// 注意：全角运算符（＝≠＞＜≥≤＋－×÷ 等）由 isOperator 处理，这里不包含
func isChineseChar(r rune) bool {
	if unicode.Is(unicode.Han, r) {
		return true
	}
	// CJK 标点（不含全角运算符，那些由 isOperator 处理）
	if r >= 0x3000 && r <= 0x303F {
		return !isOperator(r) // 排除被 isOperator 接管的全角运算符
	}
	// 全角符号区（0xFF00-0xFFEF）：排除运算符
	if r >= 0xFF00 && r <= 0xFFEF {
		return !isOperator(r)
	}
	return false
}

// isIdentStart 判断标识符起始字符
func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || isChineseChar(r)
}

// isIdentPart 判断标识符后续字符
func isIdentPart(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r)
}

// isOperator 判断运算符
// 包含 ASCII 运算符 + 全角运算符（＝≠＞＜≥≤＋－×÷ 等）
// 注意：全角等号 ＝ (U+FF1D) 必须优先识别为运算符，否则会被 isChineseChar 吞入标识符
func isOperator(r rune) bool {
	return strings.ContainsRune("+-*/%=<>!&|^~", r) ||
		strings.ContainsRune("＝≠＞＜≥≤＋－×÷", r)
}

// isDelimiter 判断分隔符
func isDelimiter(r rune) bool {
	return strings.ContainsRune("()[]{},;:.", r)
}

// Tokenize 把源码字符串转为 Token 流
func Tokenize(src string) []Token {
	var tokens []Token
	lines := strings.Split(src, "\n")

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		col := 1
		runes := []rune(line)

		i := 0
		for i < len(runes) {
			r := runes[i]

			// 换行（虽然按行切分，但保留 TokenNewline 标记逻辑行结束）
			if r == '\r' {
				i++
				col++
				continue
			}

			// 空白
			if r == ' ' || r == '\t' {
				start := i
				for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
					i++
				}
				tokens = append(tokens, Token{TokenWhitespace, string(runes[start:i]), lineNum, col})
				col += i - start
				continue
			}

			// 注释 //（行注释）
			if r == '/' && i+1 < len(runes) && runes[i+1] == '/' {
				tokens = append(tokens, Token{TokenComment, string(runes[i:]), lineNum, col})
				i = len(runes)
				continue
			}

			// # 程序集 / # 注释（# 开头视为行注释，与 transpiler.go 一致）
			// 注意：# 程序集 main 是包声明语法，parser 会从 TokenComment 中提取
			if r == '#' {
				tokens = append(tokens, Token{TokenComment, string(runes[i:]), lineNum, col})
				i = len(runes)
				continue
			}

			// @嵌入 / @结束 / @汇编（独立 Token，P3-2 核心）
			if r == '@' {
				rest := string(runes[i:])
				if strings.HasPrefix(rest, "@嵌入") {
					tokens = append(tokens, Token{TokenEmbed, "@嵌入", lineNum, col})
					i += len([]rune("@嵌入"))
					col += len([]rune("@嵌入"))
					continue
				}
				if strings.HasPrefix(rest, "@结束") {
					tokens = append(tokens, Token{TokenEndEmbed, "@结束", lineNum, col})
					i += len([]rune("@结束"))
					col += len([]rune("@结束"))
					continue
				}
				if strings.HasPrefix(rest, "@汇编") {
					tokens = append(tokens, Token{TokenAssembly, "@汇编", lineNum, col})
					i += len([]rune("@汇编"))
					col += len([]rune("@汇编"))
					continue
				}
				// 其他 @ 开头的视为未知
				tokens = append(tokens, Token{TokenUnknown, "@", lineNum, col})
				i++
				col++
				continue
			}

			// 字符串字面量 "..."
			if r == '"' {
				start := i
				i++
				col++
				for i < len(runes) && runes[i] != '"' {
					if runes[i] == '\\' && i+1 < len(runes) {
						i += 2
						col += 2
					} else {
						i++
						col++
					}
				}
				if i < len(runes) {
					i++ // closing "
					col++
				}
				tokens = append(tokens, Token{TokenString, string(runes[start:i]), lineNum, col - (i - start)})
				continue
			}

			// 字符字面量 '...'
			if r == '\'' {
				start := i
				i++
				col++
				for i < len(runes) && runes[i] != '\'' {
					if runes[i] == '\\' && i+1 < len(runes) {
						i += 2
						col += 2
					} else {
						i++
						col++
					}
				}
				if i < len(runes) {
					i++
					col++
				}
				tokens = append(tokens, Token{TokenChar, string(runes[start:i]), lineNum, col - (i - start)})
				continue
			}

			// 数字字面量
			if unicode.IsDigit(r) {
				start := i
				for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
					i++
				}
				tokens = append(tokens, Token{TokenNumber, string(runes[start:i]), lineNum, col})
				col += i - start
				continue
			}

			// 标识符 / 关键字 / 中文文本
			if isIdentStart(r) {
				start := i
				for i < len(runes) && isIdentPart(runes[i]) {
					i++
				}
				val := string(runes[start:i])
				if keywords[val] {
					tokens = append(tokens, Token{TokenKeyword, val, lineNum, col})
				} else if typeKeywords[val] {
					tokens = append(tokens, Token{TokenKeywordType, val, lineNum, col})
				} else if isChineseChar(r) {
					tokens = append(tokens, Token{TokenChineseText, val, lineNum, col})
				} else {
					tokens = append(tokens, Token{TokenIdentifier, val, lineNum, col})
				}
				col += i - start
				continue
			}

			// 运算符
			if isOperator(r) {
				start := i
				for i < len(runes) && isOperator(runes[i]) {
					i++
				}
				tokens = append(tokens, Token{TokenOperator, string(runes[start:i]), lineNum, col})
				col += i - start
				continue
			}

			// 分隔符
			if isDelimiter(r) {
				// 特判 := 短声明运算符，合并为单一 Operator token
				// 否则 : 和 = 会被切成两个 token，parser 难以识别
				if r == ':' && i+1 < len(runes) && runes[i+1] == '=' {
					tokens = append(tokens, Token{TokenOperator, ":=", lineNum, col})
					i += 2
					col += 2
					continue
				}
				// 特判 ... 可变参数运算符，合并为单一 Operator token
				// 否则 . 会被切成三个 delimiter token，parser 难以识别
				if r == '.' && i+2 < len(runes) && runes[i+1] == '.' && runes[i+2] == '.' {
					tokens = append(tokens, Token{TokenOperator, "...", lineNum, col})
					i += 3
					col += 3
					continue
				}
				tokens = append(tokens, Token{TokenDelimiter, string(r), lineNum, col})
				i++
				col++
				continue
			}

			// 未知字符
			tokens = append(tokens, Token{TokenUnknown, string(r), lineNum, col})
			i++
			col++
		}

		// 行结束（保留逻辑行标记，便于解析器识别语句边界）
		tokens = append(tokens, Token{TokenNewline, "\n", lineNum, col})
	}

	// EOF
	tokens = append(tokens, Token{TokenEOF, "", len(lines), 1})
	return tokens
}

// FilterWhitespace 过滤掉空白和换行 Token（用于语法分析）
func FilterWhitespace(tokens []Token) []Token {
	out := make([]Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Type != TokenWhitespace && t.Type != TokenNewline {
			out = append(out, t)
		}
	}
	return out
}

// CountByType 按类型统计 Token 数量（用于调试/测试）
func CountByType(tokens []Token) map[TokenType]int {
	counts := make(map[TokenType]int)
	for _, t := range tokens {
		counts[t.Type]++
	}
	return counts
}
