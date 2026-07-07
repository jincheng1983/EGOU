package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"egou/internal/transpiler"
)

func TestStripLibEntryDeclarations(t *testing.T) {
	src := `# 程序集 hello
导入 (
    "fmt"
)

函数 主函数()
    fmt.Println("hi")
结束函数

函数 辅助_被单击(参数 x 整数型)
    打印(x)
结束函数
`
	got := stripLibEntryDeclarations(src)
	if strings.Contains(got, "程序集") {
		t.Errorf("未剥离 # 程序集 行: %s", got)
	}
	if strings.Contains(got, "导入 (") {
		t.Errorf("未剥离 导入 块: %s", got)
	}
	if strings.Contains(got, "主函数") {
		t.Errorf("未剥离 主函数 段: %s", got)
	}
	if !strings.Contains(got, "辅助_被单击") {
		t.Errorf("普通函数被误删: %s", got)
	}
}

func TestMergeProjectLibs(t *testing.T) {
	root := t.TempDir()
	libs := filepath.Join(root, "libs")
	pkg := filepath.Join(libs, "alpha")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	// 没有 commands.json 的目录应被跳过
	if err := os.MkdirAll(filepath.Join(libs, "no-cmd"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "commands.json"), []byte(`{"commands":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	src := `# 程序集 hello
导入 (
    "fmt"
)

函数 主函数()
    fmt.Println("main")
结束函数
`
	if err := os.WriteFile(filepath.Join(pkg, "source.eg"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	mainSrc := `# 程序集 hello
导入 (
    "fmt"
)

函数 主函数()
    fmt.Println("entry")
结束函数
`
	got, err := mergeProjectLibs(mainSrc, root)
	if err != nil {
		t.Fatalf("mergeProjectLibs 失败: %v", err)
	}
	// 新格式使用 #@eg-file 标记每个 .elib 源码块（originTag:libs/pkg/source.eg）
	if !strings.Contains(got, "#@eg-file project:libs/alpha/source.eg") {
		t.Errorf("合并结果中没看到 alpha 文件标记: %s", got)
	}
	// 主源码的 # 程序集 只能出现一次（合并后）
	if cnt := strings.Count(got, "程序集"); cnt != 1 {
		t.Errorf("程序集 出现 %d 次，期望 1 次: %s", cnt, got)
	}
	// alpha 包的 # 程序集 应被剥离：标记行之后不应紧接 # 程序集
	if strings.Contains(got, "#@eg-file project:libs/alpha/source.eg\n# 程序集") {
		t.Errorf("alpha 包未剥离 # 程序集: %s", got)
	}
}

func TestMergeProjectLibs_NoLibsDir(t *testing.T) {
	root := t.TempDir()
	main := "函数 测试()\n结束函数\n"
	got, err := mergeProjectLibs(main, root)
	if err != nil {
		t.Fatal(err)
	}
	if got != main {
		t.Errorf("无 libs 目录时输出应等于输入，得到: %s", got)
	}
}

func TestMergeProjectLibs_AliasRegistration(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "libs", "mylib")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	cmdsJSON := `{"library":"mylib","commands":[{"displayName":"示例命令","englishName":"HelloNlib"}]}`
	if err := os.WriteFile(filepath.Join(pkg, "commands.json"), []byte(cmdsJSON), 0644); err != nil {
		t.Fatal(err)
	}
	libSrc := `函数 HelloNlib(参数 名字 文本型) 文本型
    返回 "你好"
结束函数
`
	if err := os.WriteFile(filepath.Join(pkg, "source.eg"), []byte(libSrc), 0644); err != nil {
		t.Fatal(err)
	}
	mainSrc := `# 程序集 main

函数 主函数()
    结果 ＝ 示例命令("张三")
结束函数
`
	merged, err := mergeProjectLibs(mainSrc, root)
	if err != nil {
		t.Fatalf("mergeProjectLibs 失败: %v", err)
	}
	goSrc, err := transpiler.Transpile(merged)
	transpiler.ClearExtraAliases()
	if err != nil {
		t.Fatalf("Transpile 失败: %v", err)
	}
	if !strings.Contains(goSrc, "HelloNlib(") {
		t.Errorf("中文别名未被替换为英文键:\n%s", goSrc)
	}
	if strings.Contains(goSrc, "示例命令(") {
		t.Errorf("中文别名未被替换:\n%s", goSrc)
	}
}

// TestMergeProjectLibs_FullGoStructure 验证 .elib 合并+转译后产出是结构完整的 Go 源码：
// - 只有 1 个 package main
// - 只有 1 个 mainImpl（主函数映射）
// - .elib 的函数定义存在
// - 中文别名调用被替换为英文键
func TestMergeProjectLibs_FullGoStructure(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "libs", "mathutils")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	cmdsJSON := `{"library":"mathutils","commands":[
        {"displayName":"平方","englishName":"Square"}
    ]}`
	if err := os.WriteFile(filepath.Join(pkg, "commands.json"), []byte(cmdsJSON), 0644); err != nil {
		t.Fatal(err)
	}
	libSrc := `# 程序集 mathutils

导入 (
    "fmt"
)

函数 主函数()
    fmt.Println("不应进入主函数")
结束函数

函数 Square(参数 n 整数型) 整数型
    返回 n * n
结束函数
`
	if err := os.WriteFile(filepath.Join(pkg, "source.eg"), []byte(libSrc), 0644); err != nil {
		t.Fatal(err)
	}
	mainSrc := `# 程序集 main

导入 (
    "fmt"
)

函数 主函数()
    结果 ＝ 平方(5)
    fmt.Println(结果)
结束函数
`
	merged, err := mergeProjectLibs(mainSrc, root)
	if err != nil {
		t.Fatalf("mergeProjectLibs 失败: %v", err)
	}
	goSrc, err := transpiler.Transpile(merged)
	transpiler.ClearExtraAliases()
	if err != nil {
		t.Fatalf("Transpile 失败: %v", err)
	}
	// 只有一个 package main
	if c := strings.Count(goSrc, "package main"); c != 1 {
		t.Errorf("package main 出现 %d 次，期望 1 次", c)
	}
	// 只有一个 mainImpl 定义（主函数映射）
	if c := strings.Count(goSrc, "func mainImpl()"); c != 1 {
		t.Errorf("mainImpl 出现 %d 次，期望 1 次", c)
	}
	// .elib 的 Square 函数定义存在
	if !strings.Contains(goSrc, "func Square(") {
		t.Errorf("Square 函数定义缺失:\n%s", goSrc)
	}
	// 中文别名被替换
	if !strings.Contains(goSrc, "Square(5)") {
		t.Errorf("中文别名 平方 未被替换为 Square:\n%s", goSrc)
	}
	// .elib 的 fmt.Println("不应进入主函数") 应被 strip 掉
	if strings.Contains(goSrc, "不应进入主函数") {
		t.Errorf(".elib 的主函数段未被剥离:\n%s", goSrc)
	}
}

// TestHealthCheck 验证健康检查基本字段填充正确：
// - OS/Arch 字段非空（来自 runtime）
// - TemplateDir 字段为 "(embedded)"（模板已通过 go:embed 嵌入 exe）
// - TemplateOK 总是 true
// - OK 字段与 GoCompiler + TemplateOK 一致
func TestHealthCheck(t *testing.T) {
	rpt := HealthCheck()

	if rpt.OS == "" {
		t.Errorf("HealthReport.OS 不应为空")
	}
	if rpt.Arch == "" {
		t.Errorf("HealthReport.Arch 不应为空")
	}
	if rpt.TemplateDir != "(embedded)" {
		t.Errorf("HealthReport.TemplateDir = %q, 期望 %q", rpt.TemplateDir, "(embedded)")
	}
	if !rpt.TemplateOK {
		t.Errorf("HealthReport.TemplateOK 应为 true（模板已嵌入 exe）")
	}

	// OK 必须与两个关键项一致
	expectedOK := rpt.GoCompiler != "" && rpt.TemplateOK
	if rpt.OK != expectedOK {
		t.Errorf("HealthReport.OK = %v, 但根据 GoCompiler/TemplateOK 推导应为 %v", rpt.OK, expectedOK)
	}

	// Go 编译器存在时，版本号应非空且以 "go" 开头
	if rpt.GoCompiler != "" {
		if rpt.GoVersion == "" {
			t.Errorf("GoCompiler 存在但 GoVersion 为空")
		} else if !strings.HasPrefix(rpt.GoVersion, "go") {
			t.Errorf("GoVersion = %q, 应以 'go' 开头", rpt.GoVersion)
		}
	}

	// Message 字段不应为空
	if rpt.Message == "" {
		t.Errorf("HealthReport.Message 不应为空")
	}
}

// TestTranslateGoError 验证编译错误英译中的翻译正确性。
// P2-5：测试用例覆盖常见 Go 编译错误，确保翻译后的中文可读且不破坏原始格式。
func TestTranslateGoError(t *testing.T) {
	cases := []struct{ en, zh string }{
		{"syntax error: unexpected semicolon", "语法错误: 意外的 semicolon"},
		{"undefined: foo", "未定义: foo"},
		{"cannot use x as type int in assignment to y", "无法使用 x 作为类型 int 在赋值给 y"},
		{"mismatched types int and string", "类型不匹配 int 和 string"},
		{"x declared and not used", "x 已声明但未使用"},
		{"imported and not used: fmt", "已导入但未使用: fmt"},
		{"no new variables on left side of :=", ":= 左侧没有新变量"},
		{"x redeclared in this block", "x 在此代码块中重复声明"},
		{"not enough arguments in call to foo()", "参数不足 在调用 foo()"},
		{"too many arguments in call to foo()", "参数过多 在调用 foo()"},
	}
	for _, c := range cases {
		got := translateGoError(c.en)
		if got != c.zh {
			t.Errorf("translateGoError(%q) = %q, 期望 %q", c.en, got, c.zh)
		}
	}
}

// TestParseGoCompileErrors 验证 Go 编译错误结构化解析。
func TestParseGoCompileErrors(t *testing.T) {
	output := `main.go:10:3: undefined: foo
main.go:15:5: syntax error: unexpected semicolon
main.go:20:1: x declared and not used`
	errs := parseGoCompileErrors(output)
	if len(errs) != 3 {
		t.Fatalf("解析到 %d 条错误, 期望 3", len(errs))
	}
	if errs[0].File != "main.go" || errs[0].Line != 10 || errs[0].Col != 3 {
		t.Errorf("第一条错误: %+v", errs[0])
	}
	if errs[0].Message != "undefined: foo" {
		t.Errorf("第一条错误消息: %q", errs[0].Message)
	}
	if errs[2].Line != 20 {
		t.Errorf("第三条错误行号: %d", errs[2].Line)
	}
}
