<template>
  <div class="outline-panel" ref="panelRef">
    <div class="panel-header">
      <span class="header-title">{{ t('outline.title') }}</span>
      <div class="header-actions">
        <button
          class="view-toggle"
          :class="{ active: viewMode === 'list' }"
          :title="t('outline.listView')"
          @click="viewMode = 'list'"
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M2 3h12v2H2zm0 4h12v2H2zm0 4h12v2H2z"/></svg>
        </button>
        <button
          class="view-toggle"
          :class="{ active: viewMode === 'table' }"
          :title="t('outline.tableView')"
          @click="viewMode = 'table'"
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M2 2h12v12H2zm1 4v7h10V6zm0-1h10V3H3z"/></svg>
        </button>
      </div>
    </div>

    <!-- 表格视图：搜索框 + 详细列表 -->
    <div v-if="viewMode === 'table'" class="table-view">
      <div class="search-row">
        <input
          v-model="searchQuery"
          class="search-input"
          :placeholder="t('outline.searchPlaceholder')"
          type="text"
        />
      </div>
      <div class="table-content">
        <div v-if="filteredSymbols.length === 0" class="outline-empty">
          <n-empty :description="searchQuery ? t('outline.noMatch') : t('outline.empty')" size="small" />
        </div>
        <div v-else class="symbol-table">
          <div class="table-header">
            <div class="col-name">{{ t('outline.colName') }}</div>
            <div class="col-kind">{{ t('outline.colType') }}</div>
            <div class="col-params">{{ t('outline.colParams') }}</div>
            <div class="col-return">{{ t('outline.colReturn') }}</div>
            <div class="col-line">{{ t('outline.colLine') }}</div>
          </div>
          <div class="table-body">
            <div
              v-for="(sym, i) in filteredSymbols"
              :key="'sym-' + i"
              class="table-row"
              :class="{ active: currentFunctionName === sym.name }"
              @click="onSymbolClick(sym)"
              :title="sym.tooltip"
            >
              <div class="col-name">
                <span class="sym-icon" :class="sym.iconClass">{{ sym.iconText }}</span>
                <span class="sym-name">{{ sym.displayName }}</span>
              </div>
              <div class="col-kind">{{ sym.kindLabel }}</div>
              <div class="col-params">{{ sym.paramsCount }}</div>
              <div class="col-return">{{ sym.returnType || '—' }}</div>
              <div class="col-line">{{ sym.line }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 列表视图（原有） -->
    <div v-else class="outline-content">
      <div v-if="!parsed || parsed.functions.length === 0 && parsed.globalVars.length === 0 && parsed.constants.length === 0" class="outline-empty">
        <n-empty description="暂无大纲" size="small">
          <template #extra>
            <n-text depth="3" style="font-size: var(--ide-font-size-sm);">{{ t('outline.emptyDesc') }}</n-text>
          </template>
        </n-empty>
      </div>
      <template v-else>
        <div v-if="parsed.functions.length > 0" class="outline-section">
          <div class="section-title">{{ t('outline.functions', { count: parsed.functions.length }) }}</div>
          <div
            v-for="(fn, i) in parsed.functions"
            :key="'fn-' + i"
            class="outline-item"
            :class="{ active: currentFunctionName === fn.name }"
            @click="$emit('goto-function', fn)"
          >
            <span class="item-icon func-icon">ƒ</span>
            <span class="item-name">{{ fn.name }}</span>
            <span v-if="fn.params && fn.params.length" class="item-params">({{ fn.params.length }})</span>
          </div>
        </div>
        <div v-if="parsed.globalVars.length > 0" class="outline-section">
          <div class="section-title">{{ t('outline.variables', { count: parsed.globalVars.length }) }}</div>
          <div
            v-for="(v, i) in parsed.globalVars"
            :key="'var-' + i"
            class="outline-item"
            @click="$emit('goto-line', v.line + 1)"
          >
            <span class="item-icon var-icon">V</span>
            <span class="item-name">{{ v.name }}</span>
            <span v-if="v.type" class="item-type">{{ v.type }}</span>
          </div>
        </div>
        <div v-if="parsed.constants.length > 0" class="outline-section">
          <div class="section-title">{{ t('outline.constants', { count: parsed.constants.length }) }}</div>
          <div
            v-for="(c, i) in parsed.constants"
            :key="'const-' + i"
            class="outline-item"
            @click="$emit('goto-line', c.line + 1)"
          >
            <span class="item-icon const-icon">C</span>
            <span class="item-name">{{ c.name }}</span>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { NEmpty, NText } from 'naive-ui'
import { t } from '../i18n/index.js'

const props = defineProps({
  parsed: { type: Object, default: () => ({ functions: [], globalVars: [], constants: [] }) },
  currentFunctionName: { type: String, default: '' }
})

const emit = defineEmits(['goto-function', 'goto-line'])

const panelRef = ref(null)
const viewMode = ref(localStorage.getItem('eg-outline-view') || 'list')
const searchQuery = ref('')

watch(viewMode, (v) => localStorage.setItem('eg-outline-view', v))

// 表格视图：把 parsed 的所有符号合并为统一格式
const allSymbols = computed(() => {
  const out = []
  if (!props.parsed) return out
  // 函数/方法
  (props.parsed.functions || []).forEach(fn => {
    const isMethod = fn.kind === '方法'
    const displayName = isMethod && fn.receiverType
      ? `${fn.name}(${fn.receiverType})`
      : fn.name
    const paramTypes = (fn.params || []).map(p => p.type).filter(Boolean).join(', ')
    const paramSummary = (fn.params || []).map(p => p.name + (p.type ? ': ' + p.type : '')).join(', ')
    out.push({
      name: fn.name,
      displayName,
      kindLabel: isMethod ? t('outline.kindMethod') : t('outline.kindFunction'),
      iconClass: isMethod ? 'sym-method' : 'sym-func',
      iconText: isMethod ? 'M' : 'ƒ',
      paramsCount: (fn.params || []).length,
      returnType: fn.returnType || '',
      line: fn.startLine + 1,
      tooltip: `${isMethod ? '方法' : '函数'} ${fn.name}(${paramSummary})${fn.returnType ? ' → ' + fn.returnType : ''}`,
      raw: fn
    })
  })
  // 全局变量
  ;(props.parsed.globalVars || []).forEach(v => {
    out.push({
      name: v.name,
      displayName: v.name,
      kindLabel: t('outline.kindVariable'),
      iconClass: 'sym-var',
      iconText: 'V',
      paramsCount: '—',
      returnType: v.type || '',
      line: v.line + 1,
      tooltip: `变量 ${v.name}${v.type ? ': ' + v.type : ''}`,
      raw: v,
      isVar: true
    })
  })
  // 常量
  ;(props.parsed.constants || []).forEach(c => {
    out.push({
      name: c.name,
      displayName: c.name,
      kindLabel: t('outline.kindConstant'),
      iconClass: 'sym-const',
      iconText: 'C',
      paramsCount: '—',
      returnType: c.type || '',
      line: c.line + 1,
      tooltip: `常量 ${c.name}${c.type ? ': ' + c.type : ''}`,
      raw: c,
      isConst: true
    })
  })
  return out
})

// 搜索过滤
const filteredSymbols = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return allSymbols.value
  return allSymbols.value.filter(s =>
    s.name.toLowerCase().includes(q) ||
    s.kindLabel.toLowerCase().includes(q) ||
    (s.returnType || '').toLowerCase().includes(q)
  )
})

function onSymbolClick(sym) {
  if (sym.isVar || sym.isConst) {
    emit('goto-line', sym.line)
  } else {
    emit('goto-function', sym.raw)
  }
}

watch(() => props.currentFunctionName, (name) => {
  if (!name) return
  nextTick(() => {
    const panel = panelRef.value
    if (!panel) return
    const active = panel.querySelector('.outline-item.active, .table-row.active')
    if (!active) return
    const content = panel.querySelector('.outline-content, .table-content')
    if (!content) return
    const aRect = active.getBoundingClientRect()
    const cRect = content.getBoundingClientRect()
    if (aRect.top < cRect.top + 4 || aRect.bottom > cRect.bottom - 4) {
      active.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  })
})
</script>

<style scoped>
.outline-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background-color: transparent;
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  font-size: var(--ide-font-size);
  font-weight: 600;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  background: var(--toolbar-gradient);
}
.header-title {
  flex: 1;
}
.header-actions {
  display: flex;
  gap: 2px;
}
.view-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  border-radius: 4px;
  transition: all 0.12s;
}
.view-toggle:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
.view-toggle.active {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  color: var(--accent-color);
}

/* 列表视图 */
.outline-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 4px 0;
}
.outline-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 20px;
}
.outline-section {
  margin-bottom: 8px;
}
.outline-section:last-child {
  margin-bottom: 0;
}
.section-title {
  padding: 6px 12px 4px;
  font-size: var(--ide-font-size-xs);
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  user-select: none;
}
.outline-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  cursor: pointer;
  transition: background 0.12s;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.outline-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
.outline-item.active {
  background: color-mix(in srgb, var(--accent-color) 12%, transparent);
  color: var(--accent-color);
  font-weight: 500;
}
.item-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 700;
  flex-shrink: 0;
}
.func-icon {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  color: var(--accent-color);
}
.var-icon {
  background: color-mix(in srgb, var(--color-info, #179fff) 15%, transparent);
  color: var(--color-info, #179fff);
}
.const-icon {
  background: color-mix(in srgb, var(--color-warning) 15%, transparent);
  color: var(--color-warning);
}
.item-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}
.item-params {
  font-size: var(--ide-font-size-xs);
  color: var(--text-muted);
  flex-shrink: 0;
}
.item-type {
  font-size: var(--ide-font-size-xs);
  color: var(--text-muted);
  flex-shrink: 0;
}

/* 表格视图 */
.table-view {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.search-row {
  padding: 6px 8px;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}
.search-input {
  width: 100%;
  padding: 4px 8px;
  font-size: var(--ide-font-size-sm);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-input, var(--bg-secondary));
  color: var(--text-primary);
  outline: none;
  transition: border-color 0.12s;
  box-sizing: border-box;
}
.search-input:focus {
  border-color: var(--accent-color);
}
.search-input::placeholder {
  color: var(--text-muted);
}
.table-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
}
.symbol-table {
  font-size: var(--ide-font-size-xs);
}
.table-header {
  display: grid;
  grid-template-columns: 1fr 40px 40px 60px 36px;
  gap: 4px;
  padding: 6px 8px;
  font-weight: 600;
  color: var(--text-muted);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  position: sticky;
  top: 0;
  z-index: 1;
  text-transform: uppercase;
  font-size: 10px;
  letter-spacing: 0.3px;
}
.table-body {
  display: flex;
  flex-direction: column;
}
.table-row {
  display: grid;
  grid-template-columns: 1fr 40px 40px 60px 36px;
  gap: 4px;
  padding: 5px 8px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: background 0.12s;
  border-bottom: 1px solid color-mix(in srgb, var(--border-color) 30%, transparent);
  align-items: center;
}
.table-row:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
.table-row.active {
  background: color-mix(in srgb, var(--accent-color) 12%, transparent);
  color: var(--accent-color);
}
.col-name {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.col-kind, .col-params, .col-return, .col-line {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
  font-size: 10px;
  color: var(--text-muted);
}
.col-line {
  font-variant-numeric: tabular-nums;
}
.sym-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 3px;
  font-size: 9px;
  font-weight: 700;
  flex-shrink: 0;
}
.sym-func {
  background: color-mix(in srgb, var(--accent-color) 15%, transparent);
  color: var(--accent-color);
}
.sym-method {
  background: color-mix(in srgb, var(--color-success, #18a058) 15%, transparent);
  color: var(--color-success, #18a058);
}
.sym-var {
  background: color-mix(in srgb, var(--color-info, #179fff) 15%, transparent);
  color: var(--color-info, #179fff);
}
.sym-const {
  background: color-mix(in srgb, var(--color-warning) 15%, transparent);
  color: var(--color-warning);
}
.sym-name {
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
