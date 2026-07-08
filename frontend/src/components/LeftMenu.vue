<template>
  <div class="left-menu">
    <div class="menu-top">
      <button
        v-for="item in topItems"
        :key="item.key"
        class="menu-item"
        :class="{ active: active === item.key }"
        :title="item.label"
        @click="select(item.key)"
      >
        <n-icon :component="item.icon" />
      </button>
      <!-- G7：插件自定义面板图标（icon 为 emoji 字符串） -->
      <button
        v-for="panel in pluginPanels"
        :key="'plugin-' + panel.id"
        class="menu-item"
        :class="{ active: active === 'plugin:' + panel.id }"
        :title="panel.label"
        @click="select('plugin:' + panel.id)"
      >
        <span class="menu-icon-text">{{ panel.icon }}</span>
      </button>
    </div>
    <div class="menu-bottom">
      <button class="menu-item" :title="t('leftmenu.search')" @click="$emit('search')">
        <n-icon :component="SearchOutline" />
      </button>
      <button class="menu-item" :title="t('leftmenu.user')" @click="$emit('user')">
        <n-icon :component="PersonCircleOutline" />
      </button>
      <button class="menu-item" :title="outputCollapsed ? t('leftmenu.expandOutput') : t('leftmenu.collapseOutput')" @click="toggleOutput">
        <n-icon :component="outputCollapsed ? ExpandOutline : ContractOutline" />
      </button>
      <button class="menu-item" :title="t('leftmenu.closeProject')" @click="$emit('close-project')">
        <n-icon :component="CloseCircleOutline" />
      </button>
    </div>
  </div>
</template>

<script setup>
import { NIcon } from 'naive-ui'
import {
  LibraryOutline,
  FolderOutline,
  SparklesOutline,
  DocumentOutline,
  SearchOutline,
  PersonCircleOutline,
  ExpandOutline,
  ContractOutline,
  CloseCircleOutline,
} from '@vicons/ionicons5'
import { t } from '../i18n/index.js'

const props = defineProps({
  active: { type: String, default: 'project' },
  outputCollapsed: { type: Boolean, default: false },
  // G7：插件自定义面板列表，每项 { id, icon(emoji字符串), label, render }
  pluginPanels: { type: Array, default: () => [] }
})

const emit = defineEmits(['select', 'search', 'user', 'toggle-output', 'close-project'])

const topItems = [
  { key: 'project', label: t('menu.project'), icon: FolderOutline },
  { key: 'files', label: t('menu.file'), icon: DocumentOutline },
  { key: 'support', label: t('menu.support'), icon: LibraryOutline },
  { key: 'ai', label: t('menu.ai'), icon: SparklesOutline },
]

function select(key) {
  emit('select', key)
}

function toggleOutput() {
  emit('toggle-output')
}
</script>

<style scoped>
.left-menu {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  width: 48px;
  height: 100%;
  padding: 6px 0;
  background: var(--bg-sidebar);
  backdrop-filter: blur(16px) saturate(1.2);
  -webkit-backdrop-filter: blur(16px) saturate(1.2);
  border-right: 1px solid var(--border-color);
}
.menu-top,
.menu-bottom {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}
.menu-item {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  margin: 0;
  border: none;
  background: transparent;
  box-shadow: none;
  outline: none;
  border-radius: 8px;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 20px;
  transition: background 0.15s, color 0.15s;
}
.menu-item:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}
.menu-item:active {
  color: var(--accent-color);
  background: var(--accent-bg);
}
.menu-item:focus {
  outline: none;
}
.menu-item.active {
  color: var(--accent-color);
  background: var(--accent-bg);
  box-shadow: inset 3px 0 0 var(--accent-color);
}
.menu-icon-text {
  font-size: 18px;
  line-height: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
</style>
