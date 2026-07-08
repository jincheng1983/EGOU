import { createApp } from 'vue'
import naive from 'naive-ui'
import App from './App.vue'
import './style.css'
import './i18n/index.js'  // 初始化 i18n（注册 zh-CN/en-US 字典 + 恢复 localStorage 语言）
import { IDEService } from '../bindings/egou/internal/app'

// 把 IDEService 绑定到 window，供所有用 window.IDEService.xxx 的地方调用
// （未绑定会导致 SetBuildOptions / OpenInExplorer / ScanGlobalTemplates 等全部静默失败）
window.IDEService = IDEService

// 启动时恢复 IDE 界面字体设置（避免首屏闪烁）
try {
  const uiFont = localStorage.getItem('eg-uifont')
  if (uiFont) document.documentElement.style.setProperty('--ide-font', uiFont)
  const codeFont = localStorage.getItem('eg-fontfamily')
  if (codeFont) document.documentElement.style.setProperty('--ide-code-font', codeFont)
  // 界面字号：恢复 --ide-font-size 及衍生层级（sm/xs/lg/xl 按基础字号偏移）
  const savedSize = parseInt(localStorage.getItem('eg-uifontsize'), 10)
  if (savedSize >= 11 && savedSize <= 18) {
    const root = document.documentElement.style
    root.setProperty('--ide-font-size', savedSize + 'px')
    root.setProperty('--ide-font-size-sm', (savedSize - 1) + 'px')
    root.setProperty('--ide-font-size-xs', (savedSize - 2) + 'px')
    root.setProperty('--ide-font-size-lg', (savedSize + 1) + 'px')
    root.setProperty('--ide-font-size-xl', (savedSize + 2) + 'px')
  }
} catch {}

// 配置 Monaco Worker
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
self.MonacoEnvironment = {
  getWorker() {
    return new editorWorker()
  }
}

// 全局错误捕获：把JS错误显示在页面上，方便调试
window.addEventListener('error', (e) => {
  console.error('[全局错误]', e.error || e.message)
  const div = document.createElement('div')
  div.style.cssText = 'position:fixed;top:0;left:0;right:0;z-index:99999;background:#e74c3c;color:white;padding:8px 12px;font-size:12px;font-family:monospace;white-space:pre-wrap;max-height:200px;overflow:auto;'
  div.textContent = '[JS错误] ' + (e.error?.stack || e.message)
  document.body.appendChild(div)
})

window.addEventListener('unhandledrejection', (e) => {
  console.error('[未处理Promise]', e.reason)
  const div = document.createElement('div')
  div.style.cssText = 'position:fixed;top:0;left:0;right:0;z-index:99999;background:#e67e22;color:white;padding:8px 12px;font-size:12px;font-family:monospace;white-space:pre-wrap;max-height:200px;overflow:auto;'
  div.textContent = '[Promise错误] ' + (e.reason?.stack || String(e.reason))
  document.body.appendChild(div)
})

createApp(App).use(naive).mount('#app')
