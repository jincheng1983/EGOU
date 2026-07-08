<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { UIService } from '../bindings/egruntime'

interface ComponentData {
  type: string
  name: string
  text: string
  items: string
  x: number
  y: number
  width: number
  height: number
  visible: boolean
  enabled: boolean
  fontSize: number
  color: string
  bgColor: string
  props?: Record<string, any>
}

interface WindowState {
  title: string
  icon: string
  x: number
  y: number
  width: number
  height: number
  minWidth: number
  minHeight: number
  maxWidth: number
  maxHeight: number
  resizable: boolean
  minimizable: boolean
  maximizable: boolean
  fullScreen: boolean
  alwaysOnTop: boolean
  frameless: boolean
  transparent: boolean
  translucent: boolean
  backdrop: string
  rounded: boolean
  shadow: boolean
  opacity: number
  centered: boolean
  bgColor: string
  components: ComponentData[]
}

const state = ref<WindowState>({
  title: '',
  icon: '',
  x: 0, y: 0,
  width: 538, height: 350,
  minWidth: 120, minHeight: 80,
  maxWidth: 0, maxHeight: 0,
  resizable: true, minimizable: true, maximizable: true,
  fullScreen: false, alwaysOnTop: false,
  frameless: false, transparent: false, translucent: false,
  backdrop: 'auto', rounded: true, shadow: true,
  opacity: 1, centered: true,
  bgColor: '#ffffff',
  components: []
})

function titlebarStyle() {
  // 亚克力：让标题栏也透出来，否则 85% 白色会把毛玻璃盖住
  if (state.value.backdrop === 'acrylic') {
    return {
      background: 'rgba(240, 240, 240, 0.55)',
      backdropFilter: 'blur(20px) saturate(140%)',
      webkitBackdropFilter: 'blur(20px) saturate(140%)',
      border: 'none',
      borderBottom: '1px solid rgba(0, 0, 0, 0.08)'
    }
  }
  const style: Record<string, any> = {
    background: 'rgba(240, 240, 240, 0.85)',
    backdropFilter: 'blur(8px)',
    border: 'none',
    borderBottom: '1px solid rgba(0, 0, 0, 0.08)'
  }
  if (['mica', 'tabbed'].includes(state.value.backdrop)) {
    style.background = 'rgba(240, 240, 240, 0.7)'
    style.backdropFilter = 'blur(16px)'
  }
  return style
}

// 把效果强度 1-100 截断到合法区间
function clampRuntimeIntensity(v: any): number {
  const n = Number(v)
  if (Number.isNaN(n)) return 100
  return Math.max(1, Math.min(100, n))
}

// mica/tabbed 在 Wails v3 alpha2.110 上 DWMWA_SYSTEMBACKDROP_TYPE 多数系统不生效，
// 用 CSS 模拟设计器效果；acrylic 走真实 Wails Acrylic（DWM 桌面毛玻璃，Win11 22H2+ 有效）。
// intensity 1~100 → alpha 0.01~1.0，强度越大越透明。
function windowStyle() {
  const intensity = clampRuntimeIntensity(state.value.opacity)
  const backdrop = state.value.backdrop || 'auto'
  const style: Record<string, any> = {
    width: '100%',
    height: '100%',
    // 默认白底，兼容 auto/none 等无效果的情况
    backgroundColor: state.value.bgColor || '#ffffff',
  }
  if (state.value.shadow === false) {
    style.boxShadow = 'none'
  } else {
    style.boxShadow = '0 4px 20px rgba(0, 0, 0, 0.25)'
  }
  if (state.value.rounded) {
    style.borderRadius = '8px'
  } else {
    style.borderRadius = '0'
  }
  if (backdrop === 'mica') {
    // 云母：CSS 模拟浅蓝半透明
    const a = (intensity / 100).toFixed(2)
    style.background = `linear-gradient(135deg, rgba(220, 232, 246, ${a}), rgba(198, 215, 234, ${a}))`
    style.backgroundColor = undefined as any
  } else if (backdrop === 'tabbed') {
    // 标签式：CSS 模拟灰白竖向渐变
    const a = (intensity / 100).toFixed(2)
    style.background = `linear-gradient(180deg, rgba(245, 245, 245, ${a}) 0%, rgba(232, 232, 232, ${a}) 100%)`
    style.backgroundColor = undefined as any
  } else if (backdrop === 'acrylic') {
    // 亚克力：交给 Wails 真实 DWM Acrylic（DwmSetWindowAttribute DWMWA_SYSTEMBACKDROP_TYPE=3），
    // 自身保持透明，强度仅作为微调给 BgColor 一点点 tint。
    style.background = 'transparent'
    style.backgroundColor = state.value.bgColor || 'transparent'
  }
  return style
}

function doMinimize() { UIService.MinimizeWindow() }
function doMaximize() { UIService.ToggleMaximizeWindow() }
function doClose() { UIService.CloseWindow() }

// 标题栏图标加载失败时回退到 IDE 默认图标
function onIconError(e: Event) {
  const img = e.target as HTMLImageElement
  if (img && !img.src.endsWith('/appicon.png')) {
    img.src = '/appicon.png'
  }
}

// 组件事件处理：透传到后端 UIService.HandleEvent，后端按 "组件名_事件名" 查找已注册处理器。
function handleComponentClick(c: ComponentData) {
  if (c.enabled === false) return
  UIService.HandleEvent(c.name, '被单击')
}
function handleComponentChange(c: ComponentData) {
  UIService.HandleEvent(c.name, '状态被改变')
}
function handleComponentFocus(c: ComponentData) {
  if (c.enabled === false) return
  UIService.HandleEvent(c.name, '获得焦点')
}
function handleComponentBlur(c: ComponentData) {
  if (c.enabled === false) return
  UIService.HandleEvent(c.name, '失去焦点')
}

// 解析 items 文本为数组。
function parseItems(items?: string) {
  if (!items) return []
  return items.split(/[\r\n]|\\n/).filter(Boolean)
}

function cprop(c: ComponentData, key: string, def: any = '') {
  return c.props?.[key] ?? def
}

// ===== 外置组件运行时渲染 =====
// externalComponents 存储从后端 GetEmbeddedComponents() 获取的外置组件配置。
// key 是组件类型（如 "datepicker"），value 包含 HTML 模板和事件映射。
// 由 runner.writeEmbeddedAssets 嵌入到 embeddedComponents，供前端渲染外置组件。
interface ComponentRuntimeConfig {
  html: string
  events: Record<string, string> // DOM 事件名 → EGOU 事件名
}
const externalComponents = ref<Record<string, ComponentRuntimeConfig>>({})

// isExternal 判断组件类型是否为外置组件（即不在内置 14 种类型中但有运行时配置）。
function isExternal(type: string): boolean {
  return !!externalComponents.value[type]
}

// renderExternalHTML 根据外置组件的 runtime.html 模板渲染组件 HTML。
// 模板中的 {{propName}} 占位符会被组件对应的属性值替换。
function renderExternalHTML(c: ComponentData): string {
  const cfg = externalComponents.value[c.type]
  if (!cfg) return ''
  let html = cfg.html
  // 替换 {{propName}} 占位符为属性值
  if (c.props) {
    for (const [k, v] of Object.entries(c.props)) {
      html = html.replaceAll('{{' + k + '}}', String(v ?? ''))
    }
  }
  // 替换 {{text}} 为组件文本
  html = html.replaceAll('{{text}}', c.text || '')
  return html
}

// handleExternalEvent 外置组件事件路由：根据 runtime.events 映射，
// 把 DOM 事件名（如 "change"）转换为 EGOU 事件名（如 "值被改变"），再透传给后端。
function handleExternalEvent(c: ComponentData, domEvent: string) {
  if (c.enabled === false) return
  const cfg = externalComponents.value[c.type]
  if (!cfg || !cfg.events) return
  const egEvent = cfg.events[domEvent]
  if (!egEvent) return
  UIService.HandleEvent(c.name, egEvent)
}
// ===== 外置组件运行时渲染结束 =====

// 各类型组件的内联样式，与设计器 ComponentPreview.vue 完全一致，保证 WYSIWYG。
function btnStyle(c: ComponentData) {
  return {
    borderRadius: (cprop(c, 'round') ? 9999 : cprop(c, 'borderRadius')) + 'px',
    borderWidth: (cprop(c, 'borderWidth') || 1) + 'px',
    fontWeight: cprop(c, 'fontWeight', 400),
    background: cprop(c, 'ghost') ? 'transparent' : (cprop(c, 'type') === 'primary' ? 'var(--accent-color)' : '#e1e1e1'),
    color: cprop(c, 'ghost') ? 'var(--accent-color)' : (cprop(c, 'type') === 'primary' ? '#fff' : 'inherit'),
    borderStyle: cprop(c, 'dashed') ? 'dashed' : 'solid'
  }
}
function labelStyle(c: ComponentData) {
  return {
    justifyContent: cprop(c, 'textAlign', 'left'),
    fontWeight: cprop(c, 'fontWeight', 400),
    fontFamily: cprop(c, 'fontFamily') || undefined,
    lineHeight: cprop(c, 'lineHeight') || undefined
  }
}
function sliderPct(c: ComponentData) {
  const min = Number(cprop(c, 'min', 0))
  const max = Number(cprop(c, 'max', 100))
  const val = Number(cprop(c, 'value', min))
  if (max <= min) return 0
  return Math.max(0, Math.min(100, ((val - min) / (max - min)) * 100))
}
function progressPct(c: ComponentData) {
  const v = Number(cprop(c, 'percentage', 0))
  return Math.max(0, Math.min(100, v))
}
function imageStyle(c: ComponentData) {
  return {
    objectFit: cprop(c, 'objectFit', 'contain'),
    borderRadius: (cprop(c, 'borderRadius') || 0) + 'px'
  }
}

function componentContainerStyle(c: ComponentData) {
  return {
    position: 'absolute' as const,
    left: c.x + 'px',
    top: c.y + 'px',
    width: c.width + 'px',
    height: c.height + 'px',
    fontSize: (c.fontSize || 12) + 'px',
    color: c.color || undefined,
    backgroundColor: c.bgColor || undefined,
    display: c.visible ? 'flex' : 'none',
    overflow: 'hidden'
  }
}

// applyState 防御性地应用窗口状态。
// wails v3 Events.On 回调直接接收数据（非 {data: ...} 结构），
// 但后端 ExecJS 注入的可能又是 {data: ...}，这里兼容两种格式。
function applyState(data: any) {
  const s = data?.data ?? data
  if (s && typeof s === 'object' && 'components' in s) {
    state.value = s as WindowState
  }
}

onMounted(async () => {
  // 主动获取初始状态，避免事件时序问题导致组件丢失
  try {
    const s = await UIService.GetState()
    if (s && s.components) {
      state.value = s as WindowState
    }
  } catch (e) {
    console.warn('GetState failed:', e)
  }

  // 加载外置组件运行时配置（由 runner.writeEmbeddedAssets 嵌入）
  try {
    const comps = await UIService.GetEmbeddedComponents()
    if (comps) {
      externalComponents.value = comps as Record<string, ComponentRuntimeConfig>
    }
  } catch (e) {
    // 无外置组件时正常忽略
  }

  // 监听后续状态更新（兼容多种事件参数格式）
  Events.On('ui:update', (ev: any) => {
    applyState(ev)
  })

  // 暴露全局函数，供后端 ExecJS 直接设置状态（双保险，绕过事件系统）
  ;(window as any).__setEgState__ = applyState

  // 禁用右键菜单
  document.addEventListener('contextmenu', (e) => e.preventDefault())

  // 禁用 F5/Ctrl+R 刷新
  document.addEventListener('keydown', (e) => {
    if (e.key === 'F5' || (e.ctrlKey && (e.key === 'r' || e.key === 'R'))) {
      e.preventDefault()
    }
  })
})
</script>

<template>
  <div class="runtime-window" :style="windowStyle()">
    <!-- 自定义标题栏 -->
    <div class="custom-titlebar" :style="titlebarStyle()">
      <img :src="state.icon || '/appicon.png'" class="titlebar-icon" @error="onIconError" />
      <span class="titlebar-title">{{ state.title }}</span>
      <div class="titlebar-controls">
        <button v-if="state.minimizable" class="titlebar-btn" @click="doMinimize">
          <svg width="10" height="1" viewBox="0 0 10 1"><rect width="10" height="1" fill="currentColor"/></svg>
        </button>
        <button v-if="state.maximizable" class="titlebar-btn" @click="doMaximize">
          <svg width="10" height="10" viewBox="0 0 10 10"><rect x="0.5" y="0.5" width="9" height="9" fill="none" stroke="currentColor" stroke-width="1"/></svg>
        </button>
        <button class="titlebar-btn titlebar-btn-close" @click="doClose">
          <svg width="10" height="10" viewBox="0 0 10 10"><line x1="0" y1="0" x2="10" y2="10" stroke="currentColor" stroke-width="1.2"/><line x1="10" y1="0" x2="0" y2="10" stroke="currentColor" stroke-width="1.2"/></svg>
        </button>
      </div>
    </div>
    <!-- 内容区域：使用原生 HTML 组件，与设计器 ComponentPreview 样式一致 -->
    <div class="runtime-content" :style="{ height: 'calc(100% - 32px)' }">
      <div
        v-for="c in state.components"
        :key="c.name"
        class="real-control-wrap"
        :style="componentContainerStyle(c)"
      >
        <button
          v-if="c.type === 'button'"
          class="real-control real-button"
          :style="btnStyle(c)"
          :disabled="!c.enabled"
          @click="handleComponentClick(c)"
        >{{ c.text }}</button>

        <input
          v-else-if="c.type === 'edit'"
          class="real-control real-edit"
          type="text"
          :placeholder="cprop(c, 'placeholder')"
          :value="c.text"
          :disabled="!c.enabled"
          @input="(e: any) => { c.text = e.target.value }"
          @change="handleComponentChange(c)"
          @focus="handleComponentFocus(c)"
          @blur="handleComponentBlur(c)"
        />

        <textarea
          v-else-if="c.type === 'textarea'"
          class="real-control real-textarea"
          :placeholder="cprop(c, 'placeholder')"
          :value="c.text"
          :disabled="!c.enabled"
          @input="(e: any) => { c.text = e.target.value }"
          @change="handleComponentChange(c)"
          @focus="handleComponentFocus(c)"
          @blur="handleComponentBlur(c)"
        />

        <label
          v-else-if="c.type === 'label'"
          class="real-control real-label"
          :style="labelStyle(c)"
        >{{ c.text }}</label>

        <label
          v-else-if="c.type === 'checkbox'"
          class="real-control real-check"
        >
          <input type="checkbox" tabindex="-1" :disabled="!c.enabled" :checked="!!cprop(c, 'checked')" />
          <span>{{ c.text }}</span>
        </label>

        <label
          v-else-if="c.type === 'radio'"
          class="real-control real-radio"
        >
          <input type="radio" tabindex="-1" :disabled="!c.enabled" :checked="!!cprop(c, 'checked')" />
          <span>{{ c.text }}</span>
        </label>

        <select
          v-else-if="c.type === 'listbox'"
          class="real-control real-listbox"
          size="4"
          :disabled="!c.enabled"
        >
          <option v-for="item in parseItems(c.items)" :key="item">{{ item }}</option>
        </select>

        <select
          v-else-if="c.type === 'combobox'"
          class="real-control real-combobox"
          :disabled="!c.enabled"
        >
          <option v-for="item in parseItems(c.items)" :key="item">{{ item }}</option>
        </select>

        <div
          v-else-if="c.type === 'switch'"
          class="real-control real-switch"
          :class="{ on: !!cprop(c, 'checked') }"
        >
          <div class="switch-thumb" />
          <span>{{ c.text }}</span>
        </div>

        <div
          v-else-if="c.type === 'slider'"
          class="real-control real-slider"
        >
          <div class="slider-track">
            <div class="slider-fill" :style="{ width: sliderPct(c) + '%' }" />
          </div>
        </div>

        <div
          v-else-if="c.type === 'progress'"
          class="real-control real-progress"
        >
          <div class="progress-track">
            <div class="progress-fill" :style="{ width: progressPct(c) + '%' }" />
          </div>
        </div>

        <img
          v-else-if="c.type === 'image'"
          class="real-control real-image"
          :src="cprop(c, 'src')"
          :alt="c.text"
          :style="imageStyle(c)"
        />

        <div
          v-else-if="c.type === 'tabs'"
          class="real-control real-tabs"
        >
          <div class="tabs-header">
            <span v-for="item in parseItems(c.items)" :key="item" class="tab-item">{{ item }}</span>
          </div>
        </div>

        <div
          v-else-if="c.type === 'card'"
          class="real-control real-card"
        >
          <div v-if="c.text" class="card-header">{{ c.text }}</div>
          <div class="card-body" />
        </div>

        <div
          v-else-if="c.type === 'divider'"
          class="real-control real-divider"
          :class="{ dashed: cprop(c, 'dashed'), vertical: cprop(c, 'vertical') }"
        >
          <span v-if="c.text && !cprop(c, 'vertical')" class="divider-title">{{ c.text }}</span>
        </div>

        <!-- 外置组件：根据 embeddedComponents 中的 runtime.html 模板渲染，事件委托路由 -->
        <div
          v-else-if="isExternal(c.type)"
          class="real-control real-external"
          :style="{ opacity: c.enabled === false ? 0.6 : 1 }"
          v-html="renderExternalHTML(c)"
          @click="handleExternalEvent(c, 'click')"
          @change="handleExternalEvent(c, 'change')"
          @focusin="handleExternalEvent(c, 'focus')"
          @focusout="handleExternalEvent(c, 'blur')"
        ></div>

        <span v-else class="real-control" :style="{ opacity: c.enabled === false ? 0.6 : 1 }">{{ c.text }}</span>
      </div>
    </div>
  </div>
</template>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body, #app { width: 100%; height: 100%; overflow: hidden; font-family: var(--ide-font, -apple-system, "Microsoft YaHei", sans-serif); }
</style>

<style scoped>
.runtime-window {
  position: relative;
  overflow: hidden;
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
}
.custom-titlebar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  -webkit-app-region: drag;
  --wails-draggable: drag;
  user-select: none;
  background: rgba(240, 240, 240, 0.85);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid rgba(0, 0, 0, 0.08);
  padding-left: 10px;
}
.titlebar-icon { width: 16px; height: 16px; margin-right: 6px; }
.titlebar-title {
  flex: 1;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #222;
}
.titlebar-controls {
  display: flex;
  height: 32px;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}
.titlebar-btn {
  width: 46px;
  height: 32px;
  border: none;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #333;
  transition: background 0.15s;
}
.titlebar-btn:hover { background: rgba(0,0,0,0.06); }
.titlebar-btn-close:hover { background: #e81123; color: #fff; }
.runtime-content {
  flex: 1;
  position: relative;
  overflow: auto;
}

/* 组件容器 + 各组件样式（与设计器 ComponentPreview.vue 保持一致） */
.real-control-wrap { pointer-events: auto; }
.real-control {
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
  font-family: inherit;
  font-size: inherit;
  color: inherit;
  background: inherit;
}
.real-button {
  border: 1px solid #adadad;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s, border-color 0.15s, transform 0.05s, filter 0.15s;
}
.real-button:not([disabled]):hover {
  border-color: var(--accent-color, #18a058);
  filter: brightness(0.97);
}
.real-button:not([disabled]):active {
  transform: translateY(1px);
  filter: brightness(0.93);
}
.real-edit {
  background: #ffffff;
  border: 1px solid #adadad;
  padding: 0 4px;
}
.real-textarea {
  background: #ffffff;
  border: 1px solid #adadad;
  padding: 4px;
  resize: none;
}
.real-label {
  display: flex;
  align-items: center;
  background: transparent;
  white-space: pre-wrap;
}
.real-check,
.real-radio {
  display: flex;
  align-items: center;
  gap: 4px;
  background: transparent;
}
.real-check input,
.real-radio input { margin: 0; }
.real-listbox,
.real-combobox {
  background: #ffffff;
  border: 1px solid #adadad;
}
.real-switch {
  display: flex;
  align-items: center;
  gap: 6px;
  background: transparent;
}
.switch-thumb {
  width: 32px;
  height: 18px;
  border-radius: 9px;
  background: #ccc;
  position: relative;
}
.switch-thumb::after {
  content: '';
  position: absolute;
  left: 2px;
  top: 2px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
  transition: left 0.2s;
}
.real-switch.on .switch-thumb { background: var(--accent-color, #18a058); }
.real-switch.on .switch-thumb::after { left: 16px; }
.real-slider {
  display: flex;
  align-items: center;
  padding: 0 4px;
  background: transparent;
}
.slider-track {
  flex: 1;
  height: 4px;
  background: #ddd;
  border-radius: 2px;
  overflow: hidden;
}
.slider-fill {
  height: 100%;
  background: var(--accent-color, #18a058);
}
.real-progress {
  display: flex;
  align-items: center;
  background: transparent;
  padding: 0 4px;
}
.progress-track {
  flex: 1;
  height: 8px;
  background: #eee;
  border-radius: 4px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: var(--accent-color, #18a058);
}
.real-image {
  object-fit: contain;
  background: transparent;
}
.real-tabs {
  display: flex;
  flex-direction: column;
  background: transparent;
}
.tabs-header {
  display: flex;
  border-bottom: 1px solid #d0d0d0;
  gap: 4px;
}
.tab-item {
  padding: 4px 10px;
  font-size: 12px;
  border: 1px solid #d0d0d0;
  border-bottom: none;
  background: #f5f5f5;
}
.real-card {
  display: flex;
  flex-direction: column;
  background: #fff;
  border: 1px solid #d0d0d0;
  border-radius: 6px;
  overflow: hidden;
}
.card-header {
  padding: 6px 10px;
  font-weight: 600;
  border-bottom: 1px solid #e0e0e0;
  background: #f7f7f7;
}
.card-body { flex: 1; }
.real-divider {
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
}
.real-divider::before,
.real-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: #d0d0d0;
}
.real-divider.dashed::before,
.real-divider.dashed::after {
  background: repeating-linear-gradient(90deg, #d0d0d0 0 4px, transparent 4px 8px);
}
.divider-title {
  padding: 0 8px;
  font-size: 12px;
  color: var(--text-secondary, #666);
}
.real-divider.vertical {
  flex-direction: column;
  width: 1px;
}
.real-divider.vertical::before,
.real-divider.vertical::after {
  width: 1px;
  height: auto;
  flex: 1;
}
/* 外置组件：容器填满，内部 HTML 由 runtime.html 模板决定 */
.real-external {
  width: 100%;
  height: 100%;
  box-sizing: border-box;
}
.real-external :where(input, div, select, button) {
  box-sizing: border-box;
}
</style>
