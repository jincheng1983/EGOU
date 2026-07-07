<template>
  <div class="start-page">
    <div class="start-content">
      <img src="/appicon.png" class="start-logo" alt="logo">
      <h1 class="start-title">易狗 IDE</h1>
      <p class="start-subtitle">中文 Go 语言集成开发环境</p>

      <div class="start-actions">
        <n-button size="large" type="primary" class="start-btn" @click="$emit('create-project')">
          <template #icon><n-icon :component="AddCircleOutline" /></template>
          创建项目
        </n-button>
        <n-button size="large" class="start-btn" @click="$emit('open-project')">
          <template #icon><n-icon :component="FolderOpenOutline" /></template>
          打开项目
        </n-button>
      </div>

      <div class="recent-section">
        <div class="recent-header">
          <n-icon :component="TimeOutline" />
          <span>最近打开</span>
        </div>
        <n-empty v-if="!recent.length" description="暂无最近项目" size="small" />
        <n-list v-else hoverable clickable>
          <n-list-item v-for="(item, idx) in recent" :key="idx" @click="$emit('open-recent', item)">
            <n-thing :title="item.name" :description="item.path" />
          </n-list-item>
        </n-list>
      </div>
    </div>
  </div>
</template>

<script setup>
import { NButton, NIcon, NEmpty, NList, NListItem, NThing } from 'naive-ui'
import { AddCircleOutline, FolderOpenOutline, TimeOutline } from '@vicons/ionicons5'

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
  font-size: 14px;
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
  max-width: 420px;
  text-align: center;
}
.recent-header {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 8px;
}
.recent-section :deep(.n-list-item) {
  padding: 6px 12px;
}
.recent-section :deep(.n-thing-header__title) {
  font-size: 12px;
  font-weight: 500;
}
.recent-section :deep(.n-thing-main__description) {
  font-size: 11px;
}
</style>
