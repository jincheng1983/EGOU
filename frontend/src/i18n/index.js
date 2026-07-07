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
