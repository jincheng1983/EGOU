/**
 * IDE 主题系统（单一数据源）。
 * 内置主题 + 用户自定义主题，均持久化到 localStorage。
 * 设计参考 NxEGO4 磨砂风格：半透明 rgba 背景 + 渐变叠加。
 *
 * 【P1-6 统一】所有主题变量（含背景/文字/边框/强调/渐变/语义色/滚动条/阴影）
 * 均在此文件统一定义。style.css 仅保留 :root 暗色 + [data-theme="light"] 作为
 * 初始加载回退，其值必须与下方 dark/light 主题保持同步。
 */

const STORAGE_KEY = 'egou-theme'
const CUSTOM_KEY = 'egou-custom-themes'

export const baseThemes = {
  dark: {
    label: '深色',
    isDark: true,
    variables: {
      '--bg-primary':      'rgba(30, 30, 30, 0.85)',
      '--bg-secondary':    'rgba(37, 37, 37, 0.82)',
      '--bg-tertiary':     'rgba(45, 45, 45, 0.78)',
      '--bg-hover':        'rgba(50, 50, 50, 0.88)',
      '--bg-active':       'rgba(56, 56, 56, 0.88)',
      '--bg-sidebar':      'rgba(25, 25, 25, 0.88)',
      '--bg-output':       'rgba(20, 20, 20, 0.90)',
      '--bg-input':        'rgba(30, 30, 30, 0.85)',
      '--toolbar-gradient':'linear-gradient(180deg, rgba(45,45,45,0.72) 0%, rgba(37,37,37,0.68) 100%)',
      '--card-gradient':   'linear-gradient(135deg, rgba(50,50,50,0.8) 0%, rgba(56,56,56,0.8) 100%)',
      '--border-color':    'rgba(255, 255, 255, 0.08)',
      '--border-light':    'rgba(255, 255, 255, 0.04)',
      '--accent-border':   'rgba(99, 226, 183, 0.3)',
      '--text-primary':    '#e0e0e0',
      '--text-secondary':  '#cccccc',
      '--text-muted':      '#888888',
      '--text-dim':        '#666666',
      '--text-faint':      '#555555',
      '--text-darker':     '#444444',
      '--accent-color':    '#63e2b7',
      '--accent-hover':    '#4fd6a8',
      '--accent-light':    '#7feed0',
      '--accent-bg':       'rgba(99, 226, 183, 0.15)',
      '--color-success':   '#22c55e',
      '--color-warning':   '#f59e0b',
      '--color-error':     '#f87171',
      '--color-info':      '#06b6d4',
      '--color-purple':    '#a855f7',
      '--scrollbar-thumb': 'rgba(255, 255, 255, 0.12)',
      '--scrollbar-hover': 'rgba(255, 255, 255, 0.2)',
      '--shadow-color':    'rgba(0, 0, 0, 0.5)',
      '--shadow-1':        '0 1px 3px rgba(0, 0, 0, 0.3)',
      '--shadow-2':        '0 4px 12px rgba(0, 0, 0, 0.4)',
      '--shadow-pop':      '0 8px 32px rgba(0, 0, 0, 0.5)',
      '--modal-mask-bg':   'rgba(0, 0, 0, 0.45)',
      // 圆角分级（不随主题变，但纳入单一数据源）
      '--radius-sm':       '4px',
      '--radius-md':       '6px',
      '--radius-lg':       '8px',
      '--radius-xl':       '12px',
      '--radius-2xl':      '16px',
    }
  },
  light: {
    label: '浅色',
    isDark: false,
    variables: {
      '--bg-primary':      'rgba(255, 255, 255, 0.88)',
      '--bg-secondary':    'rgba(248, 250, 252, 0.85)',
      '--bg-tertiary':     'rgba(241, 245, 249, 0.82)',
      '--bg-hover':        'rgba(241, 245, 249, 0.95)',
      '--bg-active':       'rgba(226, 232, 240, 0.95)',
      '--bg-sidebar':      'rgba(248, 250, 252, 0.88)',
      '--bg-output':       'rgba(241, 245, 249, 0.9)',
      '--bg-input':        'rgba(255, 255, 255, 0.9)',
      '--toolbar-gradient':'linear-gradient(180deg, rgba(248,250,252,0.72) 0%, rgba(241,245,249,0.68) 100%)',
      '--card-gradient':   'linear-gradient(135deg, rgba(241,245,249,0.8) 0%, rgba(226,232,240,0.8) 100%)',
      '--border-color':    'rgba(0, 0, 0, 0.08)',
      '--border-light':    'rgba(0, 0, 0, 0.04)',
      '--accent-border':   'rgba(24, 160, 88, 0.3)',
      '--text-primary':    '#1e293b',
      '--text-secondary':  '#475569',
      '--text-muted':      '#64748b',
      '--text-dim':        '#94a3b8',
      '--text-faint':      '#cbd5e1',
      '--text-darker':     '#e2e8f0',
      '--accent-color':    '#18a058',
      '--accent-hover':    '#16a34a',
      '--accent-light':    '#22c55e',
      '--accent-bg':       'rgba(24, 160, 88, 0.12)',
      '--color-success':   '#16a34a',
      '--color-warning':   '#d97706',
      '--color-error':     '#dc2626',
      '--color-info':      '#0891b2',
      '--color-purple':    '#9333ea',
      '--scrollbar-thumb': 'rgba(0, 0, 0, 0.15)',
      '--scrollbar-hover': 'rgba(0, 0, 0, 0.25)',
      '--shadow-color':    'rgba(0, 0, 0, 0.15)',
      '--shadow-1':        '0 1px 3px rgba(0, 0, 0, 0.1)',
      '--shadow-2':        '0 4px 12px rgba(0, 0, 0, 0.15)',
      '--shadow-pop':      '0 8px 32px rgba(0, 0, 0, 0.2)',
      '--modal-mask-bg':   'rgba(0, 0, 0, 0.35)',
      // 圆角分级（不随主题变，但纳入单一数据源）
      '--radius-sm':       '4px',
      '--radius-md':       '6px',
      '--radius-lg':       '8px',
      '--radius-xl':       '12px',
      '--radius-2xl':      '16px',
    }
  },
  ocean: {
    label: '海蓝',
    isDark: true,
    variables: {
      '--bg-primary':      'rgba(8, 16, 24, 0.85)',
      '--bg-secondary':    'rgba(12, 24, 40, 0.80)',
      '--bg-tertiary':     'rgba(16, 32, 52, 0.78)',
      '--bg-hover':        'rgba(20, 40, 64, 0.88)',
      '--bg-active':       'rgba(28, 52, 80, 0.88)',
      '--bg-sidebar':      'rgba(6, 14, 24, 0.85)',
      '--bg-output':       'rgba(4, 10, 18, 0.88)',
      '--bg-input':        'rgba(10, 22, 38, 0.82)',
      '--toolbar-gradient':'linear-gradient(180deg, rgba(16,32,52,0.72) 0%, rgba(12,24,40,0.68) 100%)',
      '--card-gradient':   'linear-gradient(135deg, rgba(20,40,64,0.8) 0%, rgba(28,52,80,0.8) 100%)',
      '--border-color':    'rgba(120, 180, 220, 0.14)',
      '--border-light':    'rgba(120, 180, 220, 0.06)',
      '--accent-border':   'rgba(79, 172, 254, 0.3)',
      '--text-primary':    '#e6f4ff',
      '--text-secondary':  '#b4d2eb',
      '--text-muted':      '#7a9db8',
      '--text-dim':        '#5a7a96',
      '--text-faint':      '#44617a',
      '--text-darker':     '#2e4254',
      '--accent-color':    '#4facfe',
      '--accent-hover':    '#3b9eff',
      '--accent-light':    '#6bc1ff',
      '--accent-bg':       'rgba(79, 172, 254, 0.15)',
      '--color-success':   '#22c55e',
      '--color-warning':   '#f59e0b',
      '--color-error':     '#f87171',
      '--color-info':      '#06b6d4',
      '--color-purple':    '#a855f7',
      '--scrollbar-thumb': 'rgba(120, 180, 220, 0.15)',
      '--scrollbar-hover': 'rgba(120, 180, 220, 0.25)',
      '--shadow-color':    'rgba(0, 0, 0, 0.5)',
      '--shadow-1':        '0 1px 3px rgba(0, 0, 0, 0.3)',
      '--shadow-2':        '0 4px 12px rgba(0, 0, 0, 0.4)',
      '--shadow-pop':      '0 8px 32px rgba(0, 0, 0, 0.5)',
      '--modal-mask-bg':   'rgba(0, 0, 0, 0.45)',
      // 圆角分级（不随主题变，但纳入单一数据源）
      '--radius-sm':       '4px',
      '--radius-md':       '6px',
      '--radius-lg':       '8px',
      '--radius-xl':       '12px',
      '--radius-2xl':      '16px',
    }
  },
  sunrise: {
    label: '朝阳',
    isDark: false,
    variables: {
      '--bg-primary':      'rgba(255, 248, 240, 0.88)',
      '--bg-secondary':    'rgba(255, 240, 224, 0.85)',
      '--bg-tertiary':     'rgba(255, 232, 208, 0.82)',
      '--bg-hover':        'rgba(255, 224, 192, 0.95)',
      '--bg-active':       'rgba(254, 215, 170, 0.95)',
      '--bg-sidebar':      'rgba(255, 244, 230, 0.88)',
      '--bg-output':       'rgba(255, 240, 224, 0.9)',
      '--bg-input':        'rgba(255, 248, 240, 0.9)',
      '--toolbar-gradient':'linear-gradient(180deg, rgba(255,240,224,0.72) 0%, rgba(255,232,208,0.68) 100%)',
      '--card-gradient':   'linear-gradient(135deg, rgba(255,232,208,0.8) 0%, rgba(254,215,170,0.8) 100%)',
      '--border-color':    'rgba(180, 83, 9, 0.10)',
      '--border-light':    'rgba(180, 83, 9, 0.05)',
      '--accent-border':   'rgba(249, 115, 22, 0.3)',
      '--text-primary':    '#451a03',
      '--text-secondary':  '#78350f',
      '--text-muted':      '#9a3412',
      '--text-dim':        '#c2410c',
      '--text-faint':      '#d97706',
      '--text-darker':     '#f59e0b',
      '--accent-color':    '#f97316',
      '--accent-hover':    '#ea580c',
      '--accent-light':    '#fb923c',
      '--accent-bg':       'rgba(249, 115, 22, 0.12)',
      '--color-success':   '#16a34a',
      '--color-warning':   '#d97706',
      '--color-error':     '#dc2626',
      '--color-info':      '#0891b2',
      '--color-purple':    '#9333ea',
      '--scrollbar-thumb': 'rgba(180, 83, 9, 0.15)',
      '--scrollbar-hover': 'rgba(180, 83, 9, 0.25)',
      '--shadow-color':    'rgba(180, 83, 9, 0.15)',
      '--shadow-1':        '0 1px 3px rgba(180, 83, 9, 0.1)',
      '--shadow-2':        '0 4px 12px rgba(180, 83, 9, 0.15)',
      '--shadow-pop':      '0 8px 32px rgba(180, 83, 9, 0.2)',
      '--modal-mask-bg':   'rgba(0, 0, 0, 0.4)',
      // 圆角分级（不随主题变，但纳入单一数据源）
      '--radius-sm':       '4px',
      '--radius-md':       '6px',
      '--radius-lg':       '8px',
      '--radius-xl':       '12px',
      '--radius-2xl':      '16px',
    }
  }
}

const themes = { ...baseThemes }

function loadCustomThemes() {
  try {
    const raw = localStorage.getItem(CUSTOM_KEY)
    if (raw) {
      const custom = JSON.parse(raw)
      Object.assign(themes, custom)
    }
  } catch {}
}

loadCustomThemes()

export function getThemes() {
  return themes
}

export function getThemeNames() {
  return Object.keys(themes)
}

export function getTheme(name) {
  return themes[name] || themes.dark
}

export function applyTheme(name) {
  const preset = themes[name]
  if (!preset) return null
  const root = document.documentElement
  for (const [key, value] of Object.entries(preset.variables)) {
    root.style.setProperty(key, value)
  }
  document.body.setAttribute('data-theme', name)
  try {
    localStorage.setItem(STORAGE_KEY, name)
  } catch {}
  return preset
}

export function getSavedThemeName() {
  try {
    return localStorage.getItem(STORAGE_KEY) || 'dark'
  } catch {
    return 'dark'
  }
}

export function saveCustomTheme(name, theme) {
  if (!name) return false
  themes[name] = theme
  try {
    const raw = localStorage.getItem(CUSTOM_KEY)
    const custom = raw ? JSON.parse(raw) : {}
    custom[name] = theme
    localStorage.setItem(CUSTOM_KEY, JSON.stringify(custom))
  } catch {}
  return true
}

export function deleteCustomTheme(name) {
  if (baseThemes[name]) return false
  delete themes[name]
  try {
    const raw = localStorage.getItem(CUSTOM_KEY)
    if (raw) {
      const custom = JSON.parse(raw)
      delete custom[name]
      localStorage.setItem(CUSTOM_KEY, JSON.stringify(custom))
    }
  } catch {}
  return true
}

export function isBuiltInTheme(name) {
  return Object.prototype.hasOwnProperty.call(baseThemes, name)
}
