<template>
  <div class="debug-panel">
    <!-- 控制按钮栏 -->
    <div class="debug-toolbar">
      <button
        class="debug-btn"
        :class="{ active: !isDebugging }"
        :disabled="isDebugging || !projectPath"
        title="开始调试"
        @click="startDebug"
      >
        <span class="debug-btn-icon">▶</span>
      </button>
      <button class="debug-btn" :disabled="!isDebugging" title="继续 (F5)" @click="continueDebug">
        <span class="debug-btn-icon">⏵</span>
      </button>
      <button class="debug-btn" :disabled="!isDebugging" title="单步跳过 (F10)" @click="stepOver">
        <span class="debug-btn-icon">⏭</span>
      </button>
      <button class="debug-btn" :disabled="!isDebugging" title="单步进入 (F11)" @click="stepInto">
        <span class="debug-btn-icon">⏎</span>
      </button>
      <button class="debug-btn" :disabled="!isDebugging" title="单步跳出 (Shift+F11)" @click="stepOut">
        <span class="debug-btn-icon">⏮</span>
      </button>
      <button class="debug-btn stop" :disabled="!isDebugging" title="停止调试" @click="stopDebug">
        <span class="debug-btn-icon">⏹</span>
      </button>
      <span v-if="debugStatus" class="debug-status">{{ debugStatus }}</span>
    </div>

    <!-- 调用栈 -->
    <div class="debug-section">
      <div class="debug-section-header" @click="toggleSection('stack')">
        <span class="debug-section-title">调用栈</span>
        <span class="debug-section-count">{{ stacktrace.length }}</span>
      </div>
      <div v-show="sections.stack" class="debug-section-body">
        <div v-if="stacktrace.length === 0" class="debug-empty">暂无调用栈</div>
        <div
          v-for="(frame, i) in stacktrace"
          :key="i"
          class="debug-stack-frame"
          :class="{ active: i === selectedFrame }"
          @click="selectFrame(i)"
        >
          <span class="debug-frame-fn">{{ frame.function?.name || '?' }}</span>
          <span class="debug-frame-loc">{{ frame.file }}:{{ frame.line }}</span>
        </div>
      </div>
    </div>

    <!-- 变量 -->
    <div class="debug-section">
      <div class="debug-section-header" @click="toggleSection('vars')">
        <span class="debug-section-title">变量</span>
        <span class="debug-section-count">{{ allVars.length }}</span>
      </div>
      <div v-show="sections.vars" class="debug-section-body">
        <div v-if="allVars.length === 0" class="debug-empty">暂无变量</div>
        <div v-for="v in allVars" :key="v._key" class="debug-var">
          <span class="debug-var-name">{{ v.name }}</span>
          <span class="debug-var-value" :title="v.value">{{ v.value }}</span>
          <span class="debug-var-type">{{ v.type }}</span>
        </div>
      </div>
    </div>

    <!-- 断点列表 -->
    <div class="debug-section">
      <div class="debug-section-header" @click="toggleSection('bp')">
        <span class="debug-section-title">断点</span>
        <span class="debug-section-count">{{ breakpoints.length }}</span>
      </div>
      <div v-show="sections.bp" class="debug-section-body">
        <div v-if="breakpoints.length === 0" class="debug-empty">暂无断点（在编辑器行号左侧点击添加）</div>
        <div
          v-for="(bp, i) in breakpoints"
          :key="i"
          class="debug-bp-item"
          @click="$emit('jump-to', bp.file, bp.line)"
        >
          <span class="debug-bp-loc">{{ bp.file }}:{{ bp.line }}</span>
          <button class="debug-bp-del" title="删除断点" @click.stop="removeBreakpoint(i)">✕</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { IDEService } from '../../bindings/egou/internal/app'
import { Events } from '@wailsio/runtime'

const props = defineProps({
  projectPath: { type: String, default: '' }
})
const emit = defineEmits(['jump-to', 'debug-log', 'debug-started'])

const isDebugging = ref(false)
const debugStatus = ref('')
const stacktrace = ref([])
const variables = ref({ locals: [], arguments: [] })
const breakpoints = ref([])
const selectedFrame = ref(0)

const sections = reactive({ stack: true, vars: true, bp: false })

const allVars = computed(() => {
  const locals = (variables.value.locals || []).map((v, i) => ({ ...v, _key: 'l' + i, _kind: '局部' }))
  const args = (variables.value.arguments || []).map((v, i) => ({ ...v, _key: 'a' + i, _kind: '参数' }))
  return [...args, ...locals]
})

let offHalt, offExit, offLog, offError

onMounted(() => {
  offHalt = Events.On('debug:halt', (ev) => {
    isDebugging.value = true
    debugStatus.value = ev.data.exited ? '已退出' : (ev.data.stopReason || '已暂停')
    if (!ev.data.exited && ev.data.file) {
      emit('jump-to', ev.data.file, ev.data.line)
    }
    refreshStackAndVars()
  })
  offExit = Events.On('debug:exit', () => {
    isDebugging.value = false
    debugStatus.value = '程序已退出'
    stacktrace.value = []
    variables.value = { locals: [], arguments: [] }
  })
  offLog = Events.On('debug:log', (ev) => {
    // 调试输出转发到父组件（App.vue 的输出面板）
    emit('debug-log', ev.data.line)
  })
  offError = Events.On('debug:error', (ev) => {
    debugStatus.value = '错误: ' + ev.data.error
  })
})

onUnmounted(() => {
  offHalt?.()
  offExit?.()
  offLog?.()
  offError?.()
})

function toggleSection(key) {
  sections[key] = !sections[key]
}

async function startDebug() {
  if (!props.projectPath) return
  debugStatus.value = '编译中...'
  try {
    const bps = breakpoints.value.map(bp => ({ file: bp.file, line: bp.line }))
    await IDEService.StartDebug(props.projectPath, bps)
    isDebugging.value = true
    debugStatus.value = '已就绪'
    emit('debug-started')
  } catch (e) {
    debugStatus.value = '启动失败: ' + (e.message || e)
  }
}

async function stopDebug() {
  try {
    await IDEService.StopDebug()
  } catch (e) { /* ignore */ }
  isDebugging.value = false
  debugStatus.value = ''
  stacktrace.value = []
  variables.value = { locals: [], arguments: [] }
}

async function continueDebug() {
  debugStatus.value = '运行中...'
  try { await IDEService.DebugContinue() } catch (e) { debugStatus.value = '错误: ' + e }
}
async function stepOver() {
  debugStatus.value = '单步跳过...'
  try { await IDEService.DebugNext() } catch (e) { debugStatus.value = '错误: ' + e }
}
async function stepInto() {
  debugStatus.value = '单步进入...'
  try { await IDEService.DebugStep() } catch (e) { debugStatus.value = '错误: ' + e }
}
async function stepOut() {
  debugStatus.value = '单步跳出...'
  try { await IDEService.DebugStepOut() } catch (e) { debugStatus.value = '错误: ' + e }
}

async function refreshStackAndVars() {
  try {
    const stack = await IDEService.DebugStacktrace(20)
    stacktrace.value = stack || []
    selectedFrame.value = 0
    await refreshVars(0)
  } catch (e) { /* 调试器可能已退出 */ }
}

async function refreshVars(frame) {
  try {
    const vars = await IDEService.DebugVariables(frame)
    variables.value = vars || { locals: [], arguments: [] }
  } catch (e) { /* ignore */ }
}

async function selectFrame(i) {
  selectedFrame.value = i
  await refreshVars(i)
}

function addBreakpoint(file, line) {
  const exists = breakpoints.value.some(bp => bp.file === file && bp.line === line)
  if (!exists) {
    breakpoints.value.push({ file, line })
    if (isDebugging.value) {
      IDEService.DebugToggleBreakpoint(file, line).catch(() => {})
    }
  }
}

function removeBreakpoint(index) {
  const bp = breakpoints.value[index]
  breakpoints.value.splice(index, 1)
  if (isDebugging.value && bp) {
    IDEService.DebugToggleBreakpoint(bp.file, bp.line).catch(() => {})
  }
}

defineExpose({ addBreakpoint, removeBreakpoint, isDebugging, startDebug, stopDebug, continueDebug, stepOver, stepInto, stepOut })
</script>

<style scoped>
.debug-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  font-size: 12px;
}

.debug-toolbar {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-secondary);
  flex-shrink: 0;
}

.debug-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s;
}
.debug-btn:hover:not(:disabled) {
  background: var(--hover-color);
  color: var(--text-primary);
}
.debug-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.debug-btn.active {
  color: var(--accent-color);
}
.debug-btn.stop:hover:not(:disabled) {
  color: var(--error-color);
}
.debug-btn-icon {
  font-size: 13px;
  line-height: 1;
}

.debug-status {
  margin-left: 8px;
  color: var(--text-tertiary);
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.debug-section {
  border-bottom: 1px solid var(--border-color);
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.debug-section:last-child {
  border-bottom: none;
  flex: 1;
}

.debug-section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: var(--bg-secondary);
  cursor: pointer;
  user-select: none;
  flex-shrink: 0;
}
.debug-section-header:hover {
  background: var(--hover-color);
}

.debug-section-title {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.debug-section-count {
  color: var(--text-tertiary);
  font-size: 10px;
  background: var(--bg-tertiary);
  padding: 0 5px;
  border-radius: var(--radius-sm);
  min-width: 16px;
  text-align: center;
}

.debug-section-body {
  overflow-y: auto;
  flex: 1;
  min-height: 0;
  max-height: 200px;
}

.debug-empty {
  padding: 12px 16px;
  color: var(--text-tertiary);
  font-size: 11px;
  text-align: center;
}

.debug-stack-frame {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 3px 12px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-color-light);
}
.debug-stack-frame:hover {
  background: var(--hover-color);
}
.debug-stack-frame.active {
  background: var(--accent-bg);
  border-left: 2px solid var(--accent-color);
}
.debug-frame-fn {
  color: var(--text-primary);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.debug-frame-loc {
  color: var(--text-tertiary);
  font-size: 10px;
  margin-left: 8px;
  flex-shrink: 0;
}

.debug-var {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 12px;
  border-bottom: 1px solid var(--border-color-light);
}
.debug-var:hover {
  background: var(--hover-color);
}
.debug-var-name {
  color: var(--accent-color);
  font-weight: 500;
  min-width: 60px;
}
.debug-var-value {
  color: var(--text-primary);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono, monospace);
}
.debug-var-type {
  color: var(--text-tertiary);
  font-size: 10px;
  font-style: italic;
}

.debug-bp-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 3px 12px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-color-light);
}
.debug-bp-item:hover {
  background: var(--hover-color);
}
.debug-bp-loc {
  color: var(--text-primary);
  font-family: var(--font-mono, monospace);
  font-size: 11px;
}
.debug-bp-del {
  border: none;
  background: transparent;
  color: var(--text-tertiary);
  cursor: pointer;
  font-size: 12px;
  padding: 0 4px;
  border-radius: var(--radius-sm);
}
.debug-bp-del:hover {
  color: var(--error-color);
  background: var(--hover-color);
}
</style>
