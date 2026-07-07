// libs.go 实现支持库/扩展包（.elib）管理：扫描全局库、创建/重命名/删除项目内 .elib。
//
// 第七版对应方法直接迁移，仅按第八版命名规约重命名：
//   - .nlib → .elib
//   - source.nxg → source.eg
//   - CreateNlib → CreateElib
//   - DeleteNlib → DeleteElib
//   - RenameNlib → RenameElib
//   - "NxEGOU 扩展包" → "EGOU 扩展包"

package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GetGlobalLibsDir 返回 exe 同级的 libs 目录路径（IDE 全局生态目录）。
// 该目录下的 .elib 扩展包所有项目共享。目录不存在时返回空字符串。
func (s *IDEService) GetGlobalLibsDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Join(filepath.Dir(exePath), "libs")
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

// GlobalLibEntry 表示一个全局 .elib 的摘要信息，供前端 SupportPanel 展示。
type GlobalLibEntry struct {
	Dir           string `json:"dir"`
	Path          string `json:"path"`
	Name          string `json:"name"`
	DisplayName   string `json:"displayName"`
	Version       string `json:"version"`
	Description   string `json:"description"`
	Author        string `json:"author"`
	CommandCount  int    `json:"commandCount"`
	Commands      []any  `json:"commands,omitempty"`
}

// ScanGlobalLibs 扫描 exe 同级 libs/ 目录下的所有 .elib 扩展包，
// 返回每个包的元数据 + commands.json 内容。供前端合并到 SupportPanel。
func (s *IDEService) ScanGlobalLibs() []GlobalLibEntry {
	dir := s.GetGlobalLibsDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []GlobalLibEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkgDir := filepath.Join(dir, e.Name())
		cmdsPath := filepath.Join(pkgDir, "commands.json")
		cmdsData, err := os.ReadFile(cmdsPath)
		if err != nil {
			continue
		}
		var parsed struct {
			Library            string `json:"library"`
			LibraryDisplayName string `json:"libraryDisplayName"`
			Description        string `json:"description"`
			Author             string `json:"author"`
			LibraryVersion     string `json:"libraryVersion"`
			Commands           []any  `json:"commands"`
		}
		if err := json.Unmarshal(cmdsData, &parsed); err != nil {
			continue
		}
		// 尝试读 package.json 补充元数据
		pkgPath := filepath.Join(pkgDir, "package.json")
		var pkgMeta struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Author      string `json:"author"`
		}
		if pkgData, err := os.ReadFile(pkgPath); err == nil {
			_ = json.Unmarshal(pkgData, &pkgMeta)
		}
		name := parsed.Library
		if name == "" {
			name = pkgMeta.Name
		}
		if name == "" {
			name = e.Name()
		}
		displayName := parsed.LibraryDisplayName
		if displayName == "" {
			displayName = parsed.Library
		}
		if displayName == "" {
			displayName = name
		}
		version := parsed.LibraryVersion
		if version == "" {
			version = pkgMeta.Version
		}
		if version == "" {
			version = "0.0.0"
		}
		description := parsed.Description
		if description == "" {
			description = pkgMeta.Description
		}
		author := parsed.Author
		if author == "" {
			author = pkgMeta.Author
		}
		out = append(out, GlobalLibEntry{
			Dir:          e.Name(),
			Path:         pkgDir,
			Name:         name,
			DisplayName:  displayName,
			Version:      version,
			Description:  description,
			Author:       author,
			CommandCount: len(parsed.Commands),
			Commands:     parsed.Commands,
		})
	}
	return out
}

// CreateElib 在项目 libs 目录下创建一个 .elib 扩展包骨架。
// 包含 package.json + commands.json（含一个示例命令）+ source.eg（示例实现）。
// 成功返回创建的目录绝对路径，出错返回空字符串。
func (s *IDEService) CreateElib(projectPath string, name string) string {
	if projectPath == "" || name == "" {
		return ""
	}
	pkgDir := filepath.Join(projectPath, "libs", name)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return ""
	}
	pkgJSON := fmt.Sprintf(`{
    "name": "%s",
    "version": "0.1.0",
    "description": "EGOU 扩展包",
    "author": ""
}`, name)
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		return ""
	}
	cmdsJSON := fmt.Sprintf(`{
    "library": "%s",
    "libraryDisplayName": "%s",
    "libraryVersion": "0.1.0",
    "description": "示例扩展包",
    "commands": [
        {
            "commandId": "%s.HelloElib",
            "displayName": "示例命令",
            "englishName": "HelloElib",
            "summary": "示例命令：返回一句问候文本。",
            "params": [
                { "name": "名字", "type": "文本型", "optional": false, "description": "要问候的名字" }
            ],
            "returnType": "文本型",
            "category": "示例",
            "platforms": ["windows"],
            "callSyntax": "示例命令(名字)"
        }
    ]
}`, name, name, name)
	if err := os.WriteFile(filepath.Join(pkgDir, "commands.json"), []byte(cmdsJSON), 0644); err != nil {
		return ""
	}
	src := fmt.Sprintf(`# 程序集 %s

函数 HelloElib(参数 名字 文本型) 文本型
    返回 "你好，" + 名字 + "！来自扩展包 %s。"
结束函数
`, name, name)
	if err := os.WriteFile(filepath.Join(pkgDir, "source.eg"), []byte(src), 0644); err != nil {
		return ""
	}
	return pkgDir
}

// DeleteElib 删除项目 libs 目录下的指定扩展包。
// 成功返回空字符串，失败返回错误信息。
func (s *IDEService) DeleteElib(projectPath string, name string) string {
	if projectPath == "" || name == "" {
		return "参数为空"
	}
	pkgDir := filepath.Join(projectPath, "libs", name)
	if _, err := os.Stat(pkgDir); err != nil {
		if os.IsNotExist(err) {
			return "扩展包不存在"
		}
		return err.Error()
	}
	if err := os.RemoveAll(pkgDir); err != nil {
		return err.Error()
	}
	return ""
}

// RenameElib 重命名项目 libs 目录下的扩展包。
// 成功返回新的目录绝对路径，失败返回空字符串。
func (s *IDEService) RenameElib(projectPath string, oldName string, newName string) string {
	if projectPath == "" || oldName == "" || newName == "" {
		return ""
	}
	if oldName == newName {
		return filepath.Join(projectPath, "libs", newName)
	}
	oldDir := filepath.Join(projectPath, "libs", oldName)
	newDir := filepath.Join(projectPath, "libs", newName)
	if _, err := os.Stat(oldDir); err != nil {
		return ""
	}
	if _, err := os.Stat(newDir); err == nil {
		return ""
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return ""
	}
	return newDir
}
