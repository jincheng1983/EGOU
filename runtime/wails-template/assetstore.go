package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// assetExternalDir 返回外部资源目录（开发模式热更新用）。
// 优先读环境变量 EG_PROJECT_PATH 指向的项目目录下的 assets/。
// 未设置或不存在时返回空字符串。
func assetExternalDir() string {
	pp := os.Getenv("EG_PROJECT_PATH")
	if pp == "" {
		return ""
	}
	dir := filepath.Join(pp, "assets")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}

// LoadAsset 从嵌入资源中读取指定路径的原始字节。
// path 是相对 assets/ 目录的路径（正斜杠分隔），如 "images/logo.png"。
// 查找顺序：1. 嵌入资源（embeddedFiles）2. 外部目录（EG_PROJECT_PATH/assets/，开发模式热更新）。
// 找不到时返回 nil + error。供用户代码中 载入资源(path) 调用。
func LoadAsset(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("资源路径为空")
	}
	// 统一用正斜杠，忽略前导 ./ 或 /
	path = strings.TrimLeft(path, "./")
	// 1. 嵌入资源
	if data, ok := embeddedFiles[path]; ok {
		return data, nil
	}
	// 2. 外部目录（P5 热更新：开发模式下从项目 assets/ 读取最新文件）
	if dir := assetExternalDir(); dir != "" {
		// 路径分隔符转平台本地格式
		localPath := filepath.Join(dir, filepath.FromSlash(path))
		if data, err := os.ReadFile(localPath); err == nil {
			return data, nil
		}
	}
	return nil, errors.New("资源不存在: " + path)
}

// ReadAssetText 从嵌入资源中读取指定路径的文本内容。
// 与 LoadAsset 相同，但返回字符串。供用户代码中 读资源文本(path) 调用。
func ReadAssetText(path string) (string, error) {
	data, err := LoadAsset(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListAssets 返回所有资源文件的路径列表（已排序）。
// 合并嵌入资源和外部目录（EG_PROJECT_PATH/assets/）下的文件，去重。
// 路径为相对 assets/ 目录的正斜杠格式。供用户代码中 列举资源() 调用。
func ListAssets() []string {
	set := make(map[string]struct{}, len(embeddedFiles))
	for k := range embeddedFiles {
		set[k] = struct{}{}
	}
	// P5：合并外部目录资源
	if dir := assetExternalDir(); dir != "" {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return nil
			}
			set[filepath.ToSlash(rel)] = struct{}{}
			return nil
		})
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ListWindows 返回所有嵌入窗口设计文件的名称列表（已排序）。
// 名称不含 .ew 扩展名。供用户代码中 列举窗口() 调用。
func ListWindows() []string {
	out := make([]string, 0, len(embeddedWindows))
	for k := range embeddedWindows {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// HasAsset 判断指定路径的资源是否存在。
// 检查嵌入资源和外部目录（P5 热更新）。供用户代码中 资源是否存在(path) 调用。
func HasAsset(path string) bool {
	if path == "" {
		return false
	}
	path = strings.TrimLeft(path, "./")
	if _, ok := embeddedFiles[path]; ok {
		return true
	}
	if dir := assetExternalDir(); dir != "" {
		localPath := filepath.Join(dir, filepath.FromSlash(path))
		if _, err := os.Stat(localPath); err == nil {
			return true
		}
	}
	return false
}


