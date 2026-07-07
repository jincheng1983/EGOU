// plugins.go 实现 IDE 插件管理：扫描插件目录、读取插件文件。
//
// 第七版对应方法直接迁移，无需重命名（插件目录名与命名规约无关）。

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// PluginEntry 表示一个 IDE 插件的摘要信息，供前端插件管理器和加载器使用。
type PluginEntry struct {
	Dir         string `json:"dir"`         // 插件目录名
	Path        string `json:"path"`        // 插件目录绝对路径
	Name        string `json:"name"`        // 插件名（package.json name）
	Version     string `json:"version"`     // 版本
	Author      string `json:"author"`      // 作者
	Description string `json:"description"` // 描述
	Main        string `json:"main"`        // 主入口文件（相对插件目录，如 "main.js"）
	Enabled     bool   `json:"enabled"`     // 是否启用（暂未实现禁用，默认 true）
}

// ScanPlugins 扫描 exe 同级 plugins/ 目录下的所有 IDE 插件。
// 每个子目录是一个插件，需包含 package.json 元数据文件。
// 供前端启动时加载插件 main.js。
func (s *IDEService) ScanPlugins() []PluginEntry {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	pluginsRoot := filepath.Join(filepath.Dir(exePath), "plugins")
	entries, err := os.ReadDir(pluginsRoot)
	if err != nil {
		return nil
	}
	var out []PluginEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginDir := filepath.Join(pluginsRoot, e.Name())
		pkgPath := filepath.Join(pluginDir, "package.json")
		pkgData, err := os.ReadFile(pkgPath)
		if err != nil {
			continue
		}
		var pkg struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Author      string `json:"author"`
			Description string `json:"description"`
			Main        string `json:"main"`
		}
		if err := json.Unmarshal(pkgData, &pkg); err != nil {
			continue
		}
		name := pkg.Name
		if name == "" {
			name = e.Name()
		}
		main := pkg.Main
		if main == "" {
			main = "main.js"
		}
		out = append(out, PluginEntry{
			Dir:         e.Name(),
			Path:        pluginDir,
			Name:        name,
			Version:     pkg.Version,
			Author:      pkg.Author,
			Description: pkg.Description,
			Main:        main,
			Enabled:     true,
		})
	}
	return out
}

// ReadPluginFile 读取插件目录下的文件内容，返回字符串。
// pluginName 是插件目录名，fileName 是相对插件目录的文件路径（如 "main.js"）。
// 用于前端加载插件 main.js 和其他资源文件。
func (s *IDEService) ReadPluginFile(pluginName, fileName string) string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	filePath := filepath.Join(filepath.Dir(exePath), "plugins", pluginName, fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return string(data)
}
