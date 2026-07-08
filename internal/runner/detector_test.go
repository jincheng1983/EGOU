package runner

import (
	"testing"
)

// TestFindGoInCommonPaths 验证 Go 编译器常见安装路径扫描。
// 非 Windows 平台应返回空串；Windows 平台若存在常见路径则返回该路径。
func TestFindGoInCommonPaths(t *testing.T) {
	p := findGoInCommonPaths()
	// 不做硬性断言（取决于机器是否安装 Go），只验证不 panic 且返回字符串
	if p == "" {
		t.Logf("未在常见路径找到 Go 编译器（正常情况，取决于机器配置）")
	} else {
		t.Logf("在常见路径找到 Go 编译器: %s", p)
	}
}

// TestFindGCC 验证 GCC 编译器查找。
func TestFindGCC(t *testing.T) {
	p := findGCC()
	if p == "" {
		t.Logf("未找到 GCC（cgo 不可用，正常情况）")
	} else {
		t.Logf("找到 GCC: %s", p)
	}
}

// TestFindClang 验证 Clang 编译器查找。
func TestFindClang(t *testing.T) {
	p := findClang()
	if p == "" {
		t.Logf("未找到 Clang（cgo 不可用，正常情况）")
	} else {
		t.Logf("找到 Clang: %s", p)
	}
}

// TestFindWindres 验证 windres 资源编译器查找。
func TestFindWindres(t *testing.T) {
	p := findWindres()
	if p == "" {
		t.Logf("未找到 windres（syso 生成会回退到其他方案）")
	} else {
		t.Logf("找到 windres: %s", p)
	}
}

// TestDetectToolchains 验证综合工具链检测。
// 不做硬性断言（取决于机器配置），只验证：
// 1. 不 panic
// 2. 返回的 ToolchainReport 结构完整
// 3. Go 工具的 Name 字段为 "go"
// 4. 若 Path 非空则 Version 也应非空（go version 应能解析）
func TestDetectToolchains(t *testing.T) {
	tc := DetectToolchains()

	if tc.Go.Name != "go" {
		t.Errorf("Go.Name = %q, 期望 %q", tc.Go.Name, "go")
	}
	if tc.Go.Path != "" && tc.Go.Version == "" {
		t.Errorf("Go.Path 非空但 Version 为空，应能解析版本号")
	}
	// CGO 的 Name 应为 "gcc" 或 "clang"
	if tc.CGO.Name != "gcc" && tc.CGO.Name != "clang" {
		t.Errorf("CGO.Name = %q, 期望 gcc 或 clang", tc.CGO.Name)
	}
	// 若 CGO.Path 非空则 Version 也应非空
	if tc.CGO.Path != "" && tc.CGO.Version == "" {
		t.Errorf("CGO.Path 非空但 Version 为空，应能解析版本号")
	}
	t.Logf("检测结果: Go=%+v, CGO=%+v, Windres=%+v, NPM=%+v, Rsrc=%+v, Wails3=%+v, Delve=%+v",
		tc.Go, tc.CGO, tc.Windres, tc.NPM, tc.Rsrc, tc.Wails3, tc.Delve)
}

// TestHealthCheckWithTempHome 验证 HealthCheck 在隔离 HOME 环境下不污染用户目录。
func TestHealthCheckWithTempHome(t *testing.T) {
	withTempHome(t)
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
		t.Errorf("HealthReport.TemplateOK 应为 true")
	}
	// OK 必须与 Go 编译器/模板状态一致
	expectedOK := rpt.GoCompiler != "" && rpt.TemplateOK
	if rpt.OK != expectedOK {
		t.Errorf("HealthReport.OK = %v, 但根据 GoCompiler/TemplateOK 推导应为 %v", rpt.OK, expectedOK)
	}
	// C 编译器字段应被填充（即使为空也应有字段）
	t.Logf("HealthCheck: GoCompiler=%s, GoVersion=%s, CCompiler=%s, CGOVersion=%s, Windres=%s",
		rpt.GoCompiler, rpt.GoVersion, rpt.CCompiler, rpt.CGOVersion, rpt.Windres)
}
