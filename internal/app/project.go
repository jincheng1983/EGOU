// project.go 实现项目管理相关方法：打开/创建项目、读写项目配置、版本递增、源码收集。
//
// 项目模板管理见 template.go（v0.6.12 拆分，遵守单文件不超过 500 行规约）。
//
// 第七版对应方法直接迁移，仅按第八版命名规约重命名：
//   - main.nxg → main.eg
//   - project.nxg.json → project.eg.json
//   - 启动窗口.nxw → 启动窗口.ew
//   - source.nxg → source.eg
//   - "#@nxg-file" → "#@eg-file"
//   - "NxEGOU" → "EGOU"

package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ProjectConfig 表示 project.eg.json 的项目配置。
type ProjectConfig struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	Entry        string   `json:"entry"`
	Output       string   `json:"output"`
	SDK          string   `json:"sdk"`
	Dependencies []string `json:"dependencies"`
	// 编译产物元信息（影响 .syso 资源中的版本信息）
	IconPath        string `json:"iconPath,omitempty"`        // 自定义图标文件路径（.ico），留空用模板默认图标
	CompanyName     string `json:"companyName,omitempty"`     // 公司名称
	FileDescription string `json:"fileDescription,omitempty"` // 文件描述
	LegalCopyright  string `json:"legalCopyright,omitempty"`  // 版权信息
	ProductName     string `json:"productName,omitempty"`     // 产品名称
	Comments        string `json:"comments,omitempty"`        // 备注
}

// ===== P2-14 路径安全三件套（吸取 NxEGO3）=====

// validateProjectPath 防路径遍历：拒绝包含 ".." 的路径，拒绝空路径。
// 返回的 error 为 nil 表示路径安全。
func validateProjectPath(path string) error {
	if path == "" {
		return fmt.Errorf("项目路径不能为空")
	}
	// 检查路径遍历攻击
	if strings.Contains(path, "..") {
		return fmt.Errorf("项目路径不能包含 '..'")
	}
	// 清理路径后再检查一次
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("项目路径不能包含 '..'")
	}
	return nil
}

// validateProjectName 校验项目名合法性：
// - 拒绝 Windows 保留名（CON/PRN/AUX/NUL/COM1-9/LPT1-9）
// - 拒绝 Windows 非法字符（< > : " / \ | ? *）
// - 拒绝空名或纯空格
// - 拒绝以点开头（隐藏文件）
func validateProjectName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("项目名不能为空")
	}
	// Windows 保留名
	upper := strings.ToUpper(strings.TrimSpace(name))
	reserved := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true,
		"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
		"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	if reserved[upper] {
		return fmt.Errorf("项目名不能使用 Windows 保留名: %s", upper)
	}
	// 非法字符
	illegal := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	for _, ch := range illegal {
		if strings.Contains(name, ch) {
			return fmt.Errorf("项目名不能包含非法字符: %s", ch)
		}
	}
	// 不以点开头
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("项目名不能以点开头")
	}
	return nil
}

// OpenProject 弹出系统"打开文件夹"对话框，返回用户选择的项目目录路径。
func (s *IDEService) OpenProject() string {
	if s.app == nil {
		return ""
	}
	path, err := s.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		CanChooseFiles:       false,
		CanChooseDirectories: true,
		Title:                "打开项目目录",
	}).PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return path
}

// CreateProject 在指定父目录下创建新项目。
// template: "console"（控制台）/ "window"（窗口程序）/ 其他（空白）。
// 成功返回空字符串，失败返回错误信息。
//
// 项目目录结构（按项目规约第 5 章）：
//   项目名/
//   ├─ project.eg.json      项目配置
//   ├─ main.eg              主源码入口（根目录下，规约第 5 章）
//   ├─ 启动窗口.ew/.eg       窗口设计+代码（window 模板，根目录下）
//   ├─ .gitignore           本地仓库忽略规则（用户规约：自建本地仓库做灾难恢复）
//   ├─ bin/                 编译产物输出目录
//   ├─ assets/              资源表（声音/图片）
//   ├─ native/              Dll命令
//   ├─ modules/             模块
//   ├─ types/               类
//   ├─ libs/                扩展包（.elib）
//   └─ .eg/                 IDE 元数据（项目记忆/调试配置）
func (s *IDEService) CreateProject(parentPath string, name string, template string) string {
	// P2-14：路径安全三件套 — 创建项目前校验
	if err := validateProjectPath(parentPath); err != nil {
		return err.Error()
	}
	if err := validateProjectName(name); err != nil {
		return err.Error()
	}
	projectDir := filepath.Join(parentPath, name)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err.Error()
	}

	cfg := ProjectConfig{
		Name:         name,
		Version:      "1.0.0",
		Description:  "",
		Author:       "",
		Entry:        "main.eg",
		Output:       "bin",
		SDK:          "go1.22",
		Dependencies: []string{},
	}
	cfgData, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err.Error()
	}
	if err := os.WriteFile(filepath.Join(projectDir, "project.eg.json"), cfgData, 0644); err != nil {
		return err.Error()
	}

	// 创建完整目录骨架（规约第 5 章 7 个逻辑分类节点对应的目录）
	// 空目录通过 .gitkeep 占位，确保 git 能跟踪、项目树能显示分类节点
	for _, dir := range []string{"bin", "assets", "native", "modules", "types", "libs", ".eg"} {
		if err := os.MkdirAll(filepath.Join(projectDir, dir), 0755); err != nil {
			return err.Error()
		}
		// .gitkeep 让空目录能被 git 跟踪
		_ = os.WriteFile(filepath.Join(projectDir, dir, ".gitkeep"), []byte(""), 0644)
	}

	// .gitignore（用户规约：项目下自建本地仓库做灾难恢复）
	gitignoreContent := `# 编译产物
bin/
*.exe
*.syso

# IDE 元数据（项目记忆/调试配置）
.eg/

# 日志
*.log

# 临时文件
.eg-runtime-cache/
`
	if err := os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		return err.Error()
	}

	// 源码文件直接放在项目根目录（规约第 5 章：源码文件 = 项目根目录下的 .eg 文件）
	var mainSource string
	switch template {
	case "console":
		mainSource = `# 程序集 main

导入 (
    "fmt"
)

函数 主函数()
    fmt.Println("你好，世界！")
结束函数
`
	case "window":
		mainSource = `# 程序集 main

函数 主函数()
    加载窗口("启动窗口")
    打印("窗口程序已启动")
    消息循环()
结束函数
`
		formSource := `{"form":{"title":"启动窗口","width":538,"height":350,"bgColor":"#f0f0f0"},"components":[]}`
		codeSource := `# 程序集 启动窗口

函数 主函数()
结束函数
`
		if err := os.WriteFile(filepath.Join(projectDir, "启动窗口.ew"), []byte(formSource), 0644); err != nil {
			return err.Error()
		}
		if err := os.WriteFile(filepath.Join(projectDir, "启动窗口.eg"), []byte(codeSource), 0644); err != nil {
			return err.Error()
		}
	default:
		mainSource = `# 程序集 main

函数 主函数()
结束函数
`
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.eg"), []byte(mainSource), 0644); err != nil {
		return err.Error()
	}
	return ""
}

// ReadProjectConfig 读取项目根目录下的 project.eg.json。
func (s *IDEService) ReadProjectConfig(path string) ProjectConfig {
	configPath := filepath.Join(path, "project.eg.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ProjectConfig{}
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ProjectConfig{}
	}
	return cfg
}

// SaveProjectConfig 将项目配置写回 project.eg.json（保留未知字段）。
func (s *IDEService) SaveProjectConfig(projectPath string, cfg ProjectConfig) string {
	configPath := filepath.Join(projectPath, "project.eg.json")
	// 先读原文件以保留未知字段
	raw := map[string]interface{}{}
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	// 覆盖已知字段
	raw["name"] = cfg.Name
	raw["version"] = cfg.Version
	raw["description"] = cfg.Description
	raw["author"] = cfg.Author
	raw["entry"] = cfg.Entry
	raw["output"] = cfg.Output
	raw["sdk"] = cfg.SDK
	raw["dependencies"] = cfg.Dependencies
	if cfg.IconPath != "" {
		raw["iconPath"] = cfg.IconPath
	} else {
		delete(raw, "iconPath")
	}
	if cfg.CompanyName != "" {
		raw["companyName"] = cfg.CompanyName
	} else {
		delete(raw, "companyName")
	}
	if cfg.FileDescription != "" {
		raw["fileDescription"] = cfg.FileDescription
	} else {
		delete(raw, "fileDescription")
	}
	if cfg.LegalCopyright != "" {
		raw["legalCopyright"] = cfg.LegalCopyright
	} else {
		delete(raw, "legalCopyright")
	}
	if cfg.ProductName != "" {
		raw["productName"] = cfg.ProductName
	} else {
		delete(raw, "productName")
	}
	if cfg.Comments != "" {
		raw["comments"] = cfg.Comments
	} else {
		delete(raw, "comments")
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err.Error()
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return err.Error()
	}
	return ""
}

// IncrementProjectVersion 递增项目版本号的 patch 段（major.minor.patch → patch+1），
// 写回 project.eg.json，返回新版本号。失败时返回空字符串，不影响构建。
func (s *IDEService) IncrementProjectVersion(projectPath string) string {
	configPath := filepath.Join(projectPath, "project.eg.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	// 用 map 解析以保留其他字段
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	ver, _ := raw["version"].(string)
	if ver == "" {
		ver = "0.1.0"
	}
	newVer := incrementPatchVersion(ver)
	raw["version"] = newVer
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return ""
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return ""
	}
	return newVer
}

// incrementPatchVersion 将 "major.minor.patch" 的 patch 段 +1。
// 非法格式返回原字符串，不修改。
func incrementPatchVersion(ver string) string {
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		return ver
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return ver
	}
	parts[2] = strconv.Itoa(patch + 1)
	return strings.Join(parts, ".")
}

// collectProjectSources 读取项目入口（main.eg）以及项目下所有窗口/模块/类目录中的 .eg
// 源码，合并成一段源码字符串。重复的 `import`、`# 程序集 ...` 等声明会在转译阶段去重。
//
// 由 compile.go 的 RunProject/BuildProjectRelease/ExportStandaloneProject 调用。
func collectProjectSources(projectPath string) (string, error) {
	var mainSource string
	for _, candidate := range []string{
		filepath.Join(projectPath, "src", "main.eg"),
		filepath.Join(projectPath, "main.eg"),
	} {
		if data, err := os.ReadFile(candidate); err == nil {
			mainSource = string(data)
			break
		}
	}
	if mainSource == "" {
		return "", fmt.Errorf("找不到 main.eg 入口文件")
	}

	// 扫描项目目录树，收集所有其他 .eg 文件，插入到 main 之后。
	extraSources, err := walkProjectEg(projectPath)
	if err != nil {
		return "", err
	}
	if len(extraSources) == 0 {
		return "#@eg-file main.eg\n" + mainSource, nil
	}
	return "#@eg-file main.eg\n" + mainSource + "\n\n" + strings.Join(extraSources, "\n\n"), nil
}

// walkProjectEg 递归扫描项目目录，收集所有 .eg 源码（按目录名分类：窗口/模块/类/资源）。
// 资源目录（assets）下的 .eg 不参与编译，避免被误识别。
// 对于非入口 .eg 文件，会自动剥离 `# 程序集 ...` 与 `主函数()…结束函数` 段，避免
// 与入口文件重复定义 `mainImpl` / `package main`。
func walkProjectEg(projectPath string) ([]string, error) {
	// 跳过的目录名（资源目录、依赖目录、构建产物目录、IDE 元数据目录等）。
	skipDirs := map[string]bool{
		"assets": true, "asset": true, "资源": true,
		"bin": true, "build": true, "dist": true,
		"node_modules": true, ".git": true,
		".eg": true, // IDE 元数据目录（项目记忆/调试配置），不参与编译
	}
	var out []string
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[strings.ToLower(info.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".eg" {
			return nil
		}
		// 跳过入口 main.eg（已经被 collectProjectSources 单独加载）
		rel, _ := filepath.Rel(projectPath, path)
		rel = filepath.ToSlash(rel)
		if rel == "src/main.eg" || rel == "main.eg" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		cleaned := stripEntryDeclarations(string(data))
		if strings.TrimSpace(cleaned) == "" {
			return nil
		}
		out = append(out, fmt.Sprintf("#@eg-file %s\n%s", rel, cleaned))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// stripEntryDeclarations 从附加 .eg 源码中移除：
//   - `# 程序集 ...` 头注释（避免重复的 package 标识）
//   - 顶层的 `主函数() … 结束函数` 段（避免 mainImpl 重复定义）
//
// 它不会影响普通的 `函数 xxx_被单击() … 结束函数` 等事件处理函数与辅助函数。
func stripEntryDeclarations(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	// skipDepth 含义：
	//   0  = 正常输出
	//  -1  = 正在跳过 `导入 (...)` 块，遇到 `)` 退出
	//   1  = 正在跳过 `主函数() … 结束函数` 段，遇到 `结束函数` 退出
	skipDepth := 0
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		switch skipDepth {
		case 0:
			// 跳过 `# 程序集 ...` 头部注释行
			if strings.HasPrefix(trim, "#") && strings.Contains(trim, "程序集") {
				continue
			}
			// 跳过 `导入 ()` 整段（合并到主文件时由入口文件统一提供）
			if trim == "导入 (" {
				skipDepth = -1
				continue
			}
			// 跳过 `主函数()` 顶层入口段
			if trim == "函数 主函数()" || trim == "主函数()" {
				skipDepth = 1
				continue
			}
			out = append(out, line)
		case -1:
			// 导入块结束
			if trim == ")" {
				skipDepth = 0
			}
		case 1:
			// 主函数段结束
			if trim == "结束函数" {
				skipDepth = 0
			}
		}
	}
	return strings.Join(out, "\n")
}
