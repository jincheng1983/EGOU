# EGOU 组件库目录

本目录存放窗口设计器的外置组件库，每个子目录是一个组件包。
IDE 启动时由后端 `ScanComponents` 扫描，前端加载后注册到设计器工具箱。

## 目录结构（v0.8.1 已实现）

```
components/
└─ demo-components/              组件包目录
   ├─ package.json               组件包元数据 { name, version, author, description }
   └─ components/                组件定义子目录
      ├─ 日期选择器/
      │  ├─ config.json          组件配置（type/label/icon/默认尺寸/属性schema/事件）
      │  └─ icon.svg             组件图标（可选，未实现运行时加载）
      ├─ 树形框/
      │  └─ config.json
      └─ 颜色选择器/
         └─ config.json
```

## package.json 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 组件包名（显示用） |
| version | string | 版本号 |
| author | string | 作者 |
| description | string | 描述 |

## config.json 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| type | string | 组件类型标识（全 IDE 唯一，如 "datepicker"） |
| label | string | 工具箱显示名（如 "日期选择器"） |
| icon | string | 图标文件名（相对组件目录，留空用默认图标） |
| width | int | 默认宽度（像素） |
| height | int | 默认高度（像素） |
| text | string | 默认文本（可选） |
| props | array | 属性 schema 列表（驱动属性面板） |
| events | array | 事件名列表（如 ["值被改变"]） |

## 属性 schema（props 数组项）字段

| 字段 | 类型 | 说明 |
|------|------|------|
| key | string | 属性键名（如 "format"） |
| label | string | 中文显示标签（如 "格式"） |
| type | string | 控件类型：select/number/text/bool/color/font/image |
| default | any | 默认值 |
| options | array | 仅 select 类型：[{label, value}] |
| min/max/step | number | 仅 number 类型 |
| inputType | string | 仅 text 类型（如 "textarea"） |
| rows | int | 仅 text 类型 textarea |

## 与插件的区别

- **插件**（plugins/）：提供命令/菜单/补全等 IDE 扩展，通过 main.js `activate(api)` 注册
- **组件包**（components/）：只提供窗口设计器组件，通过 config.json 声明式注册
- 两者目录独立，互不依赖

## 已知限制（v0.8.1）

- 外置组件在画布中用通用占位框渲染（虚线边框 + 类型标签），不渲染真实控件
- 外置组件的代码生成（转译为 Go 代码）尚未实现
- 图标运行时加载（icon.svg）尚未实现，统一用默认图标
