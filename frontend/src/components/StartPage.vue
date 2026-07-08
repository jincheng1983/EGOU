<template>
  <div class="start-page">
    <div class="start-content">
      <img src="/appicon.png" class="start-logo" alt="logo">
      <h1 class="start-title">{{ t('startpage.title') }}</h1>
      <p class="start-subtitle">{{ t('startpage.subtitle') }}</p>

      <div class="start-actions">
        <n-button size="large" type="primary" class="start-btn" @click="$emit('create-project')">
          <template #icon><n-icon :component="AddCircleOutline" /></template>
          {{ t('startpage.createProject') }}
        </n-button>
        <n-button size="large" class="start-btn" @click="$emit('open-project')">
          <template #icon><n-icon :component="FolderOpenOutline" /></template>
          {{ t('startpage.openProject') }}
        </n-button>
      </div>

      <div class="recent-section">
        <div class="recent-header">
          <n-icon :component="TimeOutline" />
          <span>{{ t('startpage.recent') }}</span>
        </div>
        <n-empty v-if="!recent.length" :description="t('startpage.emptyRecent')" size="small" />
        <div v-else class="recent-list">
          <div
            v-for="(item, idx) in recent"
            :key="idx"
            class="recent-item"
            @click="$emit('open-recent', item)"
          >
            <span class="recent-name">{{ item.name }}</span>
            <span class="recent-path">{{ item.path }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { NButton, NIcon, NEmpty } from 'naive-ui'
import { AddCircleOutline, FolderOpenOutline, TimeOutline } from '@vicons/ionicons5'
import { t } from '../i18n/index.js'

defineProps({
  recent: { type: Array, default: () => [] }
})

defineEmits(['create-project', 'open-project', 'open-recent'])
</script>

<style scoped>
.start-page {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  padding: 40px;
  /* 双光晕磨砂效果 — 参考 NxEGO4 风格 */
  background:
    radial-gradient(circle at 50% 35%, rgba(99, 226, 183, 0.08) 0%, transparent 40%),
    radial-gradient(circle at 50% 65%, rgba(99, 102, 241, 0.06) 0%, transparent 45%),
    radial-gradient(circle at 50% 40%, var(--bg-tertiary) 0%, transparent 50%),
    radial-gradient(circle at 50% 60%, var(--bg-secondary) 0%, var(--bg-primary) 70%);
  backdrop-filter: blur(16px) saturate(1.2);
  -webkit-backdrop-filter: blur(16px) saturate(1.2);
}
.start-logo {
  filter: drop-shadow(0 4px 16px rgba(99, 226, 183, 0.3));
}
.start-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  width: 100%;
  max-width: 520px;
}
.start-logo {
  width: 96px;
  height: 96px;
  border-radius: 20px;
  margin-bottom: 20px;
}
.start-title {
  margin: 0 0 8px;
  font-size: 32px;
  font-weight: 700;
  color: var(--text-primary);
}
.start-subtitle {
  margin: 0 0 36px;
  font-size: var(--ide-font-size-lg);
  color: var(--text-secondary);
}
.start-actions {
  display: flex;
  justify-content: center;
  gap: 16px;
  margin-bottom: 36px;
}
.start-btn {
  min-width: 140px;
  border-radius: 8px;
}
.recent-section {
  width: 100%;
  max-width: 520px;
  text-align: center;
}
.recent-header {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: var(--ide-font-size);
  color: var(--text-secondary);
  margin-bottom: 8px;
}
.recent-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.recent-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: var(--ide-font-size);
  transition: background-color 0.15s ease;
}
.recent-item:hover {
  background: var(--bg-hover);
}
.recent-name {
  font-weight: 600;
  color: var(--text-primary);
  flex-shrink: 0;
}
.recent-path {
  color: var(--text-muted);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
