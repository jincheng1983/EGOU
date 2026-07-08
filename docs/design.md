# EGOU IDE 设计文档（第八版）

> 最后更新：2026-07-08（v0.10.1）
> 配套文档：[计划文档.md](../计划文档.md) / [ui-spec.md](ui-spec.md) / [optimization_plan.md](optimization_plan.md) / [开发日记.md](开发日记.md)

---

## 1. 项目概述

EGOU（易狗 IDE）是一款基于中文语法的桌面应用开发环境，用户使用 `.eg` 中文源码编写程序，IDE 转译为 Go 代码并通过 Wails v3 编译为跨平台桌面应用。

### 1.1 定位

- **目标用户**：中文母语开发者、编程初学者、快速原型开发者
- **核心价值**：中文语法 + Go 性能 + Wails 跨平台 + 可视化设计器 + AI 辅助
- **版本基线**：第八版（最终完善版），基于第七版 NxEGOU 重构

### 1.2 设计原则

| 原则 | 说明 |
|---|---|
| Clean over Compatible | 项目上线前不做向后兼容冗余，代码精简优先 |
| 完整外置化 | exe 只做 IDE 逻辑，所有资源以磁盘文件形式存放 |
| 单一职责 | IDEService 按职责拆分 8 个文件，单文件不超 500 行 |
| 声明式扩展 | 组件包通过 config.json 声明，插件通过 activate(api) 编程 |
| 实验性并行 | AST 编译器前端与正则转译器并行存在，不强制替换 |

---

## 2. 目录架构

采用标准 Go 项目布局 + 三层 internal + 完整外置化发布。

```
EGOU/
├── cmd/egou/             # IDE 主程序入口（main.go）
├── cmd/eg/               # CLI 工具入口
├── internal/             # 内部包（不对外暴露）
│   ├── app/              # IDEService 按职责拆分（8 文件）
│   │   ├── service.go    # 结构体 + ServiceStartup + ServiceName
│   │   ├── project.go    # 项目管理
│   │   ├── file.go       # 文件操作
│   │   ├── compile.go    # 编译运行
│   │   ├── ai.go         # AI 方法
│   │   ├── libs.go       # 支持库/扩展包
│   │   ├── plugins.go    # 插件扫描
│   │   ├── components.go # 外置组件扫描
│   │   ├── debug.go      # 调试器后端
│   │   └── system.go     # 系统/健康检查
│   ├── runner/           # 编译/运行/构建
│   ├── transpiler/       # .eg → Go 转译器（正则 + AST 双通道）
│   ├── ai/               # AI 客户端 + 5 Agent 编排
│   ├── debugger/         # Delve 调试器客户端
│   ├── types/            # 跨层共享类型
│   └── examples/         # 内置示例嵌入
├── runtime/              # 运行时模板源
│   └── wails-template/   # 用户程序编译模板（含 vendor 离线依赖）
├── frontend/             # Vue 3 前端
├── scripts/              # 构建/清理脚本（build.py 强制使用）
├── docs/                 # 文档
├── build/                # Wails v3 跨平台构建配置
├── fonts/                # 字体目录（Vite 构建到 frontend/dist/fonts/）
├── libs/                 # 全局支持库源（.elib 扩展包）
├── templates/            # 项目模板源
├── plugins/              # 插件目录源
├── config/               # 配置目录源
├── components/           # 组件库目录源
└── tools/                # 工具目录（upx.exe 随包分发）
```

### 2.1 发布目录结构（bin/）

完整外置化架构：exe 只做 IDE 逻辑，所有资源以磁盘文件形式存放。

```
bin/
├── EGOU.exe              # IDE 主程序
├── frontend/dist/        # 前端构建产物（WebView 加载）
├── libs/                 # 全局支持库（.elib，所有项目共享）
├── templates/            # 项目模板（新建项目对话框）
├── plugins/              # 插件目录（外置插件生态）
├── config/               # 配置目录（外置配置）
├── components/           # 组件库目录（窗口设计器扩展）
├── wails-template/       # 用户程序编译模板（含 vendor 离线依赖）
└── tools/upx.exe         # UPX 压缩工具
```

### 2.2 模块依赖关系

```
cmd/egou/main.go
    ├── constructs → internal/app.IDEService
    ├── embeds → frontend/dist
    └── registers → Wails application

internal/app/*
    ├── calls → internal/runner
    ├── calls → internal/transpiler
    ├── calls → internal/ai
    ├── calls → internal/debugger
    └── embeds → internal/examples

internal/runner
    ├── calls → internal/transpiler
    ├── aliases → internal/types
    └── embeds → runtime/

internal/transpiler
    └── imports → internal/types

internal/ai
    └── imports → internal/types

internal/debugger
    └── standalone（通过 JSON-RPC 与 dlv 通信）

frontend/src
    └── wails_binding → internal/app.IDEService
```

依赖无环（`internal/types` 打破 runner ↔ app 循环依赖）。

---

## 3. 技术栈

### 3.1 后端（Go）

| 项 | 版本/约束 |
|---|---|
| Go | 1.25.0+ |
| 桌面框架 | Wails v3 v3.0.0-alpha2.110 |
| Service 模式 | Wails Service（实现 ServiceStartup + ServiceName） |
| 静态编译 | `-tags production,netgo,osusergo -trimpath -buildvcs=false -ldflags "-w -s"` |
| 构建脚本 | `python scripts/build.py`（强制使用，禁用 PowerShell 批量写文件） |
| Go SDK | 可配置（runner.SetGoBinary 支持 PATH 之外路径） |
| 调试器 | Delve（dlv）headless 模式，JSON-RPC 通信 |
| 混淆 | garble -tiny（IDE 编译）/ 可选 garble -literals -tiny（用户产品完整混淆） |

### 3.2 前端（Vue 3 + Naive UI）

| 项 | 版本/约束 |
|---|---|
| 框架 | Vue 3 `<script setup>`（禁止 Options API） |
| UI 库 | Naive UI（唯一允许） |
| 编辑器 | Monaco Editor，语言 ID 固定 `egou` |
| 主题 | themes.js 单一数据源（4 套主题 × 39 变量） |
| 国际化 | 自研轻量 i18n（零依赖，约 631 行） |
| 构建 | Vite |
| 字体 | Vite 构建到 frontend/dist/fonts/egou.ttf |

### 3.3 关键依赖

- **Wails v3**：桌面框架，提供 Go ↔ JS 双向绑定
- **Monaco Editor**：代码编辑器，支持自定义语言、补全、悬停、跳转
- **Delve**：Go 调试器，headless 模式通过 JSON-RPC 控制
- **garble**：Go 代码混淆工具（防反编译）
- **UPX**：可执行文件压缩（用户产品可选，IDE 本身不用）

---

## 4. 核心数据流

### 4.1 编译流程

```
用户保存 .eg
    ↓
IDE 调用后端转译器
    ↓
合并 .elib 扩展包源码（全局 libs/ + 项目 libs/）
    ↓
Transpile 正则转译（主通道）/ TranspileAST（实验通道）
    ↓
处理 @嵌入 块
    ↓
生成 //line 指令（错误定位到 .eg 源码）
    ↓
复制运行时模板到临时目录（wails-template/vendor/ 离线依赖）
    ↓
写入用户代码 + 资源嵌入 + 前端资源
    ↓
go build（debug/release 双模式，-mod=vendor）
    ↓
运行 / 复制到产物目录
    ↓
（release 模式）UPX 压缩 + SHA256 校验 + 版本号自增
```

#### 编译模式

| 模式 | 产物名 | 特性 |
|---|---|---|
| debug | `egruntime.exe` | 固定文件名，保留 DWARF（可调试） |
| release | `egruntime-v1.0.1-release.exe` | 含版本号，patch 自增，UPX 可选，garble 可选 |

#### 错误处理

- 结构化解析：`^(.+?):(\d+):(\d+):\s*(.+)$`
- 中文翻译：20 条规则（`undefined:` → `未定义:` 等）
- 前端跳转：错误条目可点击跳转到对应文件行

### 4.2 转译器双通道

EGOU 转译器有两个并行通道：

| 通道 | 文件 | 策略 | 状态 |
|---|---|---|---|
| 正则转译 | transpiler.go（1376 行） | 正则 + 字符串替换，稳定但无结构 | 主通道 |
| AST 转译 | lexer.go + parser.go + gen.go（3907 行） | 词法分析 → AST → 代码生成 | 实验性 |

AST 通道特性：
- **Token 分层**：18 种 Token 类型，`@嵌入`/`@结束` 为独立 Token
- **递归下降解析器**：2383 行，支持错误恢复
- **代码生成器**：1100 行，集成 go/format 格式化
- **符号遍历**：515 行，支持查找引用/定义跳转
- **回退机制**：`TranspileAST` 失败时调用方可回退到 `Transpile`

AST 通道目前仅在测试中使用，未集成到主编译流程（符合"实验性，不替换现有转译器"定位）。

### 4.3 AI 编排

5 Agent 协作完成开发任务：

| 角色 | 名称 | 职责 |
|---|---|---|
| planner | 架构规划 | 分析需求、设计模块、拆分任务 |
| coder | 代码生成 | 根据规划编写 EGOU 代码 |
| reviewer | 代码审查 | 检查代码质量、风格、潜在问题 |
| ui_builder | UI 设计 | 窗口布局、控件配置、事件绑定 |
| fixer | 错误修复 | 根据编译错误诊断并修复 |

#### 危险工具人机确认

- 风险等级：safe / moderate / dangerous
- 危险工具：write_file / delete_file / run_build / run_command / overwrite_file
- 确认超时：30s
- 事件通道：`ai-tool-confirm`（Go → JS）
- 前端回调：`ConfirmToolCall(requestID, approved)`

#### BuildAndFix 自动修复

- 默认最多 3 轮
- 阶段事件：build-start / build-failed / fix-start / fix-applied / build-success / max-rounds-exceeded

### 4.4 调试器

基于 Delve (dlv) headless 模式的集成调试器：

```
前端 DebugPanel.vue
    ↓ F5/F10/F11/Shift+F11
App.vue → IDEService.DebugContinue/Next/Step/StepOut
    ↓
internal/app/debug.go
    ↓
internal/debugger/client.go（JSON-RPC over TCP）
    ↓
dlv headless 子进程
    ↓
egruntime.exe（带 DWARF 调试信息）
```

#### 断点映射

`.eg` 源码通过 `//line` 指令映射到生成的 Go 代码。调试时：
- 用户在 `.eg` 文件设置断点 → 前端记录行号
- 启动调试时传给后端 → 后端通过 `//line` 指令计算对应 Go 文件:行号
- 调用 dlv `CreateBreakpoint` 设置断点

#### 入口断点

调试启动时自动在 `main.mainImpl`（转译后的主函数）设置断点并 continue，避免停在 Go runtime 入口。

### 4.5 扩展系统

EGOU 有 4 种扩展机制：

| 类型 | 目录 | 注册方式 | 用途 |
|---|---|---|---|
| 支持库（.elib） | `libs/` | commands.json 声明式 | 提供中文命令/函数 |
| 组件包 | `components/` | config.json 声明式 | 窗口设计器组件 |
| 插件 | `plugins/` | main.js activate(api) 编程 | IDE 功能扩展 |
| 项目模板 | `templates/` | template.json 声明式 | 新建项目模板 |

#### .elib 扩展包

```
<项目>/libs/<包名>/
├── package.json       # 包元信息（name/version/author）
├── commands.json      # 命令定义
└── source.eg          # 源码
```

- IDE 启动时扫描 `<项目>/libs/*/commands.json`，与内置支持库合并
- 全局库（exe 同级 `libs/`）+ 项目库（`<项目>/libs/`）双层
- 转译时自动合并 `source.eg`，剥离重复声明
- 中文别名 → 英文键映射注册到 transpiler
- 用户扩展是附加的，不能替换内置库

#### 外置组件包（G9）

```
components/<包名>/
├── package.json                    # 包元数据
└── components/
    └── <组件名>/
        ├── config.json             # 组件配置（type/label/icon/props/events/preview）
        └── icon.svg                # 图标（可选）
```

- 后端 `ScanComponents` 扫描，前端 `loadExternalComponents` 加载
- `config.json` 声明式注册，支持 `preview.html` 模板（`{{propName}}` 占位符）
- SVG 图标运行时加载（`v-html` + CSS 尺寸限制）

---

## 5. UI 布局

### 3 栏布局

```
┌─────────────────────────────────────────────┐
│                  TitleBar（40px）             │
├──────┬──────────────────────────────┬───────┤
│Left  │       Editor Stack           │ Right │
│Menu  │       (flex:1)               │ 280px │
│48px  │                              │       │
│      │                              │       │
│240px │                              │       │
│      │                              │       │
├──────┴──────────────────────────────┴───────┤
│              Output Panel（160px）           │
├─────────────────────────────────────────────┤
│              Status Bar                      │
└─────────────────────────────────────────────┘
```

详细规范见 [ui-spec.md](ui-spec.md)。

---

## 6. 关键设计决策

### 6.1 完整外置化（非 go:embed）

**决策**：exe 只做 IDE 逻辑，所有资源以磁盘文件形式存放。

**原因**：
- 用户只需安装 Go 环境，所有 IDE 依赖随包分发
- 资源可独立更新（前端/模板/支持库/插件/组件）
- 支持多 IDE 实例（不同项目用不同配置）
- 避免 go:embed 导致的 exe 体积膨胀

### 6.2 IDEService 拆分

**决策**：第七版 1688 行单文件拆为 8+ 个职责文件。

**收益**：
- 单文件 200-400 行，可读性/可维护性提升
- 职责清晰：project/file/compile/ai/libs/plugins/components/debug/system
- 并行开发互不干扰

### 6.3 AST 编译器前端实验性并行

**决策**：推进 AST 前端但不替换现有正则转译器。

**原因**：
- 正则转译器稳定可靠，已覆盖全部语法
- AST 前端为未来精确错误恢复/查找引用/重命名打基础
- 双通道并行，可在 AST 成熟后渐进切换

### 6.4 调试器入口断点

**决策**：调试启动时自动在 `main.mainImpl` 设置断点并 continue。

**原因**：
- dlv 默认停在 Go runtime 入口（非用户代码）
- 直接 step 会报 `no source for PC` 错误
- 自动 continue 到用户代码入口，避免用户困惑

### 6.5 轻量自研 i18n（非 vue-i18n）

**决策**：使用约 631 行的自研 i18n，不引入 vue-i18n。

**原因**：
- 零依赖，减少包体积
- 支持 `{paramName}` 插值、`registerMessages` 运行时追加
- zh-CN 兜底 + localStorage 持久化
- 渐进迁移策略（高 ROI 优先，不追求一次性全量覆盖）

### 6.6 用户程序离线依赖

**决策**：用户程序编译使用 `wails-template/vendor/` 离线依赖，`-mod=vendor` 标志。

**原因**：
- 用户无需配置 GOPROXY
- 编译环境完全离线可用
- 版本锁定，避免依赖漂移

---

## 7. 安全与防护

### 7.1 IDE 自身

- **不用 UPX**：避免杀毒软件误报
- **garble -tiny**（不含 `-literals`）：避免 TrojanSpy/Stealer 误报
- **SHA256 校验**：构建产物输出校验值

### 7.2 用户产品

- **Garble 混淆三档**：off（普通 go build）/ basic（`garble -tiny`，默认）/ full（`garble -literals -tiny`）
- **UPX 压缩可选**：使用修改过魔数/节区名的 `upx-egou.exe`，避免被逆向工具识别
- **随机密钥**：每次编译注入随机密钥，防止批量逆向

---

## 8. 用户项目结构

```
myego/
├── .eg/                         # IDE 项目元数据（隐藏目录）
│   ├── project.eg.json          # 项目配置（名称、类型、输出名等）
│   ├── layout/                  # 窗口布局描述（IR）
│   │   └── mainwindow.ir.json
│   ├── memory/                  # AI 项目记忆
│   └── cache/                   # 编译缓存
├── src/                         # 源代码目录
│   ├── main.eg                  # 主入口文件（中文语法）
│   ├── window_main.ew           # 主窗口设计文件
│   ├── window_main.eg           # 主窗口代码（事件处理）
│   ├── module/                  # 用户模块
│   └── include/                 # 引用其它共享模块
├── resources/                   # 项目专用资源（图片、声音等）
├── lib/                         # 第三方库（DLL/静态库）
├── libs/                        # .elib 扩展包
│   └── stringx/
├── out/                         # 编译输出（临时）
└── bin/                         # 最终可执行文件
```

---

## 9. 版本历史

| 版本 | 日期 | 重点 |
|---|---|---|
| v0.1.0 | 2026-06 | 第八版架构骨架搭建，目录结构建立 |
| v0.2.0 | 2026-06 | 前端迁移 + 3 栏布局 + 主题系统 |
| v0.3.0 | 2026-06 | 编辑器 + 转译器 + 编译运行 |
| v0.4.0 | 2026-06 | 窗口设计器 + 属性面板 |
| v0.5.0 | 2026-06 | AI 助手 + 5 Agent 编排 + i18n 框架 |
| v0.6.0 | 2026-07 | 扩展系统（.elib + 插件 + 组件包） |
| v0.7.0 | 2026-07 | 项目模板 + 资源管理 |
| v0.8.0 | 2026-07 | 调试器（Delve 集成） |
| v0.9.0 | 2026-07 | 调试器完善 + 工具链路径配置 |
| v0.10.0 | 2026-07 | G9 插件设计器组件完善（真实预览 + SVG 图标） |
| v0.10.1 | 2026-07-08 | i18n 核心组件接入（7 组件 130+ 处调用） |

详细变更见 [开发日记.md](开发日记.md)。
