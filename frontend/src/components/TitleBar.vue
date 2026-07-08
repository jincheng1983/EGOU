<template>
  <div class="title-bar" @dblclick="toggleMaximise">
    <div class="title-bar-left">
      <img src="/appicon.png" class="app-logo" alt="logo">
      <span class="app-name">易狗 IDE</span>
      <n-divider vertical style="height: 20px; margin: 0 8px;" />
      <button class="title-btn" :title="t('titlebar.saveHint')" @click="$emit('quick-save')">
        <n-icon :component="SaveOutline" />
      </button>
      <button class="title-btn" :title="t('titlebar.saveAs')" @click="$emit('save', true)">
        <n-icon :component="Save" />
      </button>
      <button class="title-btn" :title="t('titlebar.undo')" @click="$emit('undo')">
        <n-icon :component="ArrowUndoOutline" />
      </button>
      <button class="title-btn" :title="t('titlebar.redo')" @click="$emit('redo')">
        <n-icon :component="ArrowRedoOutline" />
      </button>
    </div>

    <div class="title-bar-center">
      <button class="title-btn run-btn" :title="t('titlebar.run')" @click="$emit('run')">
        <n-icon :component="PlayOutline" />
      </button>
      <n-dropdown
        :options="buildOptions"
        placement="bottom"
        trigger="click"
        @select="onBuildSelect"
      >
        <button class="title-btn build-config-btn" :title="t('titlebar.build')">
          <n-icon :component="BuildOutline" />
        </button>
      </n-dropdown>
      <button class="title-btn" :title="t('titlebar.debug')" @click="$emit('debug')">
        <n-icon :component="BugOutline" />
      </button>
    </div>

    <div class="title-bar-right">
      <button class="title-btn" :title="t('titlebar.about')" @click="$emit('about')">
        <n-icon :component="InformationCircleOutline" />
      </button>
      <n-dropdown
        :options="themeDropdownOptions"
        placement="bottom-end"
        trigger="click"
        @select="onSelectTheme"
      >
        <button class="title-btn" :title="t('titlebar.theme')">
          <n-icon :component="isDark ? SunnyOutline : MoonOutline" />
        </button>
      </n-dropdown>
      <button class="title-btn" :title="t('titlebar.snippets')" @click="$emit('snippets')">
        <n-icon :component="CodeSlashOutline" />
      </button>
      <button class="title-btn" :title="t('titlebar.systemSettings')" @click="$emit('settings')">
        <n-icon :component="SettingsOutline" />
      </button>
      <n-divider vertical style="height: 20px; margin: 0 8px;" />
      <button class="title-btn window-ctrl" @click="minimise">
        <n-icon :component="RemoveOutline" />
      </button>
      <button class="title-btn window-ctrl" @click="toggleMaximise">
        <n-icon :component="isMaximised ? ExpandOutline : SquareOutline" />
      </button>
      <button class="title-btn window-ctrl close" @click="close">
        <n-icon :component="CloseOutline" />
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { NIcon, NDivider, NDropdown } from 'naive-ui'
import { Window } from '@wailsio/runtime'
import { t } from '../i18n/index.js'
import {
  SaveOutline,
  Save,
  ArrowUndoOutline,
  ArrowRedoOutline,
  PlayOutline,
  BuildOutline,
  BugOutline,
  InformationCircleOutline,
  MoonOutline,
  SunnyOutline,
  SettingsOutline,
  CodeSlashOutline,
  RemoveOutline,
  SquareOutline,
  ExpandOutline,
  CloseOutline,
} from '@vicons/ionicons5'

const props = defineProps({
  isDark: { type: Boolean, default: true },
  themeOptions: {
    type: Array,
    default: () => []
  },
  currentTheme: { type: String, default: 'dark' }
})

const emit = defineEmits(['save', 'quick-save', 'undo', 'redo', 'run', 'build', 'debug', 'about', 'select-theme', 'settings', 'snippets'])

const buildOptions = [
  { label: '生成可执行文件 (Windows/Linux/macOS)', key: 'build-all' },
  { type: 'divider', key: 'd1' },
  { label: '编译选项...', key: 'build-options' },
]

function onBuildSelect(key) {
  emit('build', key)
}

const themeDropdownOptions = computed(() =>
  props.themeOptions.map(t => ({
    label: t.label,
    key: t.value,
    type: t.value === props.currentTheme ? 'primary' : 'default'
  }))
)

function onSelectTheme(key) {
  emit('select-theme', key)
}

const isMaximised = ref(false)

async function updateMaxState() {
  try {
    isMaximised.value = await Window.IsMaximised()
  } catch {}
}

async function minimise() {
  try { await Window.Minimise() } catch {}
}

async function toggleMaximise() {
  try {
    await Window.ToggleMaximise()
    await updateMaxState()
  } catch {}
}

async function close() {
  try { await Window.Close() } catch {}
}

onMounted(updateMaxState)
</script>

<style scoped>
.title-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 40px;
  padding: 0 10px;
  background: var(--toolbar-gradient);
  backdrop-filter: blur(12px) saturate(1.2);
  -webkit-backdrop-filter: blur(12px) saturate(1.2);
  border-bottom: 1px solid var(--border-light);
  --wails-draggable: drag;
  -webkit-app-region: drag;
  user-select: none;
}
.title-bar-left,
.title-bar-center,
.title-bar-right {
  display: flex;
  align-items: center;
  gap: 4px;
  --wails-draggable: no-drag;
  -webkit-app-region: no-drag;
}
.app-logo {
  width: 22px;
  height: 22px;
  margin-right: 8px;
  border-radius: 4px;
}
.app-name {
  font-size: var(--ide-font-size-lg);
  font-weight: 600;
  margin-right: 8px;
  color: var(--text-primary);
}
.title-btn {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  margin: 0;
  border: none;
  background: transparent;
  box-shadow: none;
  outline: none;
  border-radius: 6px;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 16px;
  --wails-draggable: no-drag;
  -webkit-app-region: no-drag;
  transition: background 0.15s, color 0.15s;
}
.title-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}
.title-btn:active {
  color: var(--accent-color);
  background: var(--accent-bg);
}
.title-btn:focus {
  outline: none;
}
.title-btn.window-ctrl {
  width: 34px;
  height: 30px;
}
.title-btn.close:hover {
  background: var(--color-error) !important;
  color: white !important;
}
.run-btn {
  color: var(--accent-color);
}
.run-btn:hover {
  color: var(--accent-color);
  background: var(--accent-bg);
}
.build-config-btn {
  color: var(--text-secondary);
}
.build-config-btn:hover {
  color: var(--accent-color);
  background: var(--accent-bg);
}
</style>
