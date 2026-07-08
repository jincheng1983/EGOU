# EGOU IDE UI 视觉规范（第八版）

> 最后更新：2026-07-08（v0.10.1）
> 配套文档：[design.md](design.md) / [optimization_plan.md](optimization_plan.md)
> 单一数据源：[frontend/src/themes.js](../frontend/src/themes.js)

---

## 1. 设计理念

### 1.1 风格定位

- **磨砂玻璃（Glassmorphism）**：半透明 rgba 背景（0.75-0.85 alpha）+ `backdrop-filter: blur(12px) saturate(1.2)`
- **中性圆角**：5 档圆角分级，避免锐利直角
- **快速过渡**：统一 0.15s，无冗长动画
- **强调色驱动**：激活态、高光线、焦点环统一使用 `var(--accent-color)`

### 1.2 设计原则

| 原则 | 说明 |
|---|---|
| 单一数据源 | themes.js 是唯一主题数据源，style.css 仅作初始加载回退 |
| CSS 变量强制 | 所有颜色必须用 `var(--xxx)`，禁止硬编码 hex/rgba |
| 主题自适应 | 同一套 CSS 适配 4 套主题，无需写主题特定样式 |
| 空间节约 | 去除重复大标题，搜索栏与按钮同行，内容区适配宽度 |
| 原生按钮优先 | 图标按钮用原生 `<button>`，避免 Naive UI 内部样式干扰 |

---

## 2. 4 套主题

| 主题 key | 中文标签 | isDark | 主背景 `--bg-primary` | 强调色 `--accent-color` | 文字主色 `--text-primary` |
|---|---|---|---|---|---|
| `dark` | 深色 | true | `rgba(30, 30, 30, 0.85)` | `#63e2b7`（薄荷绿） | `#e0e0e0` |
| `light` | 浅色 | false | `rgba(255, 255, 255, 0.88)` | `#18a058`（Naive 绿） | `#1e293b` |
| `ocean` | 海蓝 | true | `rgba(8, 16, 24, 0.85)` | `#4facfe`（天蓝） | `#e6f4ff` |
| `sunrise` | 朝阳 | false | `rgba(255, 248, 240, 0.88)` | `#f97316`（橙色） | `#451a03` |

侧边栏背景 `--bg-sidebar`：
- dark: `rgba(25, 25, 25, 0.88)` / light: `rgba(248, 250, 252, 0.88)`
- ocean: `rgba(6, 14, 24, 0.85)` / sunrise: `rgba(255, 244, 230, 0.88)`

模态遮罩 `--modal-mask-bg`：dark/ocean `rgba(0,0,0,0.45)`；light `rgba(0,0,0,0.35)`；sunrise `rgba(0,0,0,0.4)`。

存储键：`localStorage['egou-theme']`；自定义主题键：`localStorage['egou-custom-themes']`。

---

## 3. CSS 变量分层

### 3.1 背景色阶（8 档）

| 变量名 | 用途 |
|---|---|
| `--bg-primary` | 主背景 |
| `--bg-secondary` | 次背景/侧边栏等 |
| `--bg-tertiary` | 第三背景/输入框等 |
| `--bg-hover` | hover 态 |
| `--bg-active` | active/selected 态 |
| `--bg-sidebar` | 侧边栏背景 |
| `--bg-output` | 输出面板背景 |
| `--bg-input` | 输入框背景 |

### 3.2 渐变背景（2 档）

| 变量名 | 用途 |
|---|---|
| `--toolbar-gradient` | 工具栏渐变（180deg） |
| `--card-gradient` | 卡片渐变（135deg） |

### 3.3 边框（3 档）

| 变量名 | 用途 |
|---|---|
| `--border-color` | 标准边框 |
| `--border-light` | 浅边框 |
| `--accent-border` | 强调色边框（含 alpha） |

### 3.4 文字色阶（6 档）

| 变量名 | 用途 |
|---|---|
| `--text-primary` | 主要内容 |
| `--text-secondary` | 次要内容 |
| `--text-muted` | 标签、说明 |
| `--text-dim` | 路径、备注 |
| `--text-faint` | 占位、底部状态 |
| `--text-darker` | 最低对比度元素 |

### 3.5 强调色（4 变体）

| 变量名 | 用途 |
|---|---|
| `--accent-color` | 主强调色 |
| `--accent-hover` | hover 态 |
| `--accent-light` | 亮色变体 |
| `--accent-bg` | 半透明背景（用于 active/hover 底色） |

### 3.6 语义色（5 种）

| 变量名 | 语义 |
|---|---|
| `--color-success` | 成功 |
| `--color-warning` | 警告 |
| `--color-error` | 错误 |
| `--color-info` | 信息 |
| `--color-purple` | 紫色（特殊标记） |

### 3.7 滚动条（2 档）

| 变量名 | 用途 |
|---|---|
| `--scrollbar-thumb` | 滚动条滑块 |
| `--scrollbar-hover` | 滚动条 hover |

滚动条样式：6px 宽，圆角 3px，半透明。

### 3.8 阴影（4 项）

| 变量名 | 用途 |
|---|---|
| `--shadow-color` | 阴影基色 |
| `--shadow-1` | 小阴影 `0 1px 3px` |
| `--shadow-2` | 中阴影 `0 4px 12px` |
| `--shadow-pop` | 弹层阴影 `0 8px 32px` |
| `--modal-mask-bg` | 模态遮罩色 |

### 3.9 圆角分级（5 档，不随主题变）

| 变量名 | 值 | 用途 |
|---|---|---|
| `--radius-sm` | 4px | 小元素：标签、输入框 |
| `--radius-md` | 6px | 中元素：按钮、卡片 |
| `--radius-lg` | 8px | 大元素：面板、下拉菜单 |
| `--radius-xl` | 12px | 超大：模态框、抽屉 |
| `--radius-2xl` | 16px | 容器：起始页、大卡片 |

### 3.10 不随主题变的全局常量（仅 style.css `:root`）

- 字体：`--ide-font`（`'IdeFont', system-ui, ...`）、`--ide-code-font`（`'IdeFont', 'Consolas', ...`）
- 能力徽章色（固定）：

| 变量名 | 值 | 用途 |
|---|---|---|
| `--cap-chat` | `#63b3ed` | chat 能力徽章 |
| `--cap-code_gen` | `#9f7aea` | code_gen 能力徽章 |
| `--cap-debug` | `#f56565` | debug 能力徽章 |
| `--cap-explain` | `#48bb78` | explain 能力徽章 |
| `--cap-plan` | `#ecc94b` | plan 能力徽章 |
| `--cap-ui_design` | `#ed89c1` | ui_design 能力徽章 |

### 3.11 旧变量名别名（向后兼容）

| 旧名 | 新名 |
|---|---|
| `--text-tertiary` | `--text-dim` |
| `--hover-color` | `--bg-hover` |
| `--primary-color` | `--accent-color` |
| `--text-color-3` | `--text-muted` |
| `--card-bg` | `--bg-secondary` |

---

## 4. 全局工具类

### 4.1 高光线

```css
.eg-accent-line {
  background: linear-gradient(90deg, transparent, var(--accent-color), transparent);
  height: 2px;
}
```

用于标题栏底部、激活态下划线。

### 4.2 磨砂玻璃

```css
.eg-glass {
  backdrop-filter: blur(12px) saturate(1.2);
}
```

### 4.3 3D 按钮

- `.eg-btn-3d`：通用 3D 按钮基底
- `.eg-btn-compile`：编译按钮
- `.eg-btn-run`：运行按钮

### 4.4 焦点环

```css
.eg-focus {
  box-shadow: 0 0 0 2px var(--accent-bg);
}
```

### 4.5 动画

| 类名 | 时长 | 用途 |
|---|---|---|
| `eg-pulse` | 1.5s | 脉冲（加载提示） |
| `eg-fade-in` | 0.2s | 淡入 |
| `eg-typing` | 1.2s | 打字机效果（AI 响应） |

### 4.6 全局过渡

```css
* { transition: background-color 0.15s, border-color 0.15s, color 0.15s; }
```

### 4.7 Naive UI 组件磨砂覆盖

以下 Naive UI 组件被全局磨砂样式覆盖：n-card / n-modal / n-drawer / n-input / n-button / n-tag / n-list / n-tabs / n-dropdown / n-message / n-tooltip / n-popover。

---

## 5. 公共颜色常量

来源：[frontend/src/utils/colors.js](../frontend/src/utils/colors.js)

### 5.1 FLOW_RAINBOW（标准彩虹色，7 色）

项目树 / 文件树使用。

| 索引 | 颜色名 | 值 |
|---|---|---|
| 0 | 红 | `#ff6b6b` |
| 1 | 橙 | `#ffa94d` |
| 2 | 黄 | `#ffd43b` |
| 3 | 绿 | `#69db7c` |
| 4 | 蓝 | `#4dabf7` |
| 5 | 紫 | `#9775fa` |
| 6 | 粉 | `#f783ac` |

### 5.2 FLOW_RAINBOW_DARK（深一档彩虹色，7 色）

支持库面板使用，与项目树区分层次。

| 索引 | 颜色名 | 值 |
|---|---|---|
| 0 | 深红 | `#e03131` |
| 1 | 深橙 | `#e8590c` |
| 2 | 深黄 | `#f08c00` |
| 3 | 深绿 | `#2f9e44` |
| 4 | 深蓝 | `#1971c2` |
| 5 | 深紫 | `#7048e8` |
| 6 | 深粉 | `#c2255c` |

### 5.3 TYPE_COLOR（项目树节点类型颜色，15 个键）

| key | 颜色 | 用途 |
|---|---|---|
| `source` / `srcfile` / `assembly` | `#5b8def`（蓝） | 源码文件/程序集 |
| `winassembly` / `window` | `#a371f7`（紫） | 窗口程序集/窗口 |
| `module` | `#1aab6a`（绿） | 模块 |
| `class` | `#e08e3f`（橙） | 类 |
| `func` / `method` | `#3aa0ff`（亮蓝） | 函数/方法 |
| `var` | `#d96b6d`（红粉） | 变量 |
| `constant` / `resource` | `#d2a23a`（金） | 常量/资源 |
| `dll` | `#bf6ad1`（紫红） | Dll 命令 |
| `sound` | `#e05a5a`（红） | 声音资源 |
| `image` | `#4dabf7`（蓝） | 图片资源 |
| `libref` | `#9775fa`（紫） | 模块引用（.elib） |

### 5.4 GROUP_COLORS（组件分组标识色，8 色）

窗口设计器使用，同组组件共享虚线 outline。

```
['#ff6b6b', '#4dabf7', '#51cf66', '#fcc419', '#9775fa', '#ff922b', '#20c997', '#f06595']
```

依次为：红、蓝、绿、黄、紫、橙、青、粉。

---

## 6. 布局规范

### 6.1 3 栏布局

```
┌─────────────────────────────────────────────┐
│              TitleBar（40px）                 │
├──────┬──────────────────────────────┬───────┤
│Left  │       Editor Stack           │ Right │
│Menu  │       (flex:1)               │ 280px │
│48px  │                              │       │
│      │                              │       │
│240px │                              │       │
│      │                              │       │
├──────┴──────────────────────────────┴───────┤
│           Output Panel（160px）              │
├─────────────────────────────────────────────┤
│              Status Bar                      │
└─────────────────────────────────────────────┘
```

### 6.2 DOM 结构

```
.app-shell (flex column, height:100vh, 磨砂背景 var(--bg-primary))
├── <TitleBar />                              高度 40px
└── .main-container (flex:1, display:flex)
    └── .workspace (flex:1, flex column)
        ├── .workspace-body (flex:1, display:flex)
        │   ├── <LeftMenu />                  固定宽 48px
        │   ├── aside.left-panel              width: leftPanelWidth (默认 240px)
        │   ├── .splitter.splitter-v          左拖拽条
        │   ├── main.editor-stack (flex:1)    编辑器栈
        │   │   ├── .tabs-bar-wrapper > .file-tabs + .new-tab-btn
        │   │   └── .editor-area
        │   │       ├── .editor-main > <Editor />
        │   │       ├── .splitter.splitter-v（右拖拽条）
        │   │       └── aside.right-panel (280px)
        │   │       └── <PropertiesPanel /> (大纲/属性)
        │   │       └── <WindowDesigner /> (设计视图)
        │   └── (workspace-body 结束)
        ├── .output-panel (v-if !outputCollapsed && !zenMode)
        │   height: outputPanelHeight (默认 160px)
        │   ├── .splitter.splitter-h (顶部水平拖拽条)
        │   ├── n-tabs (6 标签)
        │   └── .output-actions (导出 / 清空)
        └── .status-bar (状态栏，左右两组)
```

### 6.3 尺寸与持久化

| 元素 | 默认尺寸 | 范围 | 持久化键 |
|---|---|---|---|
| 左侧面板 | 240px | 180~400px | `eg-left-width` |
| 右侧面板 | 280px | 200~500px | `eg-right-width` |
| 输出面板高度 | 160px | 80~400px | `eg-output-height` |
| 左侧菜单 | 48px | 固定 | — |
| 标题栏 | 40px | 固定 | — |
| 菜单项 | 36×36px | 固定 | — |
| 标题栏按钮 | 30×30px | 固定 | — |
| 窗口控制按钮 | 34×30px | 固定 | — |

### 6.4 拖拽器

`.splitter-v` / `.splitter-h`：视觉宽度 0px，通过 `::after` 伪元素扩展 ±4px 形成可点击热区。

### 6.5 特殊模式

- **禅模式（zenMode）**：隐藏右侧面板和输出面板，专注编码
- **输出折叠（outputCollapsed）**：隐藏输出面板，扩大编辑区

---

## 7. 标题栏（TitleBar）

### 7.1 布局

分 3 段：`.title-bar-left` / `.title-bar-center` / `.title-bar-right`。

- 背景：`var(--toolbar-gradient)` + `backdrop-filter: blur(12px) saturate(1.2)`
- 拖拽：`--wails-draggable: drag`（可拖拽移动窗口），按钮区域 `no-drag`
- 双击标题栏触发 `toggleMaximise`

### 7.2 左侧段

| # | 元素 | 图标 | 说明 |
|---|---|---|---|
| 1 | 应用图标 | — | `<img src="/appicon.png">` 22×22px |
| 2 | 应用名 | — | `<span class="app-name">易狗 IDE</span>` |
| 3 | 分隔线 | — | `<n-divider vertical>` |
| 4 | 保存 | `SaveOutline` | 无对话框，自动保存 |
| 5 | 另存为 | `Save` | 有对话框 |
| 6 | 撤销 | `ArrowUndoOutline` | — |
| 7 | 重做 | `ArrowRedoOutline` | — |

### 7.3 中间段

| # | 元素 | 图标 | 说明 |
|---|---|---|---|
| 8 | 编译运行 | `PlayOutline` | class 含 `run-btn`，强调色 `var(--accent-color)`，**仅图标无文字** |
| 9 | 生成可执行文件 | `BuildOutline` | 下拉菜单：生成可执行文件 / 编译选项 |
| 10 | 调试 | `BugOutline` | 启动/停止调试 |

### 7.4 右侧段

| # | 元素 | 图标 | 说明 |
|---|---|---|---|
| 11 | 关于 | `InformationCircleOutline` | — |
| 12 | 主题切换 | `SunnyOutline`/`MoonOutline` | 根据 `isDark` 切换图标 |
| 13 | 代码片段 | `CodeSlashOutline` | 代码片段管理 |
| 14 | 系统设置 | `SettingsOutline` | — |
| 15 | 分隔线 | — | `<n-divider vertical>` |
| 16 | 最小化 | `RemoveOutline` | class `window-ctrl`，宽 34px |
| 17 | 最大化/还原 | `ExpandOutline`/`SquareOutline` | class `window-ctrl`，宽 34px |
| 18 | 关闭 | `CloseOutline` | class `window-ctrl close`，hover 变 `var(--color-error)` |

### 7.5 规约

- **"编译运行" 仅图标**，无文字
- **图标按钮用原生 `<button>`**（不用 n-button，避免 Naive UI 内部样式干扰）
- **标题栏图标按钮无默认边框**

---

## 8. 左侧边栏（LeftMenu）

### 8.1 结构

宽 48px，分上下两组（`justify-content: space-between`）。

### 8.2 顶部组（4 标签 + 插件扩展）

固定顺序的 4 个标签：

| 顺序 | key | 标签 | 图标 |
|---|---|---|---|
| 1 | `files` | 文件 | `DocumentOutline` |
| 2 | `project` | 项目 | `FolderOutline` |
| 3 | `support` | 支持 | `LibraryOutline` |
| 4 | `ai` | AI | `SparklesOutline` |

之后追加 G7 插件自定义面板按钮（`v-for="panel in pluginPanels"`），icon 为 emoji 字符串。

### 8.3 底部组（4 个工具按钮）

| 顺序 | 功能 | 图标 | emit 事件 |
|---|---|---|---|
| 1 | 搜索 | `SearchOutline` | `search` |
| 2 | 用户 | `PersonCircleOutline` | `user` |
| 3 | 折叠/展开输出 | `ExpandOutline`/`ContractOutline` | `toggle-output` |
| 4 | 关闭项目 | `CloseCircleOutline` | `close-project` |

### 8.4 样式

- 菜单项尺寸：36×36px
- 圆角：8px（`var(--radius-lg)`）
- 激活态：`box-shadow: inset 3px 0 0 var(--accent-color)`（左侧 3px 强调色竖线）+ 背景 `var(--accent-bg)`
- **禁止折叠功能**（避免无法再展开）

---

## 9. 输出面板

### 9.1 6 标签

| # | key | 标签（i18n） | 说明 |
|---|---|---|---|
| 1 | `output` | 输出 | 编译/运行输出 |
| 2 | `errors` | 错误（带数量徽标） | 编译错误，可点击跳转 |
| 3 | `tips` | 提示 | 查找引用结果 |
| 4 | `bookmarks` | 书签 | Ctrl+F2 添加 |
| 5 | `history` | 历史 | 构建历史 |
| 6 | `debug` | 调试 | DebugPanel 组件 |

### 9.2 样式

- 背景：`var(--bg-output)`
- 圆角：8px（`var(--radius-lg)`）
- `.output-toolbar`：`flex:1` + `min-height:0` + `overflow:hidden`
- n-tabs content/pane：`flex:1` + `min-height:0` + `overflow:hidden`
- `.refs-list`：`height:100%` + `overflow:auto` + `min-height:0`

### 9.3 操作按钮

每个标签页有独立的操作按钮（导出 / 清空 / 全部展开 / 全部折叠 / 关闭等）。

---

## 10. 编辑器规约

### 10.1 语言配置

- 语言 ID：`egou`（固定）
- 缩进：2/4 空格（用 NSelect 替代 NSlider）
- 中文符号自动转换：`（）【】""，。` → English 等价物
- 注释：Go 风格 `//`（禁止单引号 `'`）

### 10.2 功能

- **代码片段**：如果 / 循环 / 函数 / 方法 / 类型 / 选择 / 判断循环
- **自动展开**：
  - `如果` → `如果 ()\n\n否则\n\n结束如果`
  - `选择` → switch-case 结构（`情况` + `默认` 骨架）
  - `判断循环` → `判断循环 ()\n\n结束循环`
- **Ctrl+Click / F12**：跳转到定义（同文件函数/方法 + 跨文件 .elib 函数）
- **书签**：Ctrl+F2 切换，Alt+F2 下一个，Alt+Shift+F2 上一个（F2 留给重命名）
- **自动保存**：debounce 3s，仅对有路径的文件，无对话框
- **代码折叠**：按 fileId 持久化到 localStorage
- **AI 无响应时**：显示"正在思考..."提示

### 10.3 断点与调试

- **断点**：编辑器行号栏（glyph margin）点击切换
- **Shift+点击**：添加断点（与书签区分）
- **F9**：切换当前行断点
- **当前执行行**：高亮显示（黄色背景）
- **断点按文件隔离**：Editor 单实例复用，切换文件时同步断点

---

## 11. 项目树规约

### 11.1 7 个逻辑分类节点

1. **源码文件**：项目根目录下的 .eg 文件（排除 modules/types 目录）
2. **Dll命令**：native 目录文件
3. **窗口**：所有 .ew 文件
4. **资源表**：assets 目录，含"声音"和"图片或图片组"子组
5. **模块引用表**：.elib 扩展包（名称 + 版本 + 作者）
6. **模块**：modules/ 目录下的 .eg 文件
7. **类**：types/ 目录下的 .eg 文件

### 11.2 右键菜单

- **分类节点**：上下文相关菜单（如"源码文件"右键显示"新建代码文件"）
- **文件节点**：打开 / 删除
- **空白区域**：全量新建菜单

---

## 12. 窗口设计器规约

### 12.1 布局

- 左侧：组件箱（toolbox）+ 层级列表（layer-list）
- 中间：设计画布（form-surface）
- 右侧：属性面板（properties）

### 12.2 组件箱

- 内置组件：button/edit/textarea/label/checkbox/radio/listbox/combobox/switch/slider/progress/image/tabs/card/divider
- 外置组件：通过 `components/` 目录声明式注册，支持 SVG 图标 + preview HTML

### 12.3 设计画布

- 网格对齐（可配置网格大小）
- 对齐辅助线（拖拽时显示）
- 等距分布提示
- 多选 + 对齐工具栏（左/中/右/顶/中/底 + 水平/垂直等距分布）
- Tab 顺序编辑模式

### 12.4 属性面板

- 基本属性：名称/位置/尺寸/可见/可用/锁定
- 外观属性：背景色/边框/圆角/字体
- 事件属性：事件名列表
- 外置组件属性：由 config.json 的 props schema 驱动

---

## 13. 字体规约

### 13.1 IDE 字体

- 界面字体：`var(--ide-font)`（`'IdeFont', system-ui, ...`）
- 代码字体：`var(--ide-code-font)`（`'IdeFont', 'Consolas', ...`）
- 字体文件：`frontend/dist/fonts/egou.ttf`（Vite 构建）
- 字体设置同步：通过 CSS 变量 `--ide-code-font` 实时同步
- 启动恢复：从 localStorage 读取字体设置，避免首屏闪烁

### 13.2 持久化键

| 键 | 用途 |
|---|---|
| `eg-uifont` | 界面字体 |
| `eg-fontfamily` | 代码字体 |

---

## 14. 国际化（i18n）

### 14.1 框架

- 自研轻量 i18n（零依赖，约 631 行）
- 支持 `{paramName}` 占位符插值
- zh-CN 兜底 + localStorage 持久化
- `registerMessages` 运行时追加（供扩展包/插件本地化）

### 14.2 命名空间

| 命名空间 | 用途 |
|---|---|
| `common` | 通用动词（确定/取消/保存/删除等） |
| `menu` | 左侧 4 标签 |
| `settings` | 设置面板（75 个 key） |
| `ai` | AI 面板 |
| `debug` | 调试面板 |
| `editor` | 编辑器（hover/补全 detail） |
| `output` | 输出面板 |
| `command` | 命令面板 |
| `leftmenu` | 左侧菜单 |
| `titlebar` | 标题栏 |

### 14.3 切换

- 设置面板 → 界面语言下拉
- 持久化键：`localStorage['egou-locale']`
- 部分文案需重启生效（menuItems 等初始化时读取的文案）

---

## 15. 状态栏规约

- 左右分区布局
- 左侧：项目名 / 当前文件 / 光标位置
- 右侧：缩进 / 编码 / 行尾 / 语言 / 健康状态
- 临时状态消息自动消失
- 去除冗余信息，节约空间
