# EGOU 项目规约

> 本文件是 EGOU（易狗 IDE 第八版，最终完善版）的硬性约束与工程规约，所有代码改动必须遵循。
> 基于第七版 NxEGOU `.trae/rules/project_rules.md` 重构，结合第八版目录架构精炼。
> 最后更新：2026-07-07

---

## 1. 目录架构（标准 Go 布局 + 三层 internal + 完整外置化）

```
EGOU/
├── cmd/egou/             # IDE 主程序入口（main.go）
├── cmd/eg/               # CLI 工具入口
├── internal/             # 内部包（不对外暴露）
│   ├── app/              # IDEService 按职责拆分（service/project/file/compile/ai/libs/plugins/system）
│   ├── runner/           # 编译/运行/构建（runner.go detector.go health.go lru_cache.go）
│   ├── transpiler/       # .eg → Go 转译器
│   ├── ai/               # AI 客户端 + 编排（client orchestrator tools buildandfix）
│   └── types/            # 跨层共享类型（Event/CompileError/HealthReport + Stage 常量）
├── runtime/              # 运行时模板源（build.py 复制到 bin/wails-template/）
│   └── wails-template/   # 用户程序模板（含 vendor 离线依赖）
├── frontend/             # Vue 3 前端
├── scripts/              # 构建/清理脚本（build.py 强制使用）
├── docs/                 # 文档（design.md ui-spec.md optimization_plan.md 开发日记.md 计划文档.md）
├── build/                # Wails v3 跨平台构建配置（windows/darwin/linux）
├── fonts/                # 字体目录（Vite 构建到 frontend/dist/fonts/）
├── libs/                 # 全局支持库源（.elib 扩展包，build.py 复制到 bin/libs/）
├── templates/            # 项目模板源（build.py 复制到 bin/templates/）
├── plugins/              # 插件目录源（build.py 复制到 bin/plugins/）
├── config/               # 配置目录源（build.py 复制到 bin/config/）
├── components/           # 组件库目录源（build.py 复制到 bin/components/）
├── tools/                # 工具目录（upx.exe 随包分发）
├── appicon.png           # 应用图标（PNG）
├── icon.ico              # Windows 图标
└── .trae/rules/          # Trae 项目规约（本文件）
```

### 发布目录结构（bin/，完整外置化）
```
bin/
├── EGOU.exe              # IDE 主程序（exe 只做逻辑，无嵌入资源）
├── frontend/dist/        # 前端构建产物（WebView 加载）
├── libs/                 # 全局支持库（.elib，所有项目共享）
├── templates/            # 项目模板（新建项目对话框）
├── plugins/              # 插件目录（外置插件生态）
├── config/               # 配置目录（外置配置）
├── components/           # 组件库目录（窗口设计器扩展）
├── wails-template/       # 用户程序编译模板（含 vendor 离线依赖）
└── tools/upx.exe         # UPX 压缩工具
```

### 规约
- `cmd/egou/main.go` 是唯一应用入口，禁止在根目录放 main.go
- `internal/` 下的包不对外暴露，只能被本项目引用
- `internal/app/` 下的 IDEService 按职责拆分 8 个文件，禁止单文件超过 500 行
- **完整外置化架构**：exe 只做 IDE 逻辑，所有资源（前端/字体/模板/支持库/插件/工具）以磁盘文件形式存放在 exe 同级目录
- `build.py` 负责把所有资源复制到 `bin/`，发布包就是 `bin/` 目录
- `runtime/wails-template/` 是用户程序编译模板源，build.py 复制到 `bin/wails-template/`（含 vendor 离线依赖）
- `fonts/` 通过 Vite 构建到 `frontend/dist/fonts/egou.ttf`，不再单独复制
- `docs/开发日记.md` 强制维护，每次代码改动后更新

---

## 2. 技术栈约束

### 后端（Go）
- **Go 1.25.0**
- **Wails v3** 是唯一桌面框架（alpha2.110）
- **Wails Service** 必须实现 `ServiceName() string`，并在 `cmd/egou/main.go` 的 `application.New()` 中通过 `application.NewService(...)` 注册
- **完整外置化**：exe 只做 IDE 逻辑，所有资源以磁盘文件形式存放在 exe 同级目录（不用 go:embed 嵌入资源）
- **Go SDK 可配置**：`runner.SetGoBinary(path)` 支持运行时切换 Go 编译器路径
- **静态编译**：`-tags production,netgo,osusergo -trimpath -buildvcs=false -ldflags "-w -s"`
- **依赖管理**：`go mod tidy` 后才能提交

### 前端（Vue 3 + Naive UI）
- **Vue 3 必须 `<script setup>`**，禁止 Options API
- **UI 库只允许 Naive UI**，禁止引入 Element Plus / Ant Design Vue 等
- **编辑器**：Monaco Editor，语言 ID 固定 `egou`
- **主题系统**：`themes.js` 是单一数据源，`style.css` 仅作初始加载回退
- **CSS 变量**：所有颜色必须用 `var(--xxx)`，禁止硬编码 hex/rgba
- **公共常量**：颜色常量集中在 `utils/colors.js`（FLOW_RAINBOW / TYPE_COLOR / GROUP_COLORS）

### 编译
- **严禁 PowerShell 批量写文件**（BOM 会破坏源码）
- **必须使用 Python 脚本静态编译**：`python scripts/build.py`
- **本机编译器路径参考**：`C:\Trae CN\编译器路径参考.md`

---

## 3. UI 布局规约

### 3 栏布局（强制）
```
┌─────────────────────────────────────────────┐
│                  TitleBar                    │
├──────┬──────────────────────────────┬───────┤
│ Left │         Editor Stack         │ Right │
│ 240px│         flex:1               │ 280px │
│      │                              │       │
│ 4 标签│                              │大纲/属性│
│ 文件  │                              │       │
│ 项目  │                              │       │
│ 支持  │                              │       │
│ AI   │                              │       │
├──────┴──────────────────────────────┴───────┤
│                 Output Panel                 │
└─────────────────────────────────────────────┘
```

- 左侧默认 240px（可拖拽调整，持久化到 localStorage），4 标签：文件 / 项目 / 支持 / AI
- 中间 `flex:1`，文件标签页 + 编辑器/设计器
- 右侧默认 280px（可拖拽调整，持久化到 localStorage）：代码视图显示**代码大纲面板**（函数/变量/常量，点击跳转），设计视图显示**属性面板**
- 底部输出面板（高度可拖拽调整），5 标签：输出 / 错误 / 提示 / 书签 / 历史
- 两侧 `v-if` 隐藏时其他列零影响（避免 4 栏布局失败）

### 左侧边栏
- **禁止折叠功能**（避免无法再展开）
- 4 标签固定顺序：文件 / 项目 / 支持 / AI

### 标题栏
- **"编译运行" 仅图标**，无文字
- **"保存"** 和 **"另存为"** 双按钮（保存无对话框，另存为有对话框）
- **"编译运行" 按钮**：透明背景 + 强调色图标，无边框，hover 效果与其他按钮一致
- **图标按钮用原生 `<button>`**（不用 n-button，避免 Naive UI 内部样式干扰）
- **扳手图标下拉菜单**：仅"生成可执行文件 (Windows/Linux/macOS)"和"编译选项..."
- **左侧播放按钮**：自动识别当前平台，触发编译运行

---

## 4. 编辑器规约

- **语言 ID**：`egou`
- **缩进**：2/4 空格（用 NSelect 替代 NSlider）
- **中文符号自动转换**：`（）【】“”，。` → English 等价物
- **注释**：Go 风格 `//`（禁止单引号 `'`）
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

---

## 5. 项目树规约

### 7 个逻辑分类节点
1. **源码文件**：项目根目录下的 .eg 文件（排除 modules/types 目录）
2. **Dll命令**：native 目录文件
3. **窗口**：所有 .ew 文件
4. **资源表**：assets 目录，含"声音"和"图片或图片组"子组
5. **模块引用表**：.elib 扩展包（名称 + 版本 + 作者）
6. **模块**：modules/ 目录下的 .eg 文件
7. **类**：types/ 目录下的 .eg 文件

### 文件分类规则
按绝对路径中的目录名匹配：
- `modules` → 模块
- `types` → 类
- `assets` → 资源表
- 根目录 .eg（非 modules/types）→ 源码文件

### 右键菜单
- **分类节点**：上下文相关菜单（如"源码文件"右键显示"新建代码文件"，"窗口"节点右键显示"新建窗口"）
- **文件节点**：打开 / 删除（源码文件/窗口/模块/类/资源/Dll 均支持删除）
- **空白区域**：全量新建菜单（新建代码文件 / 新建窗口 / 新建类 / 新建模块 / 全部展开 / 全部收缩）

---

## 6. 扩展包规约（.elib）

- **位置**：`<项目>/libs/<包名>/`
- **结构**：`package.json` + `commands.json` + `source.eg`
- **加载**：IDE 启动时扫描 `<项目>/libs/*/commands.json`，与内置支持库合并
- **编译**：转译器自动合并 `<项目>/libs/*/source.eg`，剥离重复声明（`# 程序集` / `导入` / `主函数`）
- **别名**：中文别名 → 英文键映射，注册到 transpiler
- **不可替换**：用户扩展是附加的，不能替换内置库
- **右键菜单**：.elib 节点支持 打开 / 重命名 / 删除
- **双击**：在编辑器中打开 source.eg
- **悬停**：显示 版本 / 作者 / 描述（命令节点显示 callSyntax + summary）
- **清理**：关闭项目时清理项目级库数据

---

## 7. 编译/运行规约

### 编译流程
1. 合并 .elib 扩展包源码（全局 libs/ + 项目 libs/）
2. 转译 .eg → Go（增量缓存，LRU 256）
3. 复制运行时模板到临时目录
4. 写入用户代码 + 资源嵌入 + 前端资源
5. `go build` 编译
6. 运行 / 复制到产物目录

### 错误处理
- **结构化解析**：`^(.+?):(\d+):(\d+):\s*(.+)$` 匹配 `file:line:col: message`
- **中文翻译**：`undefined:` → `未定义:`，`cannot use` → `无法使用` 等 20 条规则
- **前端跳转**：错误条目可点击跳转到对应文件行

### 版本号
- **debug 模式**：`egruntime.exe`（固定文件名）
- **release 模式**：`egruntime-v1.0.1-release.exe`（含版本号）
- **自动自增**：release 构建后 patch+1，写回 `project.eg.json`（仅 release 模式）
- **UPX 压缩**：release 模式尝试 UPX（失败不阻断）
- **SHA256 校验**：构建产物输出 SHA256 到输出面板

### 产物
- **输出目录**：`project.eg.json` 的 `output` 字段（默认 `bin`）
- **原生库**：复制 native/ 目录到产物目录
- **打开文件夹**：构建完成后自动调用 `OpenInExplorer`
- **CheckSignature**：用 Go 原生 `crypto/sha256` + `os.Stat`，**禁止用 PowerShell**

---

## 8. 主题规约

- **单一数据源**：`themes.js`（4 套主题 × 39 变量，含圆角 5 档）
- **style.css**：仅 `:root` 暗色 + `[data-theme="light"]` 作为初始加载回退；不随主题变的全局常量（字体、圆角、能力徽章色）保留在 `:root`
- **4 套主题**：深色（dark）/ 浅色（light）/ 海蓝（ocean）/ 朝阳（sunrise）
- **变量分层**：背景 8 档 / 边框 3 档 / 文字 6 档 / 强调 4 变体 / 语义色 5 种 / 阴影 3 级 / 圆角 5 档 / 渐变 2 档 / 滚动条 2 档
- **磨砂风格**：半透明 rgba 背景（0.75-0.85 alpha）+ `backdrop-filter: blur(12px)`
- **蓝色高光线**：标题栏底部、激活态下划线用 `var(--accent-color)`
- **圆角分级**：4px（小）→ 6px（中）→ 8px（大）→ 12px（模态）→ 16px（容器）
- **过渡时长**：0.15s（统一快速过渡）

---

## 9. 开发流程规约

### 强制
- **代码改动后更新开发日记**：`docs/开发日记.md`
- **使用 Python 脚本编译**：`python scripts/build.py`
- **本机编译器路径参考**：`C:\Trae CN\编译器路径参考.md`
- **项目下自建本地仓库**（git 连接太慢，本地仓库用于灾难恢复）

### 禁止
- **严禁 PowerShell 批量写文件**（BOM 破坏源码）
- **严禁硬编码颜色**（必须用 CSS 变量）
- **严禁 Options API**（必须 `<script setup>`）
- **严禁引入非 Naive UI 组件库**
- **严禁给左侧边栏加折叠功能**
- **严禁在标题栏加"新建窗口"独立图标**（通过 tab '+' 下拉菜单创建）
- **严禁单引号注释**（必须 Go 风格 `//`）
- **严禁在根目录放 main.go**（必须 `cmd/egou/main.go`）
- **严禁 git push --force 到 main/master**

### 推荐
- **代码精简**：不做向后兼容冗余，clean over compatible
- **数据迁移**：项目上线前不需要，功能只用一次的不写迁移
- **Excel 导入导出**：优先使用 Excel
- **双加密**：传输层 + 应用层（首次登录密码修改）
- **大性能数据**：可关闭应用层加密以提升效率
- **查询结果**：不返回 Base64 编码
- **术语**：用"函数"而非"子程序"

---

## 10. 命名规约

- **项目名**：`EGOU`（大写）
- **module 名**：`egou`（小写）
- **语言 ID**：`egou`
- **源码后缀**：`.eg`
- **窗口设计后缀**：`.ew`
- **项目配置**：`project.eg.json`
- **扩展包后缀**：`.elib`
- **运行时产物**：`egruntime.exe`
- **CLI 工具**：`eg`（`cmd/eg/`）
- **程序集名**：与 Go package 名一致，小写
- **函数名**：首字母大写表示公开，小写表示私有
- **变量名**：驼峰或下划线均可，推荐中文命名
- **事件处理注释**：Go 风格 `//` 而非 `'`
- **新增窗口**：自动生成 .ew（设计数据）+ .eg（代码）双文件
- **tab '+' 下拉菜单**：代码文件 / 窗口 / 类 / 模块

---

## 11. 相关文档

- [开发计划](../../docs/计划文档.md)
- [设计文档](../../docs/design.md)
- [UI 视觉规范](../../docs/ui-spec.md)
- [优化推进计划](../../docs/optimization_plan.md)
- [开发日记](../../docs/开发日记.md)
