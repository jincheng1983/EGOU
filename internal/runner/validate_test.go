package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateOutputInProject 验证可执行文件路径校验逻辑。
// P2-14：防止用户配置 output 字段指向系统目录导致编译产物覆盖系统文件。
func TestValidateOutputInProject(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		exePath   string
		wantError bool
	}{
		{
			name:      "合法：exe 在项目 bin/ 内",
			exePath:   filepath.Join(projDir, "bin", "egruntime.exe"),
			wantError: false,
		},
		{
			name:      "合法：exe 在项目 dist/ 内（自定义 output）",
			exePath:   filepath.Join(projDir, "dist", "egruntime.exe"),
			wantError: false,
		},
		{
			name:      "合法：exe 在项目根目录",
			exePath:   filepath.Join(projDir, "egruntime.exe"),
			wantError: false,
		},
		{
			name:      "非法：exe 在项目外（上级目录）",
			exePath:   filepath.Join(tmpDir, "egruntime.exe"),
			wantError: true,
		},
		{
			name:      "非法：exe 在系统目录",
			exePath:   `C:\Windows\System32\egruntime.exe`,
			wantError: true,
		},
		{
			name:      "空 exePath 跳过校验",
			exePath:   "",
			wantError: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateOutputInProject(c.exePath, projDir)
			if c.wantError && err == nil {
				t.Errorf("期望返回错误，但返回 nil")
			}
			if !c.wantError && err != nil {
				t.Errorf("期望无错误，但返回: %v", err)
			}
			if c.wantError && err != nil && !strings.Contains(err.Error(), "项目目录内") {
				t.Errorf("错误消息应包含'项目目录内'，实际: %v", err)
			}
		})
	}
}

// TestValidateOutputInProject_EmptyProject 验证 projectPath 为空时跳过校验。
func TestValidateOutputInProject_EmptyProject(t *testing.T) {
	err := validateOutputInProject("/some/path.exe", "")
	if err != nil {
		t.Errorf("projectPath 为空时应跳过校验，但返回: %v", err)
	}
}

// TestBumpPatch 验证版本号 patch 段递增。
func TestBumpPatch(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"1.0.0", "1.0.1"},
		{"1.2.3", "1.2.4"},
		{"1.2", "1.2.1"},      // 缺失 patch 段补 0
		{"1", "1.0.1"},        // 缺失 minor/patch 段补 0
		{"1.0.0-dev", "1.0.1"}, // 去除后缀
		{"1.0.0+build123", "1.0.1"}, // 去除构建元数据
		{"0.0.0", "0.0.1"},
	}
	for _, c := range cases {
		got, err := bumpPatch(c.input)
		if err != nil {
			t.Errorf("bumpPatch(%q) 返回错误: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("bumpPatch(%q) = %q, 期望 %q", c.input, got, c.want)
		}
	}
}

// TestBumpPatch_Invalid 验证非法版本号处理。
func TestBumpPatch_Invalid(t *testing.T) {
	// 空字符串应返回错误（parts 长度为 1，补齐到 3 段后 Atoi 失败视为 0）
	// 实际上空字符串会得到 [""]，补齐到 ["", "", ""]，Atoi("") 失败视为 0，结果 "0.0.1"
	got, err := bumpPatch("")
	if err != nil {
		t.Errorf("bumpPatch(\"\") 返回错误: %v", err)
	}
	if got != "0.0.1" {
		t.Errorf("bumpPatch(\"\") = %q, 期望 %q", got, "0.0.1")
	}
}
