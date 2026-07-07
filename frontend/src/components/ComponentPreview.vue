<template>
  <button v-if="comp.type === 'button'" class="real-control real-button" :style="btnStyle">{{ comp.text }}</button>
  <input
    v-else-if="comp.type === 'edit'"
    class="real-control real-edit"
    :style="editStyle"
    type="text"
    :placeholder="prop('placeholder')"
    readonly
  />
  <textarea
    v-else-if="comp.type === 'textarea'"
    class="real-control real-textarea"
    :style="editStyle"
    :placeholder="prop('placeholder')"
    readonly
  >{{ comp.text }}</textarea>
  <label v-else-if="comp.type === 'label'" class="real-control real-label" :style="labelStyle">{{ comp.text }}</label>
  <div v-else-if="comp.type === 'checkbox'" class="real-control real-check" :style="checkStyle">
    <input type="checkbox" tabindex="-1" :disabled="!comp.enabled" :checked="prop('checked')" />
    <span>{{ comp.text }}</span>
  </div>
  <div v-else-if="comp.type === 'radio'" class="real-control real-radio" :style="checkStyle">
    <input type="radio" tabindex="-1" :disabled="!comp.enabled" :checked="prop('checked')" />
    <span>{{ comp.text }}</span>
  </div>
  <select v-else-if="comp.type === 'listbox'" class="real-control real-listbox" :style="editStyle" size="4">
    <option v-for="item in itemList" :key="item">{{ item }}</option>
  </select>
  <select v-else-if="comp.type === 'combobox'" class="real-control real-combobox" :style="editStyle">
    <option v-for="item in itemList" :key="item">{{ item }}</option>
  </select>
  <div v-else-if="comp.type === 'switch'" class="real-control real-switch" :class="{ on: prop('checked') }" :style="switchStyle">
    <div class="switch-thumb" />
    <span>{{ comp.text }}</span>
  </div>
  <div v-else-if="comp.type === 'slider'" class="real-control real-slider" :style="sliderStyle">
    <div class="slider-track"><div class="slider-fill" :style="{ width: sliderPct + '%' }" /></div>
  </div>
  <div v-else-if="comp.type === 'progress'" class="real-control real-progress" :style="progressStyle">
    <div class="progress-track"><div class="progress-fill" :style="{ width: progressPct + '%' }" /></div>
  </div>
  <img v-else-if="comp.type === 'image'" class="real-control real-image" :src="prop('src') || ''" :style="imageStyle" />
  <div v-else-if="comp.type === 'tabs'" class="real-control real-tabs" :style="tabsStyle">
    <div class="tabs-header">
      <span v-for="item in itemList" :key="item" class="tab-item">{{ item }}</span>
    </div>
  </div>
  <div v-else-if="comp.type === 'card'" class="real-control real-card" :style="cardStyle">
    <div v-if="comp.text" class="card-header">{{ comp.text }}</div>
    <div class="card-body" />
  </div>
  <div v-else-if="comp.type === 'divider'" class="real-control real-divider" :class="{ dashed: prop('dashed'), vertical: prop('vertical') }" :style="dividerStyle">
    <span v-if="comp.text && !prop('vertical')" class="divider-title">{{ comp.text }}</span>
  </div>
  <!-- P3：外置组件通用占位渲染（未知类型统一用占位框，显示类型标签） -->
  <div v-else class="real-control real-external" :style="extStyle">
    <span class="external-label">{{ comp.type }}</span>
    <span v-if="comp.text" class="external-text">{{ comp.text }}</span>
  </div>
</template>

<script setup>
// 设计区组件预览 — 完全由组件属性决定样式，不引用任何 IDE 主题变量
// 这样在设计组件样式时不会受主题切换影响，所见即所得
import { computed } from 'vue'

const props = defineProps({
  comp: { type: Object, required: true }
})

// 读取组件属性，缺失时返回默认值
function prop(key, def = '') {
  return props.comp.props?.[key] ?? def
}

const itemList = computed(() => {
  const items = props.comp.items || props.comp.text || ''
  return items.split(/[\r\n]|\\n/).filter(Boolean)
})

// 中性默认值 — 不跟随主题
const NEUTRAL_BORDER = '#a0a0a0'
const NEUTRAL_BG = '#ffffff'
const NEUTRAL_TEXT = '#000000'
const NEUTRAL_FILL = '#3a8ee6'

// 按钮样式：颜色/边框/圆角/字重全部从属性读取
const btnStyle = computed(() => ({
  borderRadius: (prop('round') ? 9999 : (prop('borderRadius') || 0)) + 'px',
  borderWidth: (prop('borderWidth') || 1) + 'px',
  borderStyle: prop('dashed') ? 'dashed' : 'solid',
  borderColor: prop('borderColor') || NEUTRAL_BORDER,
  fontWeight: prop('fontWeight', 400),
  background: prop('ghost') ? 'transparent'
    : (prop('bgColor') || (prop('type') === 'primary' ? NEUTRAL_FILL : NEUTRAL_BG)),
  color: prop('color') || (prop('ghost') ? NEUTRAL_FILL
    : (prop('type') === 'primary' ? '#ffffff' : NEUTRAL_TEXT))
}))

// 输入框样式：背景/边框/文字色由属性决定
const editStyle = computed(() => ({
  background: prop('bgColor') || NEUTRAL_BG,
  borderColor: prop('borderColor') || NEUTRAL_BORDER,
  color: prop('color') || NEUTRAL_TEXT,
  borderRadius: (prop('borderRadius') || 0) + 'px'
}))

// 标签样式：对齐/字重/字体/行高/颜色
const labelStyle = computed(() => ({
  justifyContent: prop('textAlign', 'left'),
  fontWeight: prop('fontWeight', 400),
  fontFamily: prop('fontFamily') || undefined,
  lineHeight: prop('lineHeight') || undefined,
  color: prop('color') || NEUTRAL_TEXT
}))

// 复选/单选样式
const checkStyle = computed(() => ({
  color: prop('color') || NEUTRAL_TEXT
}))

// 开关样式
const switchStyle = computed(() => ({
  color: prop('color') || NEUTRAL_TEXT
}))

// 滑块样式：填充色/轨道色
const sliderStyle = computed(() => ({
  '--slider-fill': prop('color') || NEUTRAL_FILL,
  '--slider-track': prop('bgColor') || '#e0e0e0'
}))

// 进度条样式：填充色/轨道色
const progressStyle = computed(() => ({
  '--progress-fill': prop('color') || NEUTRAL_FILL,
  '--progress-track': prop('bgColor') || '#e0e0e0'
}))

// 图片样式
const imageStyle = computed(() => ({
  objectFit: prop('objectFit', 'contain'),
  borderRadius: (prop('borderRadius') || 0) + 'px'
}))

// 标签页样式
const tabsStyle = computed(() => ({
  borderColor: prop('borderColor') || NEUTRAL_BORDER,
  color: prop('color') || NEUTRAL_TEXT
}))

// 卡片样式
const cardStyle = computed(() => ({
  background: prop('bgColor') || NEUTRAL_BG,
  borderColor: prop('borderColor') || NEUTRAL_BORDER,
  borderRadius: '6px',
  color: prop('color') || NEUTRAL_TEXT
}))

// 分隔线样式
const dividerStyle = computed(() => ({
  '--divider-color': prop('borderColor') || NEUTRAL_BORDER,
  color: prop('color') || '#666666'
}))

// P3：外置组件通用占位样式（虚线边框 + 类型标签，提示用户这是外置组件）
const extStyle = computed(() => ({
  border: `1px dashed ${NEUTRAL_BORDER}`,
  borderRadius: '4px',
  background: 'rgba(128, 128, 128, 0.06)',
  color: '#666666',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  padding: '4px',
  fontSize: '11px',
  gap: '2px',
  overflow: 'hidden'
}))

const sliderPct = computed(() => {
  const min = Number(prop('min', 0))
  const max = Number(prop('max', 100))
  const val = Number(prop('value', min))
  if (max <= min) return 0
  return Math.max(0, Math.min(100, ((val - min) / (max - min)) * 100))
})

const progressPct = computed(() => {
  const v = Number(prop('percentage', 0))
  return Math.max(0, Math.min(100, v))
})
</script>

<style scoped>
/* 设计区组件预览 — 不引用任何 IDE 主题变量，全部使用中性值或组件属性 */
.real-control {
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  box-sizing: border-box;
  font-family: 'Microsoft YaHei', 'Segoe UI', sans-serif;
  font-size: inherit;
  color: #000000;
  background: #ffffff;
}
/* 编辑类控件在设计模式不需要响应输入，仅展示外观 */
.real-edit,
.real-textarea {
  pointer-events: none;
  user-select: none;
  border: 1px solid #a0a0a0;
  padding: 0 4px;
}
.real-textarea {
  padding: 4px;
  resize: none;
}
.real-button {
  border: 1px solid #a0a0a0;
  cursor: pointer;
  user-select: none;
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
.real-radio input {
  margin: 0;
}
.real-listbox,
.real-combobox {
  border: 1px solid #a0a0a0;
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
  background: #c0c0c0;
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
  background: #ffffff;
  transition: left 0.2s;
}
.real-switch.on .switch-thumb {
  background: #3a8ee6;
}
.real-switch.on .switch-thumb::after {
  left: 16px;
}
.real-slider {
  display: flex;
  align-items: center;
  padding: 0 4px;
  background: transparent;
}
.slider-track {
  flex: 1;
  height: 4px;
  background: var(--slider-track, #e0e0e0);
  border-radius: 2px;
  overflow: hidden;
}
.slider-fill {
  height: 100%;
  background: var(--slider-fill, #3a8ee6);
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
  background: var(--progress-track, #e0e0e0);
  border-radius: 4px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: var(--progress-fill, #3a8ee6);
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
  border-bottom: 1px solid #a0a0a0;
  gap: 4px;
}
.tab-item {
  padding: 4px 10px;
  font-size: 12px;
  border: 1px solid #a0a0a0;
  border-bottom: none;
  background: #f0f0f0;
}
.real-card {
  display: flex;
  flex-direction: column;
  border: 1px solid #a0a0a0;
  border-radius: 6px;
  overflow: hidden;
}
.card-header {
  padding: 6px 10px;
  font-weight: 600;
  border-bottom: 1px solid #e0e0e0;
  background: #f0f0f0;
}
.card-body {
  flex: 1;
}
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
  background: var(--divider-color, #a0a0a0);
}
.real-divider.dashed::before,
.real-divider.dashed::after {
  background: repeating-linear-gradient(90deg, var(--divider-color, #a0a0a0) 0 4px, transparent 4px 8px);
}
.divider-title {
  padding: 0 8px;
  font-size: 12px;
  color: #666666;
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
</style>
