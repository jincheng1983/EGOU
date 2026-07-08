<template>
  <div class="project-explorer" @contextmenu="onContainerContextMenu">
    <div class="explorer-header">
      <n-text depth="3" class="tree-label">{{ t('projectTree.project') }}</n-text>
      <n-button size="tiny" quaternary :title="t('projectTree.refresh')" @click="$emit('refresh')">
        <n-icon :component="RefreshOutline" />
      </n-button>
    </div>
    <n-tree
      v-if="treeData.length"
      ref="treeRef"
      class="tree-flow"
      :class="{ dim: dimLines }"
      :data="treeData"
      :default-expand-all="false"
      :default-expanded-keys="defaultExpandedKeys"
      :expanded-keys="expandedKeys"
      :selected-keys="selectedKeys"
      :render-prefix="renderPrefix"
      :render-suffix="renderSuffix"
      :node-props="nodeProps"
      selectable
      block-line
      show-line
      expand-on-click
      style="margin-top: 4px;"
      @update:expanded-keys="onExpandedKeys"
      @update:selected-keys="onSelect"
    />
    <n-empty v-else :description="t('projectTree.empty')" size="small" style="margin-top: 24px;" />
    <n-dropdown
      :show="contextMenuShow"
      :options="contextMenuOptions"
      :x="contextMenuX"
      :y="contextMenuY"
      placement="bottom-start"
      trigger="manual"
      @select="onContextMenuSelect"
      @clickoutside="contextMenuShow = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, h, watch, nextTick } from 'vue'
import { NTree, NText, NButton, NIcon, NEmpty, NDropdown } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { TYPE_COLOR, FLOW_RAINBOW } from '../utils/colors.js'
import { getCurrentProjectRoot } from '../lib/index.js'
import { t } from '../i18n/index.js'

const props = defineProps({
  files: {
    type: Array,
    default: () => []
  },
  libs: {
    type: Array,
    default: () => []
  },
  projectName: {
    type: String,
    default: ''
  },
  currentFilePath: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['open-file', 'refresh', 'new-window', 'new-module', 'new-class', 'new-code-file', 'delete-file', 'expand-all', 'collapse-all'])

const NODE_TYPE = {
  SOURCE: 'source',
  WINDOW: 'window',
  MODULE: 'module',
  CLASS: 'class',
  DLL: 'dll',
  RESOURCE: 'resource',
  RES_SOUND: 'sound',
  RES_IMAGE: 'image',
  LIB_REF: 'libref',
  SRC_FILE: 'srcfile',
}

const TYPE_ICON = {
  source:     '📄',
  srcfile:    '📄',
  window:     '🪟',
  module:     '🧩',
  class:      '🧱',
  dll:        '🔗',
  resource:   '🖼️',
  sound:      '🔊',
  image:      '🖼️',
  libref:     '📦',
}

const treeRef = ref(null)
const expandedKeys = ref(['source-root', 'windows-root'])

function fileExtension(name) {
  const m = (name || '').toLowerCase().match(/\.([a-z0-9]+)$/)
  return m ? m[1] : ''
}

function isInDir(path, dir) {
  return new RegExp('[\\\\/]' + dir + '[\\\\/]', 'i').test(path || '')
}

function collect(nodes, predicate, out) {
  for (const node of nodes || []) {
    if (!node.isDir && predicate(node)) out.push(node)
    if (node.isDir && node.children) collect(node.children, predicate, out)
  }
}

function injectLevel(nodes, level = 0) {
  return nodes.map(node => ({
    ...node,
    __level: level,
    children: node.children && node.children.length
      ? injectLevel(node.children, level + 1)
      : node.children
  }))
}

function renderPrefix(info) {
  const opt = info.option || {}
  const t = opt.nodeType
  if (t && TYPE_COLOR[t]) {
    const icon = TYPE_ICON[t] || '·'
    return h('span', { class: 'node-prefix', style: { color: TYPE_COLOR[t] } }, icon)
  }
  return h('span', { class: 'node-prefix' }, '📄')
}

function renderSuffix(info) {
  const opt = info.option || {}
  if (opt.suffix) {
    return h('span', { class: 'node-suffix' }, opt.suffix)
  }
  return null
}

function isSourceFile(f) {
  if (fileExtension(f.name) !== 'eg') return false
  return !isInDir(f.path, 'modules') && !isInDir(f.path, 'types')
}

function getFileNodeKey(filePath) {
  if (!filePath) return null
  const name = filePath.split(/[\\/]/).pop() || ''
  const ext = fileExtension(name)
  if (ext === 'ew') return 'win-' + filePath
  if (ext === 'eg') {
    if (isInDir(filePath, 'modules')) return 'mod-' + filePath
    if (isInDir(filePath, 'types')) return 'cls-' + filePath
    return 'src-' + filePath
  }
  if (isInDir(filePath, 'native')) return 'dll-' + filePath
  if (['wav', 'mp3', 'ogg', 'flac', 'midi'].includes(ext)) return 'snd-' + filePath
  if (['png', 'jpg', 'jpeg', 'bmp', 'gif', 'ico', 'svg', 'webp'].includes(ext)) return 'img-' + filePath
  return null
}

function getCategoryKeyForPath(filePath) {
  if (!filePath) return null
  const name = filePath.split(/[\\/]/).pop() || ''
  const ext = fileExtension(name)
  if (ext === 'ew') return 'windows-root'
  if (ext === 'eg') {
    if (isInDir(filePath, 'modules')) return 'modules-root'
    if (isInDir(filePath, 'types')) return 'classes-root'
    return 'source-root'
  }
  if (isInDir(filePath, 'native')) return 'dll-root'
  if (isInDir(filePath, 'assets') || isInDir(filePath, 'resource')) {
    return 'resources-root'
  }
  return null
}

const treeData = computed(() => {
  const roots = []
  if (!props.projectName) return roots

  const sourceFiles = []
  const windows = []
  const modules = []
  const classes = []
  const resources = []
  const sounds = []
  const images = []
  const nativeFiles = []

  collect(props.files, n => isSourceFile(n), sourceFiles)
  collect(props.files, n => fileExtension(n.name) === 'ew', windows)
  collect(props.files, n => fileExtension(n.name) === 'eg' && isInDir(n.path, 'modules'), modules)
  collect(props.files, n => fileExtension(n.name) === 'eg' && isInDir(n.path, 'types'), classes)
  collect(props.files, n => isInDir(n.path, 'assets') || isInDir(n.path, 'resource') || isInDir(n.path, '资源'), resources)
  collect(props.files, n => isInDir(n.path, 'native'), nativeFiles)

  for (const r of resources) {
    const ext = fileExtension(r.name)
    if (['wav', 'mp3', 'ogg', 'flac', 'midi'].includes(ext)) sounds.push(r)
    else if (['png', 'jpg', 'jpeg', 'bmp', 'gif', 'ico', 'svg', 'webp'].includes(ext)) images.push(r)
  }

  // 1. 源码文件
  roots.push({
    label: t('projectTree.sourceFiles'),
    key: 'source-root',
    nodeType: NODE_TYPE.SOURCE,
    categoryKey: 'source',
    isCategory: true,
    isDir: true,
    children: injectLevel(sourceFiles.map(f => ({
      label: f.name,
      key: `src-${f.path}`,
      nodeType: NODE_TYPE.SRC_FILE,
      isDir: false,
      path: f.path
    })))
  })

  // 2. Dll命令
  roots.push({
    label: t('projectTree.dllCommands'),
    key: 'dll-root',
    nodeType: NODE_TYPE.DLL,
    categoryKey: 'dll',
    isCategory: true,
    isDir: true,
    children: injectLevel(nativeFiles.map(f => ({
      label: f.name,
      key: `dll-${f.path}`,
      nodeType: NODE_TYPE.DLL,
      isDir: false,
      path: f.path
    })))
  })

  // 3. 窗口
  roots.push({
    label: t('projectTree.windows'),
    key: 'windows-root',
    nodeType: NODE_TYPE.WINDOW,
    categoryKey: 'windows',
    isCategory: true,
    isDir: true,
    children: injectLevel(windows.map(w => ({
      label: w.name,
      key: `win-${w.path}`,
      nodeType: NODE_TYPE.WINDOW,
      isDir: false,
      path: w.path
    })))
  })

  // 4. 资源表
  {
    const resChildren = []
    if (sounds.length) {
      resChildren.push({
        label: t('projectTree.sound'),
        key: 'res-sound-root',
        nodeType: NODE_TYPE.SOUND,
        isDir: true,
        children: injectLevel(sounds.map(s => ({
          label: s.name,
          key: `snd-${s.path}`,
          nodeType: NODE_TYPE.SOUND,
          isDir: false,
          path: s.path
        })))
      })
    }
    if (images.length) {
      resChildren.push({
        label: t('projectTree.images'),
        key: 'res-image-root',
        nodeType: NODE_TYPE.IMAGE,
        isDir: true,
        children: injectLevel(images.map(img => ({
          label: img.name,
          key: `img-${img.path}`,
          nodeType: NODE_TYPE.IMAGE,
          isDir: false,
          path: img.path
        })))
      })
    }
    roots.push({
      label: t('projectTree.resources'),
      key: 'resources-root',
      nodeType: NODE_TYPE.RESOURCE,
      categoryKey: 'resources',
      isCategory: true,
      isDir: true,
      children: injectLevel(resChildren, 0).map(n => ({ ...n, __level: 0 }))
    })
  }

  // 5. 模块引用表
  roots.push({
    label: t('projectTree.modulesRef'),
    key: 'libs-root',
    nodeType: NODE_TYPE.LIB_REF,
    categoryKey: 'libs',
    isCategory: true,
    isDir: true,
    children: injectLevel((props.libs || []).map(lib => ({
      label: (lib.displayName || lib.name) + (lib.version ? ' - ' + lib.version : ''),
      key: `lib-${lib.dir || lib.name}`,
      nodeType: NODE_TYPE.LIB_REF,
      isDir: false,
      libName: lib.name,
      suffix: lib.author ? lib.author : ''
    })))
  })

  // 6. 模块
  roots.push({
    label: t('projectTree.modules'),
    key: 'modules-root',
    nodeType: NODE_TYPE.MODULE,
    categoryKey: 'modules',
    isCategory: true,
    isDir: true,
    children: injectLevel(modules.map(m => ({
      label: m.name,
      key: `mod-${m.path}`,
      nodeType: NODE_TYPE.MODULE,
      isDir: false,
      path: m.path
    })))
  })

  // 7. 类
  roots.push({
    label: t('projectTree.classes'),
    key: 'classes-root',
    nodeType: NODE_TYPE.CLASS,
    categoryKey: 'classes',
    isCategory: true,
    isDir: true,
    children: injectLevel(classes.map(c => ({
      label: c.name,
      key: `cls-${c.path}`,
      nodeType: NODE_TYPE.CLASS,
      isDir: false,
      path: c.path
    })))
  })

  return injectLevel(roots)
})

const defaultExpandedKeys = computed(() => ['source-root', 'windows-root'])

function onExpandedKeys(keys) {
  expandedKeys.value = keys
}

function expandAll() {
  const allKeys = []
  function collectKeys(nodes) {
    for (const n of nodes) {
      if (n.isDir) {
        allKeys.push(n.key)
        if (n.children) collectKeys(n.children)
      }
    }
  }
  collectKeys(treeData.value)
  expandedKeys.value = allKeys
}

function collapseAll() {
  expandedKeys.value = []
}

const contextMenuShow = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuNode = ref(null)
const contextMenuIsBlank = ref(false)

const blankMenuOptions = computed(() => [
  { label: t('projectTree.newCodeFile'), key: 'new-code' },
  { label: t('projectTree.newWindow'), key: 'new-window' },
  { label: t('projectTree.newClass'), key: 'new-class' },
  { label: t('projectTree.newModule'), key: 'new-module' },
  { type: 'divider', key: 'd-blank' },
  { label: t('projectTree.expandAll'), key: 'expand-all' },
  { label: t('projectTree.collapseAll'), key: 'collapse-all' },
])

const contextMenuOptions = computed(() => {
  if (contextMenuIsBlank.value) return blankMenuOptions.value
  const node = contextMenuNode.value
  if (!node) return []
  const opts = []
  if (node.isCategory) {
    switch (node.categoryKey) {
      case 'source':
        opts.push({ label: t('projectTree.newCodeFile'), key: 'new-code' })
        break
      case 'windows':
        opts.push({ label: t('projectTree.newWindow'), key: 'new-window' })
        break
      case 'modules':
        opts.push({ label: t('projectTree.newModule'), key: 'new-module' })
        break
      case 'classes':
        opts.push({ label: t('projectTree.newClass'), key: 'new-class' })
        break
      case 'dll':
      case 'resources':
      case 'libs':
        break
    }
    if (opts.length > 0) opts.push({ type: 'divider' })
    opts.push({ label: t('projectTree.expandAll'), key: 'expand-all' })
    opts.push({ label: t('projectTree.collapseAll'), key: 'collapse-all' })
  } else {
    if (node.path) {
      opts.push({ label: t('projectTree.open'), key: 'open' })
      const deletable = node.nodeType === NODE_TYPE.SRC_FILE ||
        node.nodeType === NODE_TYPE.WINDOW ||
        node.nodeType === NODE_TYPE.MODULE ||
        node.nodeType === NODE_TYPE.CLASS ||
        node.nodeType === NODE_TYPE.SOUND ||
        node.nodeType === NODE_TYPE.IMAGE ||
        node.nodeType === NODE_TYPE.DLL
      if (deletable) {
        opts.push({ type: 'divider' })
        opts.push({ label: t('projectTree.delete'), key: 'delete' })
      }
    } else if (node.nodeType === NODE_TYPE.LIB_REF && node.libName) {
      // 规约 §6：.elib 节点支持 打开 / 重命名 / 删除
      opts.push({ label: t('projectTree.open'), key: 'open-elib' })
      opts.push({ type: 'divider' })
      opts.push({ label: t('projectTree.rename'), key: 'rename-elib' })
      opts.push({ label: t('projectTree.delete'), key: 'delete-elib' })
    }
  }
  return opts
})

function onContainerContextMenu(e) {
  if (e.target.closest('.n-tree-node-content-wrapper')) return
  if (e.target.closest('.explorer-header')) return
  if (!props.projectName) return
  e.preventDefault()
  contextMenuIsBlank.value = true
  contextMenuNode.value = null
  contextMenuX.value = e.clientX
  contextMenuY.value = e.clientY
  contextMenuShow.value = true
}

function nodeProps({ option }) {
  const level = option && option.__level != null ? option.__level : 0
  const color = FLOW_RAINBOW[level % FLOW_RAINBOW.length]
  const props = {
    'data-level': String(level),
    style: { '--flow-color': color, '--n-line-color': color }
  }
  props.onContextmenu = (e) => {
    e.preventDefault()
    e.stopPropagation()
    contextMenuIsBlank.value = false
    contextMenuNode.value = option
    contextMenuX.value = e.clientX
    contextMenuY.value = e.clientY
    contextMenuShow.value = true
  }
  return props
}

const selectedKeys = computed(() => {
  const key = getFileNodeKey(props.currentFilePath)
  return key ? [key] : []
})

watch(() => props.currentFilePath, (newPath) => {
  if (!newPath) return
  const catKey = getCategoryKeyForPath(newPath)
  if (catKey && !expandedKeys.value.includes(catKey)) {
    expandedKeys.value = [...expandedKeys.value, catKey]
  }
  nextTick(() => {
    const treeEl = treeRef.value?.$el
    if (!treeEl) return
    const selected = treeEl.querySelector('.n-tree-node--selected')
    if (selected) {
      const container = treeEl.closest('.project-explorer') || treeEl.parentElement
      if (container) {
        const sRect = selected.getBoundingClientRect()
        const cRect = container.getBoundingClientRect()
        if (sRect.top < cRect.top + 4 || sRect.bottom > cRect.bottom - 4) {
          selected.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
        }
      }
    }
  })
}, { immediate: true })

function onSelect(keys) {
  const key = keys && keys[0]
  if (!key) return
  const filePrefixes = ['src-', 'win-', 'mod-', 'cls-', 'snd-', 'img-', 'dll-']
  for (const prefix of filePrefixes) {
    if (key.startsWith(prefix)) {
      const path = key.slice(prefix.length)
      emit('open-file', path)
      return
    }
  }
}

function onContextMenuSelect(key) {
  contextMenuShow.value = false
  const node = contextMenuNode.value
  if (!node) return
  switch (key) {
    case 'new-window':
      emit('new-window')
      break
    case 'new-module':
      emit('new-module')
      break
    case 'new-class':
      emit('new-class')
      break
    case 'new-code':
      emit('new-code-file')
      break
    case 'expand-all':
      expandAll()
      emit('expand-all')
      break
    case 'collapse-all':
      collapseAll()
      emit('collapse-all')
      break
    case 'open':
      if (node.path) emit('open-file', node.path)
      break
    case 'delete':
      if (node.path) emit('delete-file', node.path)
      break
    case 'open-elib': {
      // 规约 §6：打开 .elib 的 source.eg
      const root = getCurrentProjectRoot()
      if (root && node.libName) {
        const sep = root.includes('\\') ? '\\' : '/'
        emit('open-file', root + sep + 'libs' + sep + node.libName + sep + 'source.eg')
      }
      break
    }
    case 'rename-elib': {
      const root = getCurrentProjectRoot()
      if (root && node.libName && window.IDEService?.RenameElib) {
        const newName = window.prompt(t('projectTree.inputLibName'), node.libName)
        if (newName && newName !== node.libName) {
          window.IDEService.RenameElib(root, node.libName, newName).then(() => emit('refresh'))
        }
      }
      break
    }
    case 'delete-elib': {
      const root = getCurrentProjectRoot()
      if (root && node.libName && window.IDEService?.DeleteElib) {
        if (window.confirm(t('projectTree.confirmDeleteLib', { name: node.libName }))) {
          window.IDEService.DeleteElib(root, node.libName).then(() => emit('refresh'))
        }
      }
      break
    }
  }
}
</script>

<style scoped>
.project-explorer {
  flex: 1;
  min-height: 0;
  padding: 8px;
  overflow: auto;
}
.explorer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 4px 6px 8px;
}
.tree-label {
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
}
:deep(.node-prefix) {
  display: inline-block;
  width: 20px;
  text-align: center;
  font-size: var(--ide-font-size-sm);
  font-weight: 600;
}
:deep(.node-suffix) {
  margin-left: 6px;
  font-size: var(--ide-font-size-xs);
  color: var(--text-secondary);
  opacity: 0.8;
}

.tree-flow :deep(.n-tree-node-indent--show-line::before) {
  border-left-style: dashed !important;
  border-left-width: 1px !important;
}
.tree-flow :deep(.n-tree-node-indent--is-leaf::after) {
  border-bottom-color: var(--n-line-color, var(--flow-color, var(--border-color))) !important;
  border-bottom-style: dashed !important;
}

.tree-flow.dim :deep(.n-tree-node-content-wrapper),
.tree-flow.dim :deep(.n-tree-node-content-wrapper *) {
  --n-line-color: var(--border-color) !important;
  --flow-color: var(--border-color) !important;
}
</style>
