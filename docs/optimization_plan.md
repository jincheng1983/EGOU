# EGOU IDE 优化推进计划（第八版）

> 最后更新：2026-07-08（v0.10.1）
> 配套文档：[design.md](design.md) / [ui-spec.md](ui-spec.md) / [开发日记.md](开发日记.md)

本文档记录第八版开发过程中的已优化项、进行中项和未来计划项。

---

## 1. 已完成的优化

### 1.1 架构层面

| 优化项 | 版本 | 说明 |
|---|---|---|
| IDEService 拆分 | v0.1.0 | 第七版 1688 行单文件拆为 8+ 个职责文件，单文件 200-400 行 |
| 标准目录布局 | v0.1.0 | 采用标准 Go 项目布局，cmd/internal/runtime 分层 |
| 完整外置化 | v0.1.0 | exe 只做逻辑，资源全部磁盘文件，支持多实例 |
| internal/types 解耦 | v0.1.0 | 打破 runner ↔ app 循环依赖 |
| Wails Service 模式 | v0.1.0 | 实现 ServiceStartup + ServiceName，标准注册 |

### 1.2 前端层面

| 优化项 | 版本 | 说明 |
|---|---|---|
| Vue 3 `<script setup>` | v0.2.0 | 全面禁止 Options API |
| 主题单一数据源 | v0.2.0 | themes.js 集中管理 4 套主题 × 39 变量 |
| CSS 变量强制 | v0.2.0 | 禁止硬编码颜色，全部用 `var(--xxx)` |
| 3 栏布局 + 拖拽 | v0.2.0 | 左 240px / 右 280px / 输出 160px 可拖拽，持久化 |
| 左侧边栏禁止折叠 | v0.2.0 | 避免折叠后无法再展开 |
| 标题栏原生 button | v0.2.0 | 避免 Naive UI 内部样式干扰 |
| 字体启动恢复 | v0.2.0 | 从 localStorage 读取字体设置，避免首屏闪烁 |
| 输出面板防溢出 | v0.7.0 | flex:1 + min-height:0 + overflow:hidden |
| 窗口设计器对齐辅助 | v0.4.0 | 拖拽时显示对齐线 + 等距分布提示 |
| refs-list 滚动修复 | v0.7.0 | height:100% + overflow:auto + min-height:0 |
| AI 会话状态保持 | v0.5.0 | AIPanel 用 v-show 而非 v-else-if，切换 tab 不丢状态 |

### 1.3 编辑器层面

| 优化项 | 版本 | 说明 |
|---|---|---|
| 中文符号自动转换 | v0.3.0 | `（）【】""，。` → English 等价物 |
| 代码片段自动展开 | v0.3.0 | 如果/选择/判断循环等关键字自动展开骨架 |
| 跨文件跳转定义 | v0.3.0 | Ctrl+Click/F12 跳转到 .elib 函数定义 |
| 书签系统 | v0.3.0 | Ctrl+F2 切换，持久化到 localStorage |
| 代码折叠持久化 | v0.3.0 | 按 fileId 持久化折叠状态 |
| 自动保存 | v0.3.0 | debounce 3s，无对话框 |

### 1.4 编译/转译层面

| 优化项 | 版本 | 说明 |
|---|---|---|
| 增量编译缓存 | v0.3.0 | LRU 256，未修改文件跳过转译 |
| 结构化错误解析 | v0.3.0 | `file:line:col: message` 正则匹配 |
| 中文错误翻译 | v0.3.0 | 20 条规则（undefined → 未定义等） |
| //line 指令 | v0.3.0 | 错误定位到 .eg 源码而非合并后的 usercode.go |
| 离线依赖 | v0.6.0 | wails-template/vendor/ + -mod=vendor，无需 GOPROXY |
| Go SDK 可配置 | v0.9.0 | runner.SetGoBinary 支持 PATH 之外路径 |
| AST 编译器前端 | v0.5.0+ | lexer + parser + gen + symbols，6563 行 + 5900 行测试 |

### 1.5 调试器层面

| 优化项 | 版本 | 说明 |
|---|---|---|
| Delve 集成 | v0.8.0 | headless 模式 + JSON-RPC |
| 入口断点自动 continue | v0.9.0 | 避免停在 Go runtime 入口 |
| 断点按文件隔离 | v0.9.13 | Editor 单实例复用，切换文件同步断点 |
| 断点保留 | v0.9.13 | 调试结束不清除用户断点 |
| F9 列表同步 | v0.9.13 | toggleBreakpointLine 返回值决定 add/remove |
| 工具链路径配置 | v0.9.3 | Go 编译器 + dlv 路径可配置 |

### 1.6 AI 层面

| 优化项 | 版本 | 说明 |
|---|---|---|
| 5 Agent 编排 | v0.5.0 | planner/coder/reviewer/ui_builder/fixer |
| 危险工具人机确认 | v0.5.0 | safe/moderate/dangerous 三级 + 30s 超时 |
| BuildAndFix 自动修复 | v0.5.0 | 默认 3 轮，错误正则 + 代码块提取 |
| "正在思考..."提示 | v0.5.0 | AI 无响应时显示 |

### 1.7 扩展系统层面

| 优化项 | 版本 | 说明 |
|---|---|---|
| .elib 扩展包 | v0.6.0 | 全局库 + 项目库双层，自动合并 source.eg |
| 插件系统 | v0.6.0 | activate(api) 编程式注册 |
| 外置组件包（G9） | v0.10.0 | config.json 声明式 + preview HTML + SVG 图标 |
| 项目模板 | v0.7.0 | console/window/blank 三种模板 |

### 1.8 安全层面

| 优化项 | 版本 | 说明 |
|---|---|---|
| CheckSignature Go 原生 | v0.6.0 | 替代 PowerShell，用 crypto/sha256 + os.Stat |
| IDE 不用 UPX | v0.6.0 | 避免杀毒软件误报 |
| garble -tiny（无 -literals） | v0.6.0 | 避免 TrojanSpy/Stealer 误报 |
| 用户产品 Garble 三档 | v0.6.0 | off/basic/full |
| 修改版 UPX | v0.6.0 | 魔数/节区名/版本字符串修改，避免被识别 |

### 1.9 国际化

| 优化项 | 版本 | 说明 |
|---|---|---|
| i18n 框架 | v0.5.0 | 零依赖自研，约 273 行 |
| 核心组件接入 | v0.10.1 | 7 组件 130+ 处调用，字典扩展到 631 行 |

---

## 2. 已知问题与限制

### 2.1 非阻塞问题

| 问题 | 影响 | 状态 |
|---|---|---|
| dlv 1.25.2 + Go 1.26.4 不兼容 | State() 返回 threads=0 | 需用户配置匹配的 dlv 版本 |
| `runtime/wails-template` 报 embeddedFiles undefined | build-tag 问题 | 预先存在，与 IDE 无关 |
| 计划文档位置不一致 | 文档引用 | 实际在根目录，计划中写 docs/ |
| i18n 覆盖率不足 | 部分组件仍硬编码中文 | SettingsPanel 76 处 / App.vue 500+ 处 / WindowDesigner 263 处待迁移 |

### 2.2 已修复的关键 Bug

| Bug | 版本 | 根因 |
|---|---|---|
| AST parser 无限循环 OOM | v0.5.0 | default 分支未消费 Token，syncToNextStatement 死循环 |
| 转译器类型转换正则无 ^ 锚点 | v0.5.0 | `文本型(字符)` → `string(string((字符))` 错误嵌套 |
| stripEntryDeclarations 误删事件处理 | v0.6.0 | 嵌套条件判断导致 `导入 (...)` 块后代码全被剥离 |
| parseBlockStmt 不跳过注释 | v0.6.0 | 注释出现在函数声明后导致空函数体 |
| scheduleScroll 闭包陷阱 | v0.7.0 | null ref 导致自动滚动失败 |
| outputAutoScroll 编译时被置 false | v0.7.0 | 运行事件输出追加后未重置 |
| 调试结束清除用户断点 | v0.9.13 | clearDebugState 调用 debugBreakpoints.clear() |
| 断点不按文件隔离 | v0.9.13 | Editor 单实例复用但断点全局共享 |
| F9 删除断点列表不同步 | v0.9.13 | onToggleBreakpoint 总是 addBreakpoint |
| Wails Frameless 窗口高度 | v0.7.0 | 未计算 +32px 标题栏高度 |
| 输出面板文字溢出 | v0.7.0 | n-tabs content 未设 min-height:0 |

---

## 3. 进行中的优化

### 3.1 i18n 国际化（渐进迁移）

**当前状态**：7 组件 130+ 处调用，字典 631 行。

**待迁移**：
- SettingsPanel.vue：76 处设置项标签
- App.vue：500+ 处对话框/状态栏/右键菜单文案
- WindowDesigner.vue：263 处中文文案
- PropertiesPanel.vue / FileExplorer.vue / ProjectExplorer.vue / SupportPanel.vue / StartPage.vue：尚未接入

**策略**：每次聚焦一个组件，高 ROI 优先。

### 3.2 AST 编译器前端（实验性）

**当前状态**：6563 行代码 + 5900 行测试通过，TranspileAST 入口 + 回退机制完整。

**待推进**：
- 提高语法覆盖率（当前未集成到主编译流程）
- 基于 AST 实现精确错误恢复
- 基于符号表实现查找引用/重命名（symbols.go 已有 FindIdentRefs/FindDefinition）
- 未来可替代正则转译器

---

## 4. 未来优化计划

### 4.1 短期（下一个版本）

| 优化项 | 优先级 | 说明 |
|---|---|---|
| 外置组件代码生成 | 高 | 转译器需认识外置组件类型，生成 Go 代码 |
| i18n 全量覆盖 | 中 | 渐进迁移剩余组件 |
| dlv 版本自动检测 | 中 | 启动时检测 dlv 与 Go 版本兼容性 |
| 条件断点 | 低 | dlv 支持条件断点，前端需暴露 UI |

### 4.2 中期

| 优化项 | 优先级 | 说明 |
|---|---|---|
| AST 通道集成 | 中 | 可选启用 AST 转译，对比正则通道结果 |
| 项目记忆（AI 上下文） | 中 | 滚动摘要 + 关键决策记录，存到 `<项目>/.eg/memory/` |
| 手动暂停调试 | 低 | dlv 支持 halt，前端需暴露按钮 |
| 插件市场 | 低 | 在线插件安装/更新 |

### 4.3 长期

| 优化项 | 优先级 | 说明 |
|---|---|---|
| AST 替代正则转译 | 低 | AST 通道成熟后渐进切换 |
| 多语言支持 | 低 | i18n 扩展更多语言（日语/韩语等） |
| 团队协作 | 低 | 项目共享/版本控制集成 |
| 云端构建 | 低 | 在线编译/发布 |

---

## 5. 性能优化记录

### 5.1 编译性能

- **增量缓存**：LRU 256，未修改文件跳过转译
- **离线依赖**：`-mod=vendor` 避免 GOPROXY 查询
- **静态编译**：`-trimpath -ldflags "-w -s"` 减小体积

### 5.2 前端性能

- **Vite 构建**：开发热更新 + 生产 Tree Shaking
- **Monaco Worker**：Web Worker 处理语法分析，不阻塞 UI
- **debounce 自动保存**：3s 延迟，避免频繁 IO
- **CSS 变量主题切换**：无需重渲染 DOM

### 5.3 内存优化

- **Editor 单实例复用**：切换文件复用 Monaco 实例，避免内存泄漏
- **断点按文件过滤**：而非每文件独立存储
- **LRU 缓存**：编译产物缓存上限 256

---

## 6. 代码质量指标

### 6.1 当前规模

| 模块 | 行数 | 说明 |
|---|---|---|
| internal/app/ | ~3000 行 | 9 个职责文件 |
| internal/transpiler/ | 6563 行 | 正则 + AST 双通道 |
| internal/runner/ | ~1500 行 | 编译/运行/缓存 |
| internal/ai/ | ~2000 行 | 5 Agent + 工具 |
| internal/debugger/ | ~800 行 | Delve 客户端 |
| frontend/src/ | ~15000 行 | Vue 组件 |
| **总计** | ~30000 行 | |

### 6.2 测试覆盖

| 模块 | 测试文件 | 测试行数 |
|---|---|---|
| internal/transpiler/ | 8 个 | ~5900 行 |
| internal/runner/ | — | — |
| internal/ai/ | — | — |
| internal/debugger/ | — | — |

转译器测试覆盖最完整（含 block_decl/event_handler/line_directive/merged_package/stringx_verify/user_event 等）。

### 6.3 代码规约遵守

- ✅ Vue 3 `<script setup>` 100%
- ✅ Naive UI 唯一 UI 库
- ✅ CSS 变量 100%（无硬编码颜色）
- ✅ Go 风格 `//` 注释
- ✅ 单文件不超 500 行（internal/app/）
- ✅ Python 脚本编译（禁用 PowerShell 批量写文件）
