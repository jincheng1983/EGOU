<template>
  <Transition name="bp-fade">
    <div v-if="visible" class="build-progress">
      <div class="bp-steps">
        <div
          v-for="(step, idx) in steps"
          :key="step.key"
          class="bp-step"
          :class="{ active: currentStepIdx >= idx, done: currentStepIdx > idx, current: currentStepIdx === idx }"
        >
          <div class="bp-step-dot">
            <svg v-if="currentStepIdx > idx || (currentStepIdx === idx && percent >= 100)" class="bp-check" viewBox="0 0 16 16" fill="none">
              <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <svg v-else-if="currentStepIdx === idx && percent < 100" class="bp-spinner" viewBox="0 0 16 16">
              <circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="2" fill="none" stroke-dasharray="28 12" stroke-linecap="round"/>
            </svg>
            <span v-else class="bp-step-num">{{ idx + 1 }}</span>
          </div>
          <span class="bp-step-label">{{ step.label }}</span>
          <div v-if="idx < steps.length - 1" class="bp-step-line" :class="{ filled: currentStepIdx > idx }" />
        </div>
      </div>
      <div class="bp-bar-track">
        <div class="bp-bar-fill" :style="{ width: displayPercent + '%' }" />
        <div class="bp-bar-shine" :style="{ left: displayPercent + '%' }" v-if="displayPercent < 100" />
      </div>
      <div class="bp-percent">{{ Math.round(displayPercent) }}%</div>
    </div>
  </Transition>
</template>

<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'
import { t } from '../i18n/index.js'

const props = defineProps({
  active: { type: Boolean, default: false },
  step: { type: String, default: '' },
  percent: { type: Number, default: 0 },
})

const emit = defineEmits(['update:active'])

const visible = ref(false)
const displayPercent = ref(0)
const fakeTimer = ref(null)
const hideTimer = ref(null)

const steps = computed(() => [
  { key: 'prepare', label: t('buildprogress.preparing') },
  { key: 'transpile', label: t('buildprogress.transpiling') },
  { key: 'build', label: t('buildprogress.compiling') },
  { key: 'link', label: t('buildprogress.linking') },
  { key: 'run', label: t('buildprogress.running') },
])

const stepOrder = ['prepare', 'transpile', 'ready', 'build', 'link', 'run', 'done']

const currentStepIdx = computed(() => {
  const idx = stepOrder.indexOf(props.step)
  if (idx === -1) return 0
  if (props.step === 'ready') return 1
  if (props.step === 'done') return steps.value.length - 1
  return Math.min(idx, steps.value.length - 1)
})

function clearFakeTimer() {
  if (fakeTimer.value) {
    clearInterval(fakeTimer.value)
    fakeTimer.value = null
  }
}

function clearHideTimer() {
  if (hideTimer.value) {
    clearTimeout(hideTimer.value)
    hideTimer.value = null
  }
}

function startFakeProgress(targetPct) {
  clearFakeTimer()
  fakeTimer.value = setInterval(() => {
    if (displayPercent.value < targetPct - 2) {
      displayPercent.value += 0.5
    } else if (displayPercent.value < targetPct) {
      displayPercent.value += 0.2
    } else {
      clearFakeTimer()
    }
  }, 150)
}

watch(() => props.active, (val) => {
  clearHideTimer()
  if (val) {
    visible.value = true
    displayPercent.value = 0
    startFakeProgress(5)
  }
}, { immediate: true })

watch(() => props.percent, (val) => {
  if (val >= 100) {
    clearFakeTimer()
    displayPercent.value = 100
  } else if (val > 0) {
    startFakeProgress(val)
  }
})

watch(() => props.step, (newStep) => {
  if (newStep === 'done' && props.percent >= 100) {
    clearFakeTimer()
    displayPercent.value = 100
    clearHideTimer()
    hideTimer.value = setTimeout(() => {
      visible.value = false
      emit('update:active', false)
    }, 1200)
  } else if (newStep === 'build') {
    startFakeProgress(75)
  } else if (newStep === 'link') {
    startFakeProgress(85)
  } else if (newStep === 'run') {
    startFakeProgress(95)
  }
})

onUnmounted(() => {
  clearFakeTimer()
  clearHideTimer()
})

function reset() {
  clearFakeTimer()
  clearHideTimer()
  visible.value = false
  displayPercent.value = 0
  emit('update:active', false)
}

defineExpose({ reset })
</script>

<style scoped>
.build-progress {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 6px 12px;
  background: var(--bg-secondary);
  border-radius: 6px;
  border: 1px solid var(--border-color);
  margin-bottom: 6px;
  min-height: 38px;
  flex-shrink: 0;
}

.bp-steps {
  display: flex;
  align-items: center;
  gap: 0;
  flex-shrink: 0;
}

.bp-step {
  display: flex;
  align-items: center;
  position: relative;
}

.bp-step-dot {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-tertiary);
  border: 2px solid var(--border-color);
  color: var(--text-dim);
  font-size: 11px;
  font-weight: 600;
  transition: all 0.3s ease;
  flex-shrink: 0;
  z-index: 1;
}

.bp-step.active .bp-step-dot,
.bp-step.current .bp-step-dot {
  border-color: var(--accent-color);
  color: var(--accent-color);
  background: var(--accent-bg);
}

.bp-step.done .bp-step-dot {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: #fff;
}

.bp-step-num {
  line-height: 1;
}

.bp-check {
  width: 12px;
  height: 12px;
}

.bp-spinner {
  width: 14px;
  height: 14px;
  animation: bp-spin 1s linear infinite;
}

@keyframes bp-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.bp-step-label {
  margin-left: 8px;
  font-size: var(--ide-font-size, 13px);
  white-space: nowrap;
  color: var(--text-dim);
  transition: color 0.3s ease;
}

.bp-step.active .bp-step-label,
.bp-step.current .bp-step-label {
  color: var(--text-primary);
  font-weight: 500;
}

.bp-step.done .bp-step-label {
  color: var(--text-secondary);
}

.bp-step-line {
  width: 32px;
  height: 2px;
  background: var(--border-color);
  margin: 0 12px;
  border-radius: 1px;
  transition: background 0.4s ease;
  flex-shrink: 0;
}

.bp-step-line.filled {
  background: var(--accent-color);
}

.bp-bar-track {
  flex: 1;
  height: 4px;
  background: var(--bg-tertiary);
  border-radius: 2px;
  overflow: hidden;
  position: relative;
  min-width: 60px;
}

.bp-bar-fill {
  height: 100%;
  background: var(--accent-color);
  border-radius: 2px;
  transition: width 0.3s ease;
  position: relative;
}

.bp-bar-shine {
  position: absolute;
  top: -2px;
  width: 40px;
  height: 8px;
  background: radial-gradient(ellipse at center, var(--accent-bg) 0%, transparent 70%);
  transform: translateX(-50%);
  transition: left 0.3s ease;
  pointer-events: none;
}

.bp-percent {
  font-size: var(--ide-font-size-lg, 14px);
  font-weight: 600;
  color: var(--accent-color);
  min-width: 42px;
  text-align: right;
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
}

.bp-fade-enter-active {
  transition: all 0.25s ease;
}

.bp-fade-leave-active {
  transition: all 0.35s ease;
}

.bp-fade-enter-from {
  opacity: 0;
  transform: scaleY(0.8);
  transform-origin: top;
}

.bp-fade-leave-to {
  opacity: 0;
  transform: scaleY(0.8);
  transform-origin: top;
}
</style>
