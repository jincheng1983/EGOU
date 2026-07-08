<template>
  <div class="support-panel">
    <div class="panel-header">
      <span class="panel-title">{{ t('support.title') }}</span>
      <div class="header-actions" v-if="elibCount > 0">
        <n-tag size="tiny" round :bordered="false" type="info">{{ t('support.extPackages', { count: elibCount }) }}</n-tag>
        <n-button size="tiny" quaternary circle @click="openLibsDir" :title="t('support.openLibsDir')">
          <n-icon :component="FolderOpenOutline" />
        </n-button>
        <n-button size="tiny" quaternary circle @click="toggleCreateElib" :title="t('support.newPackage')">
          <n-icon :component="AddCircleOutline" />
        </n-button>
      </div>
    </div>
    <div class="create-elib-bar" v-if="showCreateElib">
      <n-input v-model:value="newElibName" size="small" :placeholder="t('support.packageNamePlaceholder')" @keyup.enter="confirmCreateElib" />
      <n-button size="small" type="primary" @click="confirmCreateElib">{{ t('support.create') }}</n-button>
    </div>
    <div class="search-box">
      <n-input v-model:value="keyword" size="small" :placeholder="t('support.searchPlaceholder')" clearable>
        <template #prefix>
          <n-icon :component="SearchOutline" />
        </template>
      </n-input>
    </div>
    <n-tree
      :data="filteredTree"
      :node-props="nodeProps"
      selectable
      block-line
      show-line
      class="support-tree"
      @update:selected-keys="onSelect"
    />
    <n-dropdown
      placement="bottom-start"
      trigger="manual"
      :x="ctxMenuX"
      :y="ctxMenuY"
      :options="ctxMenuOptions"
      :show="showCtxMenu"
      @select="onCtxMenuSelect"
      @clickoutside="showCtxMenu = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, h } from 'vue'
import { NTree, NInput, NIcon, NButton, NTag, NDropdown, useMessage, useDialog } from 'naive-ui'
import { SearchOutline, FolderOpenOutline, AddCircleOutline } from '@vicons/ionicons5'
import {
  getMergedTree,
  getMergedCommands,
  getProjectLibsSummary,
  getCurrentProjectRoot,
  loadProjectLibs,
  libVersion
} from '../utils/supportCommands.js'
import { FLOW_RAINBOW_DARK as FLOW_RAINBOW } from '../utils/colors.js'
import { t } from '../i18n/index.js'

const emit = defineEmits(['show-help', 'open-file'])

const keyword = ref('')
const selectedKey = ref(null)

// 项目扩展包状态
const showCreateElib = ref(false)
const newElibName = ref('')

const elibCount = computed(() => {
  void libVersion.value
  return getProjectLibsSummary().length
})

function toggleCreateElib() {
  showCreateElib.value = !showCreateElib.value
  if (!showCreateElib.value) newElibName.value = ''
}

async function openLibsDir() {
  const root = getCurrentProjectRoot()
  if (!root) return
  const sep = root.includes('\\') ? '\\' : '/'
  const libsPath = root + sep + 'libs'
  if (typeof window !== 'undefined' && window.IDEService && window.IDEService.OpenInExplorer) {
    const err = await window.IDEService.OpenInExplorer(libsPath)
    if (err) console.warn('[lib] 打开 libs 目录失败:', err)
  }
}

async function confirmCreateElib() {
  const name = newElibName.value.trim()
  if (!name) return
  const root = getCurrentProjectRoot()
  if (!root) return
  if (typeof window !== 'undefined' && window.IDEService && window.IDEService.CreateElib) {
    const pkgDir = await window.IDEService.CreateElib(root, name)
    if (pkgDir) {
      await loadProjectLibs(root)
      showCreateElib.value = false
      newElibName.value = ''
    } else {
      console.warn('[lib] 创建扩展包失败')
    }
  }
}

// ===== 右键菜单（项目扩展包节点）=====
const showCtxMenu = ref(false)
const ctxMenuX = ref(0)
const ctxMenuY = ref(0)
const ctxMenuOptions = ref([])
const ctxTargetPkg = ref(null) // 当前右键的目标 .elib 信息
const message = useMessage()
const dialog = useDialog()

function openCtxMenu(e, pkgName, pkgDir) {
  e.preventDefault()
  ctxTargetPkg.value = { name: pkgName, dir: pkgDir }
  ctxMenuOptions.value = [
    { label: t('support.openSource'), key: 'open-source' },
    { label: t('support.openDir'), key: 'open-dir' },
    { type: 'divider', key: 'd1' },
    { label: t('support.rename'), key: 'rename' },
    { label: t('support.delete'), key: 'delete' }
  ]
  ctxMenuX.value = e.clientX
  ctxMenuY.value = e.clientY
  showCtxMenu.value = true
}

async function onCtxMenuSelect(key) {
  showCtxMenu.value = false
  const target = ctxTargetPkg.value
  if (!target) return
  const root = getCurrentProjectRoot()
  if (key === 'open-source') {
    const sep = target.dir.includes('\\') ? '\\' : '/'
    emit('open-file', target.dir + sep + 'source.eg')
  } else if (key === 'open-dir') {
    if (window.IDEService?.OpenInExplorer) {
      const err = await window.IDEService.OpenInExplorer(target.dir)
      if (err) message?.warning?.(t('support.openFailed', { err }))
    }
  } else if (key === 'rename') {
    // 规约 §6：用 Naive UI dialog 替代 window.prompt
    let newName = target.name
    dialog.create({
      title: t('support.renameTitle'),
      content: () => h(NInput, {
        defaultValue: target.name,
        placeholder: t('support.renamePlaceholder'),
        onUpdateValue: (v) => { newName = v },
        autofocus: true
      }),
      positiveText: '确定',
      negativeText: '取消',
      onPositiveClick: async () => {
        if (!newName || newName === target.name) return
        if (window.IDEService?.RenameElib) {
          const newDir = await window.IDEService.RenameElib(root, target.name, newName)
          if (newDir) {
            await loadProjectLibs(root)
            message?.success?.(t('support.renamed', { name: newName }))
          } else {
            message?.error?.(t('support.renameFailed'))
          }
        }
      }
    })
  } else if (key === 'delete') {
    // 规约 §6：用 Naive UI dialog 替代 window.confirm
    dialog.warning({
      title: t('support.deleteTitle'),
      content: t('support.deleteContent', { name: target.name }),
      positiveText: '删除',
      negativeText: '取消',
      onPositiveClick: async () => {
        if (window.IDEService?.DeleteElib) {
          const err = await window.IDEService.DeleteElib(root, target.name)
          if (err) {
            message?.error?.(t('support.deleteFailed', { err }))
          } else {
            await loadProjectLibs(root)
            message?.success?.(t('support.deleted', { name: target.name }))
          }
        }
      }
    })
    return
  }
}

// 流程线（按层级）彩虹调色板，比项目/文件页略深一档，区分两个 tab

// 递归给每个节点打上 `__level` 字段
function injectLevel(nodes, level = 0) {
  return nodes.map(node => ({
    ...node,
    __level: level,
    children: node.children && node.children.length
      ? injectLevel(node.children, level + 1)
      : node.children
  }))
}

// 用合并视图（内置 + 项目 libs）作为基础；libVersion 变化时自动重算
const treeData = computed(() => {
  // 读 libVersion.value 让 watch 知道依赖
  void libVersion.value
  return injectLevel(getMergedTree())
})

const filteredTree = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return treeData.value
  return treeData.value
    .map(group => {
      const children = group.children.filter(item =>
        item.label.toLowerCase().includes(kw) || item.key.toLowerCase().includes(kw)
      )
      return children.length ? { ...group, children } : null
    })
    .filter(Boolean)
})

// node-props 钩子：给 .n-tree-node-content-wrapper 注入 data-level 与彩虹色 CSS 变量
// 项目扩展包的 lib 根节点（key 以 'project:' 开头）额外加：
//   - title: hover 显示元信息（版本/作者/描述）
//   - ondblclick: 双击在编辑器打开 source.eg
function nodeProps({ option }) {
  const level = option && option.__level != null ? option.__level : 0
  const color = FLOW_RAINBOW[level % FLOW_RAINBOW.length]
  const props = {
    'data-level': String(level),
    style: { '--flow-color': color, '--n-line-color': color }
  }
  if (option && typeof option.key === 'string' && option.key.startsWith('project:')) {
    const meta = option.projectMeta || {}
    const parts = []
    if (meta.version) parts.push('版本 ' + meta.version)
    if (meta.author) parts.push('作者 ' + meta.author)
    if (meta.description) parts.push(meta.description)
    parts.push(t('support.hint'))
    props.title = parts.join(' | ')
    props.ondblclick = () => {
      if (meta.packageDir) {
        const sep = meta.packageDir.includes('\\') ? '\\' : '/'
        emit('open-file', meta.packageDir + sep + 'source.eg')
      }
    }
    // 右键菜单
    if (meta.packageDir) {
      const pkgName = option.key.replace(/^project:/, '')
      props.oncontextmenu = (e) => openCtxMenu(e, pkgName, meta.packageDir)
    }
  } else if (option && option.meta) {
    // 命令节点：hover 显示 summary
    const m = option.meta
    const parts = []
    if (m.callSyntax) parts.push(m.callSyntax)
    if (m.summary) parts.push(m.summary)
    if (parts.length) props.title = parts.join(' | ')
  }
  return props
}

function onSelect(keys) {
  const key = keys[0]
  selectedKey.value = key || null
  if (!key) return
  // 从合并视图拿帮助
  void libVersion.value
  const help = getMergedCommands()[key] || t('support.noDesc')
  emit('show-help', { name: key, help })
}
</script>

<style scoped>
.support-panel {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  font-size: var(--ide-font-size);
  font-weight: 600;
  border-bottom: 1px solid var(--border-color);
}
.panel-title {
  flex-shrink: 0;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.create-elib-bar {
  display: flex;
  gap: 6px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--border-color);
}
.search-box {
  padding: 8px 12px;
}
.help-card {
  margin: 8px 12px;
  padding: 10px;
  border-radius: 8px;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-color);
}
.help-name {
  font-weight: 600;
  font-size: var(--ide-font-size);
  margin-bottom: 4px;
  color: var(--text-primary);
}
.help-desc {
  font-size: var(--ide-font-size-sm);
  color: var(--text-secondary);
  margin-bottom: 8px;
  line-height: 1.4;
}

/* ===== 流程线：与项目/文件统一 ===== */
.support-tree :deep(.n-tree-node-indent--show-line::before) {
  border-left-style: dashed !important;
  border-left-width: 1px !important;
}
.support-tree :deep(.n-tree-node-indent--is-leaf::after) {
  border-bottom-color: var(--n-line-color, var(--flow-color, var(--border-color))) !important;
  border-bottom-style: dashed !important;
}
</style>
