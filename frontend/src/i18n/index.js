// EGOU 轻量 i18n（P3-4，吸取 NxEGO2）
//
// 设计目标：
//   - 不引入 vue-i18n 等重量级依赖
//   - 支持 {paramName} 占位符插值
//   - zh-CN 作为兜底语言
//   - registerMessages 运行时追加翻译（供扩展包/插件本地化使用）
//   - 语言选择持久化到 localStorage
//   - 约 200 行，零依赖
//
// 使用示例：
//   import { t, setLocale, registerMessages } from '@/i18n'
//   t('common.ok')                      // "确定"
//   t('ai.thinking', { agent: '编程' }) // "编程 正在思考..."
//   registerMessages('en-US', { common: { ok: 'OK' } })

const DEFAULT_LOCALE = 'zh-CN'
const FALLBACK_LOCALE = 'zh-CN'
const STORAGE_KEY = 'egou-locale'

// 翻译字典：{ [locale]: { [namespace]: { [key]: value } } }
const messages = {}

// 从 localStorage 读取上次选择的语言（首次默认 zh-CN）
function loadPersistedLocale() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    return saved || DEFAULT_LOCALE
  } catch {
    return DEFAULT_LOCALE
  }
}

// 当前语言
let currentLocale = loadPersistedLocale()

// 从对象中按点分隔路径取值：getNestedValue({a:{b:1}}, 'a.b') -> 1
function getNestedValue(obj, path) {
  if (!obj || typeof path !== 'string') return undefined
  const keys = path.split('.')
  let cur = obj
  for (const k of keys) {
    if (cur == null || typeof cur !== 'object') return undefined
    cur = cur[k]
    if (cur === undefined) return undefined
  }
  return cur
}

// 深度合并 source 到 target（仅对象类型递归，数组直接替换）
function deepMerge(target, source) {
  if (!target || typeof target !== 'object') return source
  if (!source || typeof source !== 'object') return target
  const out = Array.isArray(target) ? [...target] : { ...target }
  for (const key of Object.keys(source)) {
    const sv = source[key]
    const tv = out[key]
    if (sv && typeof sv === 'object' && !Array.isArray(sv) &&
        tv && typeof tv === 'object' && !Array.isArray(tv)) {
      out[key] = deepMerge(tv, sv)
    } else {
      out[key] = sv
    }
  }
  return out
}

// 占位符插值：translate('Hello {name}', { name: 'EGOU' }) -> "Hello EGOU"
function interpolate(template, params) {
  if (!params || typeof template !== 'string') return template
  return template.replace(/\{(\w+)\}/g, (_, k) => (params[k] !== undefined ? String(params[k]) : `{${k}}`))
}

// 翻译函数：t(key, params?, locale?)
// 找不到时回退到 FALLBACK_LOCALE，再找不到返回 key 本身
function t(key, params, locale) {
  const loc = locale || currentLocale
  const primary = messages[loc] ? getNestedValue(messages[loc], key) : undefined
  const value = primary !== undefined
    ? primary
    : (messages[FALLBACK_LOCALE] ? getNestedValue(messages[FALLBACK_LOCALE], key) : undefined)
  if (value === undefined || value === null) return key
  return interpolate(value, params)
}

// 注册/追加翻译字典（深度合并到已有同名 locale）
function registerMessages(locale, dict) {
  if (!locale || !dict) return
  messages[locale] = messages[locale] ? deepMerge(messages[locale], dict) : { ...dict }
}

// 切换当前语言（持久化到 localStorage）
function setLocale(locale) {
  if (locale && messages[locale]) {
    currentLocale = locale
    try { localStorage.setItem(STORAGE_KEY, locale) } catch {}
  }
}

// 获取当前语言
function getLocale() {
  return currentLocale
}

// 列出已注册的语言
function listLocales() {
  return Object.keys(messages)
}

// 内置 zh-CN 兜底字典（仅核心通用文案，组件级文案由各组件自行注册）
registerMessages(DEFAULT_LOCALE, {
  common: {
    ok: '确定',
    cancel: '取消',
    save: '保存',
    saveAs: '另存为',
    delete: '删除',
    rename: '重命名',
    open: '打开',
    close: '关闭',
    yes: '是',
    no: '否',
    loading: '加载中...',
    empty: '暂无数据',
    error: '错误',
    warning: '警告',
    info: '提示',
    success: '成功',
    attach: '附件',
    image: '图片',
    file: '文件',
    folder: '目录',
    copy: '复制',
    paste: '粘贴',
    cut: '剪切',
    selectAll: '全选',
    find: '查找',
    replace: '替换',
    refresh: '刷新',
    confirm: '确认',
    back: '返回',
    forward: '前进',
    details: '详情',
  },
  ai: {
    thinking: '正在思考...',
    send: '发送',
    stop: '停止生成',
    newChat: '新建会话',
    history: '历史会话',
    permission: 'AI 助手使用须知',
    accept: '同意并开始使用',
    goToSettings: '前往 AI 设置',
  },
  menu: {
    file: '文件',
    project: '项目',
    support: '支持',
    ai: 'AI',
  },
  settings: {
    title: '设置',
    general: '常规',
    theme: '主题',
    editor: '编辑器',
    designer: '设计器',
    build: '编译',
    ui: '界面',
    ai: 'AI',
    plugins: '插件',
    templates: '模板',
    language: '界面语言',
    languageDesc: '切换 IDE 界面显示语言（重启后完全生效，部分组件渐进迁移中）',
    // 设置子项分组标题
    appearance: '外观',
    fontSettings: '字体设置',
    codeFont: '代码字体',
    uiFont: '界面字体',
    fontSize: '字号',
    lineHeight: '行高',
    tabSize: '缩进',
    wordWrap: '自动换行',
    minimap: '小地图',
    lineNumbers: '行号',
    autoSave: '自动保存',
    autoSaveDelay: '自动保存延迟',
    autoConvertSymbols: '中文符号自动转英文',
    renderWhitespace: '显示空白',
    cursorBlinking: '光标闪烁',
    cursorSmooth: '光标平滑移动',
    cursorWidth: '光标宽度',
    bracketPairColorization: '括号着色',
    guidesBracketPairs: '括号指引',
    fontLigatures: '字体连字',
    editorTheme: '编辑器主题',
    themeMode: '主题模式',
    customTheme: '自定义主题',
    builtInThemes: '内置主题',
    // 设计器
    designerGrid: '设计器网格',
    gridSize: '网格大小',
    showGrid: '显示网格',
    snapGrid: '对齐网格',
    defaultRadius: '默认圆角',
    defaultBorderWidth: '默认边框',
    // 编译
    buildMode: '编译模式',
    buildModeDebug: '调试',
    buildModeRelease: '发布',
    autoOpenFolder: '编译后自动打开文件夹',
    garbleLevel: 'Garble 混淆强度',
    garbleOff: '关闭',
    garbleBasic: '基础（默认）',
    garbleFull: '完整',
    showBuildHistory: '显示构建历史',
    outputDir: '输出目录',
    toolchainPath: '工具链路径',
    goPath: 'Go 编译器路径',
    delvePath: 'Delve 调试器路径',
    toolchainHint: '留空则自动查找（PATH/GOPATH/bin）',
    // 界面
    startup: '启动',
    openLastProject: '启动时打开上次项目',
    panelSize: '面板尺寸',
    leftPanelWidth: '左侧面板宽度',
    rightPanelWidth: '右侧面板宽度',
    outputPanelHeight: '输出面板高度',
    statusBar: '状态栏',
    sbShowCursor: '显示光标位置',
    sbShowIndent: '显示缩进',
    sbShowEncoding: '显示编码',
    sbShowEol: '显示行尾',
    sbShowLang: '显示语言',
    sbShowHealth: '显示健康状态',
    autoSwitchOutputTab: '编译时自动切换到输出',
    smartScroll: '智能滚动',
    // AI
    aiModels: 'AI 模型',
    addModel: '添加模型',
    defaultModel: '默认模型',
    apiBase: 'API 地址',
    apiKey: 'API Key',
    modelName: '模型名称',
    // 插件
    pluginList: '插件列表',
    reloadPlugins: '重新加载插件',
    noPlugins: '暂无插件',
    // 模板
    projectTemplates: '项目模板',
    saveAsTemplate: '保存为模板',
    templateName: '模板名称',
  },
  leftmenu: {
    search: '搜索',
    user: '用户',
    expandOutput: '展开输出栏',
    collapseOutput: '收起输出栏',
    closeProject: '关闭项目',
  },
  titlebar: {
    run: '编译运行',
    save: '保存',
    saveAs: '另存为',
    settings: '设置',
    saveHint: '保存 (Ctrl+S)',
    undo: '撤销',
    redo: '重做',
    build: '编译项目',
    debug: '调试',
    step: '单步',
    breakpoint: '断点',
    about: '关于',
    theme: '切换主题',
    snippets: '代码片段管理',
    systemSettings: '系统设置',
  },
  ai: {
    thinking: '正在思考...',
    send: '发送',
    stop: '停止生成',
    newChat: '新建会话',
    history: '历史会话',
    permission: 'AI 助手使用须知',
    accept: '同意并开始使用',
    goToSettings: '前往 AI 设置',
    permissionBody: 'AI 助手可以帮助你编写、解释和调试 EGOU 代码。使用前请注意：',
    permissionItem1: 'AI 可能会生成错误的代码，请在运行前仔细检查',
    permissionItem2: '对话内容将发送到你在「系统设置 → AI」中配置的模型提供商',
    permissionItem3: 'API Key 保存在本地 localStorage 中，不会上传到任何第三方',
    permissionItem4: 'AI 不会自动修改你的文件，除非你明确确认',
    inputPlaceholder: '输入问题，AI 将协助你编写代码',
    newChatLabel: '新会话',
    emptyHistory: '暂无历史会话',
    stopped: '已停止',
  },
  debug: {
    start: '开始调试',
    continue: '继续 (F5)',
    stepOver: '单步跳过 (F10)',
    stepInto: '单步进入 (F11)',
    stepOut: '单步跳出 (Shift+F11)',
    stop: '停止调试',
    removeBreakpoint: '删除断点',
    paused: '已暂停',
    exited: '已退出',
    running: '运行中...',
    compiling: '编译中...',
    runningToEntry: '运行到入口断点...',
    stepOverIng: '单步跳过...',
    stepIntoIng: '单步进入...',
    stepOutIng: '单步跳出...',
    noProject: '未打开项目，无法调试',
    alreadyDebugging: '已在调试中',
    startFailed: '启动失败: {msg}',
    error: '错误: {msg}',
    unknownError: '未知错误',
    localVars: '局部',
    argVars: '参数',
    callStack: '调用栈',
    variables: '变量',
    breakpoints: '断点',
    noBreakpoints: '暂无断点',
    file: '文件',
    line: '行',
  },
  editor: {
    bookmark: '书签 (行 {line})',
    breakpoint: '断点 (行 {line})',
    currentLine: '当前执行位置',
    keyword: '关键字',
    dataType: '数据类型',
    snippet: '代码片段',
    libCommand: '支持库命令',
    function: '函数',
    method: '方法',
    type: '类型',
    const: '常量',
    var: '变量',
  },
  output: {
    output: '输出',
    errors: '错误',
    errorsWithCount: '错误 {count}',
    tips: '提示',
    bookmarks: '书签',
    history: '历史',
    debug: '调试',
    findRefs: '查找引用: {query}（共 {total} 处，{files} 个文件）',
    expandAll: '全部展开',
    collapseAll: '全部折叠',
    close: '关闭',
    bookmarkList: '书签（共 {count} 处）',
    clearAll: '清除全部',
    emptyBookmarks: '暂无书签。按 Ctrl+F2 在当前行添加书签。',
    buildHistory: '构建历史（共 {count} 条）',
    emptyBuildHistory: '暂无构建历史。构建完成后会自动记录。',
    copyPath: '复制路径',
    open: '打开',
    line: '行 {line}',
  },
  command: {
    startDebug: '开始调试',
    stopDebug: '停止调试',
    debugContinue: '继续执行',
    debugStepOver: '单步跳过',
    debugStepInto: '单步进入',
    debugStepOut: '单步跳出',
    toggleBreakpoint: '切换断点',
    toggleBreakpointGlyph: '切换断点（行号栏）',
    categoryDebug: '调试',
    categoryBookmark: '书签',
    debugContinueDesc: '继续执行（调试中）/ 编译运行',
    toggleBookmark: '切换书签',
    nextBookmark: '下一书签',
    prevBookmark: '上一书签',
  },
})

// 英文翻译（en-US）
registerMessages('en-US', {
  common: {
    ok: 'OK',
    cancel: 'Cancel',
    save: 'Save',
    saveAs: 'Save As',
    delete: 'Delete',
    rename: 'Rename',
    open: 'Open',
    close: 'Close',
    yes: 'Yes',
    no: 'No',
    loading: 'Loading...',
    empty: 'No data',
    error: 'Error',
    warning: 'Warning',
    info: 'Info',
    success: 'Success',
    attach: 'Attachment',
    image: 'Image',
    file: 'File',
    folder: 'Folder',
    copy: 'Copy',
    paste: 'Paste',
    cut: 'Cut',
    selectAll: 'Select All',
    find: 'Find',
    replace: 'Replace',
    refresh: 'Refresh',
    confirm: 'Confirm',
    back: 'Back',
    forward: 'Forward',
    details: 'Details',
  },
  ai: {
    thinking: 'Thinking...',
    send: 'Send',
    stop: 'Stop',
    newChat: 'New Chat',
    history: 'History',
    permission: 'AI Assistant Notice',
    accept: 'Accept and Start',
    goToSettings: 'Go to AI Settings',
    permissionBody: 'The AI assistant can help you write, explain, and debug EGOU code. Please note before use:',
    permissionItem1: 'AI may generate incorrect code; please review carefully before running',
    permissionItem2: 'Conversation content will be sent to the model provider configured in "System Settings → AI"',
    permissionItem3: 'API Key is stored locally in localStorage and never uploaded to third parties',
    permissionItem4: 'AI will not modify your files unless you explicitly confirm',
    inputPlaceholder: 'Ask a question; AI will help you write code',
    newChatLabel: 'New chat',
    emptyHistory: 'No chat history',
    stopped: 'Stopped',
  },
  menu: {
    file: 'Files',
    project: 'Project',
    support: 'Support',
    ai: 'AI',
  },
  settings: {
    title: 'Settings',
    general: 'General',
    theme: 'Theme',
    editor: 'Editor',
    designer: 'Designer',
    build: 'Build',
    ui: 'UI',
    ai: 'AI',
    plugins: 'Plugins',
    templates: 'Templates',
    language: 'Language',
    languageDesc: 'Switch IDE UI language (full effect after restart, some components are being migrated progressively)',
    appearance: 'Appearance',
    fontSettings: 'Font Settings',
    codeFont: 'Code Font',
    uiFont: 'UI Font',
    fontSize: 'Font Size',
    lineHeight: 'Line Height',
    tabSize: 'Tab Size',
    wordWrap: 'Word Wrap',
    minimap: 'Minimap',
    lineNumbers: 'Line Numbers',
    autoSave: 'Auto Save',
    autoSaveDelay: 'Auto Save Delay',
    autoConvertSymbols: 'Auto-convert Chinese symbols to English',
    renderWhitespace: 'Render Whitespace',
    cursorBlinking: 'Cursor Blinking',
    cursorSmooth: 'Smooth Cursor Movement',
    cursorWidth: 'Cursor Width',
    bracketPairColorization: 'Bracket Pair Colorization',
    guidesBracketPairs: 'Bracket Pair Guides',
    fontLigatures: 'Font Ligatures',
    editorTheme: 'Editor Theme',
    themeMode: 'Theme Mode',
    customTheme: 'Custom Theme',
    builtInThemes: 'Built-in Themes',
    designerGrid: 'Designer Grid',
    gridSize: 'Grid Size',
    showGrid: 'Show Grid',
    snapGrid: 'Snap to Grid',
    defaultRadius: 'Default Radius',
    defaultBorderWidth: 'Default Border Width',
    buildMode: 'Build Mode',
    buildModeDebug: 'Debug',
    buildModeRelease: 'Release',
    autoOpenFolder: 'Auto-open folder after build',
    garbleLevel: 'Garble Obfuscation',
    garbleOff: 'Off',
    garbleBasic: 'Basic (default)',
    garbleFull: 'Full',
    showBuildHistory: 'Show Build History',
    outputDir: 'Output Directory',
    toolchainPath: 'Toolchain Path',
    goPath: 'Go Compiler Path',
    delvePath: 'Delve Debugger Path',
    toolchainHint: 'Leave empty for auto-detect (PATH/GOPATH/bin)',
    startup: 'Startup',
    openLastProject: 'Open last project on startup',
    panelSize: 'Panel Size',
    leftPanelWidth: 'Left Panel Width',
    rightPanelWidth: 'Right Panel Width',
    outputPanelHeight: 'Output Panel Height',
    statusBar: 'Status Bar',
    sbShowCursor: 'Show cursor position',
    sbShowIndent: 'Show indentation',
    sbShowEncoding: 'Show encoding',
    sbShowEol: 'Show EOL',
    sbShowLang: 'Show language',
    sbShowHealth: 'Show health status',
    autoSwitchOutputTab: 'Switch to Output on build',
    smartScroll: 'Smart Scroll',
    aiModels: 'AI Models',
    addModel: 'Add Model',
    defaultModel: 'Default Model',
    apiBase: 'API Base URL',
    apiKey: 'API Key',
    modelName: 'Model Name',
    pluginList: 'Plugins',
    reloadPlugins: 'Reload Plugins',
    noPlugins: 'No plugins installed',
    projectTemplates: 'Project Templates',
    saveAsTemplate: 'Save as Template',
    templateName: 'Template Name',
  },
  leftmenu: {
    search: 'Search',
    user: 'User',
    expandOutput: 'Expand Output',
    collapseOutput: 'Collapse Output',
    closeProject: 'Close Project',
  },
  titlebar: {
    run: 'Run',
    save: 'Save',
    saveAs: 'Save As',
    settings: 'Settings',
    saveHint: 'Save (Ctrl+S)',
    undo: 'Undo',
    redo: 'Redo',
    build: 'Build Project',
    debug: 'Debug',
    step: 'Step',
    breakpoint: 'Breakpoint',
    about: 'About',
    theme: 'Switch Theme',
    snippets: 'Snippets Manager',
    systemSettings: 'System Settings',
  },
  debug: {
    start: 'Start Debug',
    continue: 'Continue (F5)',
    stepOver: 'Step Over (F10)',
    stepInto: 'Step Into (F11)',
    stepOut: 'Step Out (Shift+F11)',
    stop: 'Stop Debug',
    removeBreakpoint: 'Remove Breakpoint',
    paused: 'Paused',
    exited: 'Exited',
    running: 'Running...',
    compiling: 'Compiling...',
    runningToEntry: 'Running to entry breakpoint...',
    stepOverIng: 'Stepping over...',
    stepIntoIng: 'Stepping into...',
    stepOutIng: 'Stepping out...',
    noProject: 'No project open; cannot debug',
    alreadyDebugging: 'Already debugging',
    startFailed: 'Start failed: {msg}',
    error: 'Error: {msg}',
    unknownError: 'Unknown error',
    localVars: 'Local',
    argVars: 'Args',
    callStack: 'Call Stack',
    variables: 'Variables',
    breakpoints: 'Breakpoints',
    noBreakpoints: 'No breakpoints',
    file: 'File',
    line: 'Line',
  },
  editor: {
    bookmark: 'Bookmark (line {line})',
    breakpoint: 'Breakpoint (line {line})',
    currentLine: 'Current execution position',
    keyword: 'Keyword',
    dataType: 'Data Type',
    snippet: 'Snippet',
    libCommand: 'Library Command',
    function: 'Function',
    method: 'Method',
    type: 'Type',
    const: 'Constant',
    var: 'Variable',
  },
  output: {
    output: 'Output',
    errors: 'Errors',
    errorsWithCount: 'Errors {count}',
    tips: 'Tips',
    bookmarks: 'Bookmarks',
    history: 'History',
    debug: 'Debug',
    findRefs: 'Find references: {query} ({total} results in {files} files)',
    expandAll: 'Expand All',
    collapseAll: 'Collapse All',
    close: 'Close',
    bookmarkList: 'Bookmarks ({count})',
    clearAll: 'Clear All',
    emptyBookmarks: 'No bookmarks. Press Ctrl+F2 to add a bookmark on the current line.',
    buildHistory: 'Build history ({count})',
    emptyBuildHistory: 'No build history. Builds will be recorded automatically.',
    copyPath: 'Copy Path',
    open: 'Open',
    line: 'Line {line}',
  },
  command: {
    startDebug: 'Start Debug',
    stopDebug: 'Stop Debug',
    debugContinue: 'Continue',
    debugStepOver: 'Step Over',
    debugStepInto: 'Step Into',
    debugStepOut: 'Step Out',
    toggleBreakpoint: 'Toggle Breakpoint',
    toggleBreakpointGlyph: 'Toggle Breakpoint (gutter)',
    categoryDebug: 'Debug',
    categoryBookmark: 'Bookmark',
    debugContinueDesc: 'Continue (debugging) / Run',
    toggleBookmark: 'Toggle Bookmark',
    nextBookmark: 'Next Bookmark',
    prevBookmark: 'Previous Bookmark',
  },
})

export {
  t,
  setLocale,
  getLocale,
  listLocales,
  registerMessages,
  getNestedValue,
  deepMerge,
  interpolate,
  DEFAULT_LOCALE,
  FALLBACK_LOCALE,
}
