<template>
  <div class="file-explorer">
    <div class="explorer-header">
      <n-text depth="3" class="tree-label">{{ projectName || t('fileTree.projectDir') }}</n-text>
      <n-button size="tiny" quaternary :title="t('common.refresh')" @click="$emit('refresh')">
        <n-icon :component="RefreshOutline" />
      </n-button>
    </div>
    <n-tree
      v-if="treeData.length"
      class="tree-flow"
      :class="{ dim: dimLines, rainbow: !dimLines }"
      :data="treeData"
      :default-expand-all="false"
      :selected-keys="selectedKeys"
      :node-props="nodeProps"
      selectable
      block-line
      show-line
      expand-on-click
      style="margin-top: 4px;"
      @update:selected-keys="onSelect"
    />
    <n-empty v-else :description="t('fileTree.empty')" size="small" style="margin-top: 24px;" />
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
import { ref, computed } from 'vue'
import { NTree, NText, NButton, NIcon, NEmpty, NDropdown } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { FLOW_RAINBOW as RAINBOW } from '../utils/colors.js'
import { t } from '../i18n/index.js'

const props = defineProps({
  files: {
    type: Array,
    default: () => []
  },
  projectName: {
    type: String,
    default: ''
  },
  dimLines: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['open-file', 'refresh', 'delete-file'])

const contextMenuShow = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuNode = ref(null)
const contextMenuOptions = computed(() => [
  { label: t('common.open'), key: 'open' },
  { label: t('common.delete'), key: 'delete' }
])

function iconForFile(name) {
  const ext = (name.split('.').pop() || '').toLowerCase()
  if (ext === 'ew') return '🪟'
  if (ext === 'eg') return '📄'
  if (['json', 'toml', 'yaml', 'yml'].includes(ext)) return '⚙️'
  if (['png', 'jpg', 'jpeg', 'bmp', 'gif', 'ico', 'svg'].includes(ext)) return '🖼️'
  if (['wav', 'mp3', 'ogg', 'flac'].includes(ext)) return '🔊'
  if (['ttf', 'otf', 'woff'].includes(ext)) return '🔤'
  return '📄'
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

function buildTree(nodes) {
  return injectLevel(nodes || []).map(node => {
    if (node.isDir) {
      return {
        label: node.name,
        key: `folder-${node.path}`,
        isDir: true,
        path: node.path,
        isLeaf: false,
        prefix: () => '📁',
        children: buildTree(node.children)
      }
    }
    return {
      label: node.name,
      key: `file-${node.path}`,
      isDir: false,
      path: node.path,
      isLeaf: true,
      prefix: () => iconForFile(node.name)
    }
  })
}

const treeData = computed(() => buildTree(props.files))

function nodeProps({ option }) {
  const level = option && option.__level != null ? option.__level : 0
  const color = RAINBOW[level % RAINBOW.length]
  const props = {
    'data-level': String(level),
    style: { '--flow-color': color, '--n-line-color': color }
  }
  if (!option.isDir) {
    props.onContextmenu = (e) => {
      e.preventDefault()
      contextMenuNode.value = option
      contextMenuX.value = e.clientX
      contextMenuY.value = e.clientY
      contextMenuShow.value = true
    }
  }
  return props
}

const selectedKeys = computed(() => [])

function onSelect(keys) {
  const key = keys && keys[0]
  if (!key) return
  if (key.startsWith('file-')) {
    const path = key.replace(/^file-/, '')
    emit('open-file', path)
  }
}

function onContextMenuSelect(key) {
  contextMenuShow.value = false
  const node = contextMenuNode.value
  if (!node) return
  const path = node.path
  if (key === 'open') {
    emit('open-file', path)
  } else if (key === 'delete') {
    emit('delete-file', path)
  }
}
</script>

<style scoped>
.file-explorer {
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
