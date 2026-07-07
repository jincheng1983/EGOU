// file.go 实现文件操作相关方法：打开/保存/读取/删除文件、列出项目目录树。
//
// 第七版对应方法直接迁移，仅按第八版命名规约重命名：
//   - "NxEGOU 源码 (*.nxg)" → "EGOU 源码 (*.eg)"
//   - "NxEGOU 窗口 (*.nxw)" → "EGOU 窗口 (*.ew)"
//   - "打开 .nxg 文件" → "打开 .eg 文件"

package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// FileResponse 返回通过系统对话框选择的文件内容。
type FileResponse struct {
	Name    string `json:"name,omitempty"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// SaveResponse 返回保存操作结果。
type SaveResponse struct {
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

// FileNode 表示项目目录树中的一个节点。
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Children []*FileNode `json:"children,omitempty"`
}

// OpenFile 弹出系统"打开文件"对话框，返回选中文件的名称与内容。
func (s *IDEService) OpenFile() FileResponse {
	if s.app == nil {
		return FileResponse{Error: "应用实例未初始化"}
	}
	path, err := s.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		CanChooseFiles:       true,
		CanChooseDirectories: false,
		Title:                "打开 .eg 文件",
		Filters: []application.FileFilter{
			{DisplayName: "EGOU 源码 (*.eg)", Pattern: "*.eg"},
		},
	}).PromptForSingleSelection()
	if err != nil {
		return FileResponse{Error: err.Error()}
	}
	if path == "" {
		return FileResponse{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return FileResponse{Error: err.Error()}
	}
	return FileResponse{Name: filepath.Base(path), Path: path, Content: string(data)}
}

// PickFilePath 弹出系统"打开文件"对话框，只返回选中文件的路径（不读内容）。
// 用于图片选择器、资源选择等场景，title 为对话框标题，filter 为过滤字符串
// 格式："显示名|*.ext1;*.ext2|显示名2|*.ext3"，空则不过滤。
func (s *IDEService) PickFilePath(title string, filter string) string {
	if s.app == nil {
		return ""
	}
	opts := application.OpenFileDialogOptions{
		CanChooseFiles:       true,
		CanChooseDirectories: false,
		Title:                title,
	}
	if filter != "" {
		// 解析 "显示名1|*.ext1;*.ext2|显示名2|*.ext3" 格式
		parts := strings.Split(filter, "|")
		for i := 0; i+1 < len(parts); i += 2 {
			displayName := parts[i]
			pattern := parts[i+1]
			opts.Filters = append(opts.Filters, application.FileFilter{
				DisplayName: displayName,
				Pattern:     pattern,
			})
		}
	}
	path, err := s.app.Dialog.OpenFileWithOptions(&opts).PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return path
}

// QuickSave 按已有路径直接保存，不弹出对话框。
func (s *IDEService) QuickSave(path string, content string) SaveResponse {
	if path == "" {
		return SaveResponse{Error: "未指定保存路径"}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return SaveResponse{Error: err.Error()}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return SaveResponse{Error: err.Error()}
	}
	return SaveResponse{Path: path}
}

// SaveFile 弹出系统"保存文件"对话框，将内容写入用户选择的路径。
func (s *IDEService) SaveFile(name string, content string, title string) SaveResponse {
	if s.app == nil {
		return SaveResponse{Error: "应用实例未初始化"}
	}
	if title == "" {
		title = "保存文件"
	}
	path, err := s.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    title,
		Filename: name,
		Filters: []application.FileFilter{
			{DisplayName: "EGOU 源码 (*.eg)", Pattern: "*.eg"},
			{DisplayName: "EGOU 窗口 (*.ew)", Pattern: "*.ew"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	}).PromptForSingleSelection()
	if err != nil {
		return SaveResponse{Error: err.Error()}
	}
	if path == "" {
		return SaveResponse{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return SaveResponse{Error: err.Error()}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return SaveResponse{Error: err.Error()}
	}
	return SaveResponse{Path: path}
}

// ReadProjectFile 读取项目中的指定文件。
func (s *IDEService) ReadProjectFile(path string) FileResponse {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileResponse{Error: err.Error()}
	}
	return FileResponse{Name: filepath.Base(path), Content: string(data)}
}

// DeleteFile 删除指定文件。
func (s *IDEService) DeleteFile(path string) SaveResponse {
	if path == "" {
		return SaveResponse{Error: "未指定文件路径"}
	}
	if err := os.Remove(path); err != nil {
		return SaveResponse{Error: err.Error()}
	}
	return SaveResponse{Path: path}
}

// ListProjectDir 递归列出项目目录结构。
// 跳过 .git / bin / node_modules 等无关目录。
func (s *IDEService) ListProjectDir(path string) []*FileNode {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var nodes []*FileNode
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == "bin" || name == "node_modules" {
			continue
		}
		childPath := filepath.Join(path, name)
		node := &FileNode{
			Name:  name,
			Path:  childPath,
			IsDir: entry.IsDir(),
		}
		if entry.IsDir() {
			node.Children = s.ListProjectDir(childPath)
		}
		nodes = append(nodes, node)
	}
	return nodes
}
