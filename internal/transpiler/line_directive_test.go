// Package transpiler — //line 指令测试（#@eg-file 标记识别 + 多文件合并场景）
package transpiler

import (
	"strings"
	"testing"
)

// TestLineDirectiveSingleFileNoMarker 验证单文件场景（无 #@eg-file 标记）不插入 //line 指令
// 保持输出干净，避免噪音
func TestLineDirectiveSingleFileNoMarker(t *testing.T) {
	src := `# 程序集 main

函数 主函数()
	返回
结束函数
`
	out, err := TranspileAST(src)
	if err != nil {
		t.Fatalf("TranspileAST failed: %v", err)
	}
	if strings.Contains(out, "//line ") {
		t.Errorf("单文件场景不应插入 //line 指令, 实际:\n%s", out)
	}
}

// TestLineDirectiveMergedFiles 验证多文件合并场景正确插入 //line 指令
// 模拟 runner.go mergeLibsFromDir 生成的合并源码：主源码 + #@eg-file 标记 + 扩展包源码
func TestLineDirectiveMergedFiles(t *testing.T) {
	// 模拟合并源码：第 1-5 行是主源码，第 6 行是 #@eg-file 标记，第 7 行起是扩展包源码
	src := `# 程序集 main

函数 主函数()
	返回
结束函数

#@eg-file global:libs/stringx/source.eg
函数 取长度(s 文本型) 整数型
	返回 len(s)
结束函数
`
	file, errs := Parse(src)
	if len(errs) > 0 {
		t.Fatalf("Parse errors: %v", errs)
	}
	if file == nil {
		t.Fatal("file is nil")
	}

	// 验证 FileMarkers 被正确收集
	if len(file.FileMarkers) != 1 {
		t.Fatalf("期望 1 个 FileMarker, 实际 %d", len(file.FileMarkers))
	}
	m := file.FileMarkers[0]
	if m.FileName != "global:libs/stringx/source.eg" {
		t.Errorf("FileMarker.FileName 期望 global:libs/stringx/source.eg, 实际 %q", m.FileName)
	}
	if m.GlobalLine != 7 { // #@eg-file 在第 7 行（1-based）
		t.Errorf("FileMarker.GlobalLine 期望 7, 实际 %d", m.GlobalLine)
	}

	out, err := GenerateGo(file)
	if err != nil {
		t.Fatalf("GenerateGo failed: %v", err)
	}

	// 验证主源码声明前插入 //line "源码.eg":N（主源码在标记之前，用默认文件名）
	if !strings.Contains(out, `//line "源码.eg":`) {
		t.Errorf("期望主源码声明前插入 //line \"源码.eg\":N, 实际:\n%s", out)
	}

	// 验证扩展包声明前插入 //line "global:libs/stringx/source.eg":N
	if !strings.Contains(out, `//line "global:libs/stringx/source.eg":`) {
		t.Errorf("期望扩展包声明前插入 //line \"global:libs/stringx/source.eg\":N, 实际:\n%s", out)
	}
}

// TestLineDirectiveFileLocalLineCalc 验证文件内行号计算正确
// #@eg-file 标记在合并源码第 N 行，下一行是文件第 1 行，所以声明在合并源码第 M 行时文件内行号 = M - N
func TestLineDirectiveFileLocalLineCalc(t *testing.T) {
	// 第 1-3 行主源码，第 4 行标记，第 5 行扩展包函数声明
	src := `# 程序集 main

#@eg-file global:libs/x/source.eg
函数 foo()
	返回
结束函数
`
	file, _ := Parse(src)
	if file == nil {
		t.Fatal("file is nil")
	}
	out, _ := GenerateGo(file)

	// foo 声明在合并源码第 5 行，标记在第 4 行，文件内行号 = 5 - 4 = 1
	if !strings.Contains(out, `//line "global:libs/x/source.eg":1`) {
		t.Errorf("期望 //line \"global:libs/x/source.eg\":1, 实际:\n%s", out)
	}
}
