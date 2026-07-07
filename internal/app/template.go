// template.go 实现项目模板管理：扫描/创建/保存/删除全局项目模板。
//
// 全局模板存放在 exe 同级 templates/ 目录，每个子目录是一个模板，含 template.json 元数据。
// 供前端新建项目对话框合并到模板选项列表。
//
// 从 project.go 拆分（v0.6.12 单文件不超过 500 行规约）。

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// GlobalTemplateEntry 表示一个全局项目模板的摘要信息，供前端新建项目对话框展示。
type GlobalTemplateEntry struct {
	Dir         string `json:"dir"`         // 模板目录名（作为 template key）
	Path        string `json:"path"`        // 模板目录绝对路径
	Name        string `json:"name"`        // 模板显示名（如 "OpenGL 图形程序"）
	Description string `json:"description"` // 模板描述
	Icon        string `json:"icon"`        // 模板图标（emoji 或图标名，可选）
}

// ScanGlobalTemplates 扫描 exe 同级 templates/ 目录下的所有项目模板。
// 每个子目录是一个模板，需包含 template.json 元数据文件。
// 供前端新建项目对话框合并到模板选项列表。
func (s *IDEService) ScanGlobalTemplates() []GlobalTemplateEntry {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	tmplRoot := filepath.Join(filepath.Dir(exePath), "templates")
	entries, err := os.ReadDir(tmplRoot)
	if err != nil {
		return nil
	}
	var out []GlobalTemplateEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tmplDir := filepath.Join(tmplRoot, e.Name())
		metaPath := filepath.Join(tmplDir, "template.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		}
		if err := json.Unmarshal(metaData, &meta); err != nil {
			continue
		}
		name := meta.Name
		if name == "" {
			name = e.Name()
		}
		out = append(out, GlobalTemplateEntry{
			Dir:         e.Name(),
			Path:        tmplDir,
			Name:        name,
			Description: meta.Description,
			Icon:        meta.Icon,
		})
	}
	return out
}

// CreateProjectFromTemplate 从全局模板目录创建新项目。
// templateName 是模板目录名（exe 同级 templates/<templateName>），parentPath 是父目录，name 是项目名。
// 复制模板目录下所有文件到项目目录（排除 template.json），然后在项目目录写入 project.eg.json。
// 返回空字符串表示成功，非空表示错误信息。
func (s *IDEService) CreateProjectFromTemplate(templateName, parentPath, name string) string {
	// P2-14：路径安全三件套 — 从模板创建项目前校验
	if err := validateProjectPath(parentPath); err != nil {
		return err.Error()
	}
	if err := validateProjectName(name); err != nil {
		return err.Error()
	}
	exePath, err := os.Executable()
	if err != nil {
		return err.Error()
	}
	templateDir := filepath.Join(filepath.Dir(exePath), "templates", templateName)
	if _, err := os.Stat(templateDir); err != nil {
		return "模板不存在: " + templateName
	}
	projectDir := filepath.Join(parentPath, name)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err.Error()
	}
	// 复制模板目录下所有文件（排除 template.json）到项目目录
	err = filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		if rel == "template.json" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(projectDir, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		return err.Error()
	}
	// 如果项目目录没有 project.eg.json，生成默认配置
	cfgPath := filepath.Join(projectDir, "project.eg.json")
	if _, err := os.Stat(cfgPath); err != nil {
		cfg := ProjectConfig{
			Name:         name,
			Version:      "1.0.0",
			Entry:        "main.eg",
			Output:       "bin",
			SDK:          "go1.22",
			Dependencies: []string{},
		}
		cfgData, _ := json.MarshalIndent(cfg, "", "    ")
		_ = os.WriteFile(cfgPath, cfgData, 0644)
	}
	// 补全目录骨架（规约第 5 章 7 个逻辑分类节点对应的目录）
	// 模板可能没有包含所有目录，这里确保都存在
	for _, dir := range []string{"bin", "assets", "native", "modules", "types", "libs", ".eg"} {
		dirPath := filepath.Join(projectDir, dir)
		if _, err := os.Stat(dirPath); err != nil {
			_ = os.MkdirAll(dirPath, 0755)
			_ = os.WriteFile(filepath.Join(dirPath, ".gitkeep"), []byte(""), 0644)
		}
	}
	// 补全 .gitignore（如果模板没有提供）
	gitignorePath := filepath.Join(projectDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err != nil {
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
		_ = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
	}
	return ""
}

// SaveProjectAsTemplate 把当前项目另存为全局项目模板（P6）。
// 复制项目文件到 exe 同级 templates/<templateName>/，并生成 template.json 元数据。
// 排除产物目录（bin/build/dist）、node_modules、.git、libs、native、runtime-frontend 等。
// 成功返回空字符串，失败返回错误信息。
func (s *IDEService) SaveProjectAsTemplate(projectPath, templateName, description, icon string) string {
	if projectPath == "" || templateName == "" {
		return "项目路径或模板名为空"
	}
	exePath, err := os.Executable()
	if err != nil {
		return err.Error()
	}
	templateDir := filepath.Join(filepath.Dir(exePath), "templates", templateName)
	// 模板已存在时先删除，实现"覆盖保存"
	if _, err := os.Stat(templateDir); err == nil {
		_ = os.RemoveAll(templateDir)
	}
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return err.Error()
	}
	// 排除的目录名（小写匹配）
	skipDirs := map[string]bool{
		"bin": true, "build": true, "dist": true,
		"node_modules": true, ".git": true, "libs": true,
		"native": true, "runtime-frontend": true, ".trae": true,
	}
	err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[strings.ToLower(info.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(projectPath, path)
		if err != nil {
			return nil
		}
		target := filepath.Join(templateDir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		return err.Error()
	}
	// 生成 template.json 元数据
	tmplMeta := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}{
		Name:        templateName,
		Description: description,
		Icon:        icon,
	}
	metaData, _ := json.MarshalIndent(tmplMeta, "", "  ")
	if len(metaData) > 0 && metaData[len(metaData)-1] != '\n' {
		metaData = append(metaData, '\n')
	}
	_ = os.WriteFile(filepath.Join(templateDir, "template.json"), metaData, 0644)
	return ""
}

// DeleteGlobalTemplate 删除 exe 同级 templates/ 下的指定模板。
// 成功返回空字符串，失败返回错误信息。
func (s *IDEService) DeleteGlobalTemplate(templateName string) string {
	if templateName == "" {
		return "模板名为空"
	}
	exePath, err := os.Executable()
	if err != nil {
		return err.Error()
	}
	templateDir := filepath.Join(filepath.Dir(exePath), "templates", templateName)
	if _, err := os.Stat(templateDir); err != nil {
		return "模板不存在: " + templateName
	}
	if err := os.RemoveAll(templateDir); err != nil {
		return err.Error()
	}
	return ""
}
