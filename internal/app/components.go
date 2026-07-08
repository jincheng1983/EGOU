// components.go 实现外置组件库扫描：扫描 exe 同级 components/ 目录下的组件包，
// 供窗口设计器加载外置组件（P3 插件窗口设计器组件）。
//
// 组件包目录结构：
//   <exe目录>/components/<包名>/
//     package.json              组件包元数据 { name, version, author, description }
//     components/
//       <组件名>/
//         config.json           组件配置（type/label/icon/默认尺寸/属性schema/事件）
//         icon.svg              组件图标（可选，config.json icon 字段引用）
//
// 与插件（plugins/）的区别：
//   - 插件提供命令/菜单/补全等 IDE 扩展，通过 main.js activate(api) 注册
//   - 组件包只提供窗口设计器组件，通过 config.json 声明式注册
//   - 两者目录独立，互不依赖

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// ComponentPackage 表示一个外置组件包的摘要信息。
type ComponentPackage struct {
	Dir         string `json:"dir"`         // 组件包目录名（如 "my-components"）
	Path        string `json:"path"`        // 组件包目录绝对路径
	Name        string `json:"name"`        // 包名（package.json name）
	Version     string `json:"version"`     // 版本
	Author      string `json:"author"`      // 作者
	Description string `json:"description"` // 描述
	Components  []ComponentDef `json:"components"` // 包含的组件列表
}

// ComponentDef 表示一个外置组件的定义，供窗口设计器工具箱和属性面板使用。
type ComponentDef struct {
	Type        string            `json:"type"`         // 组件类型标识（如 "datepicker"），必须全 IDE 唯一
	Label       string            `json:"label"`        // 工具箱显示名（如 "日期选择器"）
	Icon        string            `json:"icon"`         // 图标文件名（相对组件目录，如 "icon.svg"），空表示用默认图标
	Width       int               `json:"width"`        // 默认宽度
	Height      int               `json:"height"`       // 默认高度
	Text        string            `json:"text"`         // 默认文本（可选）
	Props       []ComponentPropSchema `json:"props"`     // 属性 schema 列表（驱动属性面板）
	Events      []string          `json:"events"`       // 事件名列表（如 ["值被改变"]）
	PackageDir  string            `json:"packageDir"`   // 所属组件包目录名（前端加载图标用）
	Preview     *ComponentPreview `json:"preview,omitempty"` // 预览渲染配置（G9 完善）
}

// ComponentPreview 描述外置组件在设计器中的预览渲染方式。
// HTML 是模板字符串，支持 {{propName}} 占位符（运行时替换为属性值）。
// 例如：<input type="text" value="{{value}}" placeholder="{{format}}" />
type ComponentPreview struct {
	HTML string `json:"html"` // 预览 HTML 模板（支持 {{propName}} 占位符）
}

// ComponentPropSchema 描述组件单个属性的元数据，驱动属性面板动态渲染控件。
// 与 WindowDesigner.vue 中 propSchemas 的 schema 项结构一致。
type ComponentPropSchema struct {
	Key      string             `json:"key"`      // 属性键名（如 "format"）
	Label    string             `json:"label"`    // 中文显示标签（如 "格式"）
	Type     string             `json:"type"`     // 控件类型：select/number/text/bool/color/font/image
	Default  interface{}        `json:"default"`  // 默认值
	Options  []ComponentPropOption `json:"options,omitempty"` // 仅 select 类型
	Min      *float64           `json:"min,omitempty"`      // 仅 number 类型
	Max      *float64           `json:"max,omitempty"`      // 仅 number 类型
	Step     *float64           `json:"step,omitempty"`     // 仅 number 类型
	InputType string            `json:"inputType,omitempty"` // 仅 text 类型（如 "textarea"）
	Rows     int                `json:"rows,omitempty"`     // 仅 text 类型 textarea
}

// ComponentPropOption 是 select 类型属性的选项。
type ComponentPropOption struct {
	Label string `json:"label"` // 显示文本
	Value string `json:"value"` // 值
}

// ScanComponents 扫描 exe 同级 components/ 目录下的所有外置组件包。
// 每个组件包子目录需包含 package.json 元数据文件。
// 组件包内的 components/ 子目录下每个子目录是一个组件，需包含 config.json。
// 返回所有组件包及其组件列表，供前端窗口设计器加载。
func (s *IDEService) ScanComponents() []ComponentPackage {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	componentsRoot := filepath.Join(filepath.Dir(exePath), "components")
	entries, err := os.ReadDir(componentsRoot)
	if err != nil {
		return nil
	}
	var out []ComponentPackage
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkgDir := filepath.Join(componentsRoot, e.Name())
		pkg := scanComponentPackage(pkgDir, e.Name())
		if pkg != nil && len(pkg.Components) > 0 {
			out = append(out, *pkg)
		}
	}
	return out
}

// scanComponentPackage 扫描单个组件包，读取 package.json + components/*/config.json。
func scanComponentPackage(pkgPath, dirName string) *ComponentPackage {
	// 1. 读取 package.json
	pkgData, err := os.ReadFile(filepath.Join(pkgPath, "package.json"))
	if err != nil {
		return nil
	}
	var pkgMeta struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Author      string `json:"author"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(pkgData, &pkgMeta); err != nil {
		return nil
	}
	name := pkgMeta.Name
	if name == "" {
		name = dirName
	}

	pkg := &ComponentPackage{
		Dir:         dirName,
		Path:        pkgPath,
		Name:        name,
		Version:     pkgMeta.Version,
		Author:      pkgMeta.Author,
		Description: pkgMeta.Description,
	}

	// 2. 扫描 components/ 子目录下的组件
	componentsDir := filepath.Join(pkgPath, "components")
	compEntries, err := os.ReadDir(componentsDir)
	if err != nil {
		// 没有 components/ 子目录，返回空组件列表的包（前端可显示包信息）
		return pkg
	}

	var compDirs []string
	for _, ce := range compEntries {
		if ce.IsDir() {
			compDirs = append(compDirs, ce.Name())
		}
	}
	sort.Strings(compDirs)

	for _, compDirName := range compDirs {
		compDir := filepath.Join(componentsDir, compDirName)
		def := scanComponentDef(compDir, dirName)
		if def != nil {
			pkg.Components = append(pkg.Components, *def)
		}
	}
	return pkg
}

// scanComponentDef 读取单个组件的 config.json，解析为 ComponentDef。
func scanComponentDef(compDir, packageDir string) *ComponentDef {
	configPath := filepath.Join(compDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var def ComponentDef
	if err := json.Unmarshal(data, &def); err != nil {
		return nil
	}
	if def.Type == "" {
		return nil // type 是必填字段
	}
	if def.Label == "" {
		def.Label = def.Type
	}
	if def.Width <= 0 {
		def.Width = 80
	}
	if def.Height <= 0 {
		def.Height = 24
	}
	def.PackageDir = packageDir
	return &def
}

// ReadComponentFile 读取组件包内的文件内容，返回字符串。
// packageDir 是组件包目录名，fileName 是相对组件包目录的文件路径。
// 用于前端加载组件图标（如 "components/日期选择器/icon.svg"）。
func (s *IDEService) ReadComponentFile(packageDir, fileName string) string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	filePath := filepath.Join(filepath.Dir(exePath), "components", packageDir, fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return string(data)
}
