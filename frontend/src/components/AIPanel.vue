<template>
  <div class="ai-panel">
    <div class="panel-header">
      <n-select
        v-model:value="currentAgentId"
        size="tiny"
        :options="agentOptions"
        :render-label="renderAgentLabel"
        class="ai-agent-select"
      />
      <n-button text size="tiny" class="ai-header-btn" :title="t('ai.history')" @click="toggleHistory">
        <n-icon :component="TimeOutline" />
      </n-button>
      <n-button text size="tiny" class="ai-header-btn" :title="t('ai.newChat')" @click="newChat">
        <n-icon :component="AddCircleOutline" />
      </n-button>
    </div>
    <div v-if="currentAgentCapabilities.length > 0" class="ai-capabilities">
      <span
        v-for="cap in currentAgentCapabilities"
        :key="cap"
        class="ai-cap-badge"
        :class="'cap-' + cap"
      >{{ CAPABILITY_LABELS[cap] || cap }}</span>
    </div>
    <div v-if="historyVisible" class="ai-history-panel">
      <div class="ai-history-header">
        <span class="ai-history-title">{{ t('ai.history') }}</span>
        <n-button text size="tiny" @click="historyVisible = false">✕</n-button>
      </div>
      <div class="ai-history-list">
        <div
          v-for="(h, hidx) in chatHistory"
          :key="hidx"
          class="ai-history-item"
          @click="loadHistory(hidx)"
        >
          <span class="ai-history-preview">{{ h.preview || t('ai.newChatLabel') }}</span>
          <n-button text size="tiny" type="error" @click.stop="deleteHistory(hidx)">✕</n-button>
        </div>
        <div v-if="chatHistory.length === 0" class="ai-history-empty">{{ t('ai.emptyHistory') }}</div>
      </div>
    </div>
    <div ref="chatRef" class="ai-chat">
      <div v-if="!aiAccepted" class="ai-empty">
        <div class="ai-permission">
          <div class="ai-perm-title">{{ t('ai.permission') }}</div>
          <div class="ai-perm-body">
            <p>{{ t('ai.permissionBody') }}</p>
            <ul>
              <li>{{ t('ai.permissionItem1') }}</li>
              <li>{{ t('ai.permissionItem2') }}</li>
              <li>{{ t('ai.permissionItem3') }}</li>
              <li>{{ t('ai.permissionItem4') }}</li>
            </ul>
          </div>
          <div class="ai-perm-actions">
            <n-button type="primary" size="small" @click="acceptAI">{{ t('ai.accept') }}</n-button>
            <n-button text size="small" @click="goToSettings">{{ t('ai.goToSettings') }}</n-button>
          </div>
        </div>
      </div>
      <template v-else>
        <div v-if="messages.length === 0 && loading" class="ai-thinking">
          <div class="ai-thinking-dots">
            <span></span><span></span><span></span>
          </div>
          <span class="ai-thinking-text">{{ t('ai.thinking') }}</span>
        </div>
        <div v-else-if="messages.length === 0" class="ai-empty">
          <n-empty :description="t('ai.inputPlaceholder')" size="small" />
        </div>
        <div v-else class="ai-messages">
          <div
            v-for="(msg, idx) in messages"
            :key="idx"
            class="ai-message"
            :class="{ user: msg.role === 'user', assistant: msg.role === 'assistant' }"
          >
            <div class="ai-bubble">
              <pre class="ai-text">{{ msg.content }}</pre>
            </div>
          </div>
          <div v-if="loading && !activeAssistantMsg" class="ai-message assistant">
            <div class="ai-bubble ai-thinking-bubble">
              <div class="ai-thinking-dots">
                <span></span><span></span><span></span>
              </div>
              <span class="ai-thinking-text">{{ t('ai.thinking') }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>
    <!-- P2-11 危险工具人机确认对话框 -->
    <div v-if="pendingToolCall" class="ai-tool-confirm">
      <div class="ai-tool-confirm-header">
        <span class="ai-tool-confirm-icon">⚠️</span>
        <span class="ai-tool-confirm-title">工具调用确认</span>
        <span class="ai-tool-risk" :class="'risk-' + pendingToolCall.risk">{{ riskLabel(pendingToolCall.risk) }}</span>
      </div>
      <div class="ai-tool-confirm-body">
        <div class="ai-tool-name">工具：{{ pendingToolCall.tool }}</div>
        <div class="ai-tool-summary">{{ pendingToolCall.summary }}</div>
        <div v-if="pendingToolCall.params && Object.keys(pendingToolCall.params).length > 0" class="ai-tool-params">
          <div v-for="(v, k) in pendingToolCall.params" :key="k" class="ai-tool-param">
            <span class="param-key">{{ k }}：</span><span class="param-val">{{ v }}</span>
          </div>
        </div>
      </div>
      <div class="ai-tool-confirm-actions">
        <n-button size="small" type="error" @click="rejectToolCall">拒绝</n-button>
        <n-button size="small" type="primary" @click="approveToolCall">同意执行</n-button>
      </div>
    </div>
    <div class="ai-input-area" v-if="aiAccepted">
      <div class="ai-input-box">
        <n-input
          v-model:value="input"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 6 }"
          :placeholder="t('ai.inputPlaceholder')"
          :disabled="loading"
          class="ai-textarea"
          @keydown="onKeyDown"
        />
      </div>
      <div class="ai-input-tools">
        <n-select
          v-model:value="activeModelIdLocal"
          size="tiny"
          :options="modelOptions"
          class="ai-model-select-bottom"
          :placeholder="t('settings.defaultModel')"
        />
        <n-button text class="ai-tool-btn" :title="t('ai.send') + ' (@)'" @click="insertFileContext">
          @
        </n-button>
        <n-button v-if="supportsFiles" text class="ai-tool-btn" :title="t('common.attach')">
          <n-icon :component="AttachOutline" />
        </n-button>
        <n-button v-if="supportsVision" text class="ai-tool-btn" :title="t('common.image')">
          <n-icon :component="ImageOutline" />
        </n-button>
        <div class="ai-tools-spacer"></div>
        <n-button v-show="loading" text class="ai-stop-btn" :title="t('ai.stop')" @click="stopGenerate">
          <n-icon :component="StopOutline" />
        </n-button>
        <n-button v-show="!loading" class="ai-send-btn" :title="t('ai.send') + ' (Enter)'" :disabled="!input.trim()" @click="send">
          <n-icon :component="SendOutline" />
        </n-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, watch, h, onMounted, onUnmounted } from 'vue'
import { NEmpty, NInput, NButton, NIcon, NSelect, useMessage } from 'naive-ui'
import { AttachOutline, ImageOutline, TimeOutline, AddCircleOutline, StopOutline, SendOutline } from '@vicons/ionicons5'
import { IDEService } from '../../bindings/egou/internal/app'
import { Events } from '@wailsio/runtime'
import { BUILTIN_AGENTS, autoSelectAgent, DEFAULT_AGENT_ID, CAPABILITY_LABELS } from '../lib/aiAgents.js'
import { BUILTIN_SKILLS, matchSkill, formatSkillResult, EG_SYNTAX_REFERENCE, injectSkillPrompt } from '../lib/aiSkills.js'
import { t } from '../i18n/index.js'

const message = useMessage()

const props = defineProps({
  agents: { type: Array, default: () => [] },
  currentAgentId: { type: String, default: DEFAULT_AGENT_ID },
  aiConfig: { type: Object, default: () => ({ endpoint: '', apiKey: '', model: '', temperature: 0.7, maxTokens: 4096, stream: true, compressThreshold: 6000, keepRecent: 8, supportsVision: false, supportsFiles: false, modelName: '未配置' }) },
  models: { type: Array, default: () => [] },
  activeModelId: { type: String, default: '' },
  getCurrentFile: { type: Function, default: null },
  autoPickAgent: { type: Boolean, default: true },
  projectPath: { type: String, default: '' }
})
const emit = defineEmits(['open-settings', 'update:currentAgentId', 'switch-model'])

const allAgents = computed(() => {
  const custom = props.agents || []
  return [...BUILTIN_AGENTS, ...custom.filter(a => !BUILTIN_AGENTS.some(d => d.id === a.id))]
})

const agentOptions = computed(() => allAgents.value.map(a => ({
  label: a.name,
  value: a.id,
  emoji: a.emoji || '',
  desc: a.desc || '',
  capabilities: a.capabilities || []
})))

const availableSkills = computed(() => BUILTIN_SKILLS)

const currentAgentId = ref(props.currentAgentId || DEFAULT_AGENT_ID)
watch(() => props.currentAgentId, (v) => { if (v) currentAgentId.value = v })
watch(currentAgentId, (v) => emit('update:currentAgentId', v))
watch(allAgents, () => {
  if (!allAgents.value.find(a => a.id === currentAgentId.value)) {
    currentAgentId.value = allAgents.value[0]?.id || DEFAULT_AGENT_ID
  }
}, { immediate: true })

const modelOptions = computed(() => {
  return props.models.map(m => ({
    label: m.name || m.model,
    value: m.id,
    caps: {
      vision: m.supportsVision,
      files: m.supportsFiles
    }
  }))
})

function renderModelLabel(option) {
  const caps = option.caps || {}
  return h('div', { style: 'display:flex;align-items:center;gap:4px;overflow:hidden;' }, [
    h('span', { style: 'overflow:hidden;text-overflow:ellipsis;white-space:nowrap;' }, option.label),
    caps.vision ? h('span', { style: 'font-size:10px;margin-left:2px;' }, '🖼️') : null,
    caps.files ? h('span', { style: 'font-size:10px;' }, '📎') : null,
  ])
}

const activeModelIdLocal = ref(props.activeModelId || modelOptions.value[0]?.value)
watch(() => props.activeModelId, (v) => { if (v) activeModelIdLocal.value = v })
watch(activeModelIdLocal, (v) => { if (v !== props.activeModelId) emit('switch-model', v) })
watch(modelOptions, () => {
  if (!modelOptions.value.find(m => m.value === activeModelIdLocal.value)) {
    activeModelIdLocal.value = modelOptions.value[0]?.value
  }
}, { immediate: true })

const currentModelOption = computed(() => modelOptions.value.find(m => m.value === activeModelIdLocal.value))
const supportsVision = computed(() => currentModelOption.value?.caps?.vision || false)
const supportsFiles = computed(() => currentModelOption.value?.caps?.files || false)

const currentChatId = ref('chat_' + Date.now())
const historyVisible = ref(false)

function loadChatHistory() {
  try {
    const saved = localStorage.getItem('eg-ai-chat-history')
    return saved ? JSON.parse(saved) : []
  } catch (e) { return [] }
}

const chatHistory = ref(loadChatHistory())

function saveHistory() {
  localStorage.setItem('eg-ai-chat-history', JSON.stringify(chatHistory.value.slice(0, 50)))
}

function toggleHistory() {
  historyVisible.value = !historyVisible.value
}

function getChatPreview() {
  const firstUser = messages.value.find(m => m.role === 'user')
  return firstUser ? firstUser.content.slice(0, 50) : ''
}

function saveCurrentToHistory() {
  if (messages.value.length === 0) return
  const preview = getChatPreview()
  if (!preview) return
  const existing = chatHistory.value.findIndex(h => h.id === currentChatId.value)
  const entry = {
    id: currentChatId.value,
    preview,
    agentId: currentAgentId.value,
    modelId: activeModelIdLocal.value,
    messages: JSON.parse(JSON.stringify(messages.value)),
    time: Date.now()
  }
  if (existing >= 0) {
    chatHistory.value[existing] = entry
  } else {
    chatHistory.value.unshift(entry)
  }
  saveHistory()
}

function newChat() {
  if (messages.value.length > 0 && getChatPreview()) {
    saveCurrentToHistory()
  }
  currentChatId.value = 'chat_' + Date.now()
  messages.value = []
  activeAssistantMsg.value = null
  historyVisible.value = false
}

function loadHistory(idx) {
  if (messages.value.length > 0 && getChatPreview()) {
    saveCurrentToHistory()
  }
  const h = chatHistory.value[idx]
  if (!h) return
  currentChatId.value = h.id
  messages.value = JSON.parse(JSON.stringify(h.messages || []))
  if (h.agentId) currentAgentId.value = h.agentId
  if (h.modelId) activeModelIdLocal.value = h.modelId
  activeAssistantMsg.value = null
  historyVisible.value = false
}

function deleteHistory(idx) {
  chatHistory.value.splice(idx, 1)
  saveHistory()
}

const currentAgent = computed(() => allAgents.value.find(a => a.id === currentAgentId.value) || allAgents.value[0])
const currentAgentCapabilities = computed(() => currentAgent.value?.capabilities || [])

function renderAgentLabel(option) {
  // 只显示智能体名称（含 emoji），避免窄容器换行错位和 ResizeObserver loop
  return h('div', { style: 'display:flex;align-items:center;gap:4px;overflow:hidden;white-space:nowrap;' }, [
    option.emoji ? h('span', { style: 'flex-shrink:0;' }, option.emoji) : null,
    h('span', { style: 'overflow:hidden;text-overflow:ellipsis;white-space:nowrap;' }, option.label)
  ])
}

const messages = ref([])
const input = ref('')
const loading = ref(false)
const chatRef = ref(null)
const aiAccepted = ref(localStorage.getItem('eg-ai-accepted') === '1')
const activeAssistantMsg = ref(null)

const memoryVisible = ref(false)
const projectMemory = ref('')
let memorySaveTimer = null

// 从后端加载项目级记忆（项目切换时自动加载）
async function loadProjectMemory() {
  if (!props.projectPath) {
    projectMemory.value = ''
    return
  }
  try {
    const content = await IDEService.ReadProjectMemory(props.projectPath)
    projectMemory.value = content || ''
  } catch (e) {
    console.warn('[ai] 加载项目记忆失败:', e)
    projectMemory.value = ''
  }
}

// 防抖保存项目记忆（编辑后 800ms 自动保存）
function saveProjectMemoryDebounced(content) {
  if (memorySaveTimer) clearTimeout(memorySaveTimer)
  memorySaveTimer = setTimeout(async () => {
    if (!props.projectPath) return
    try {
      const err = await IDEService.SaveProjectMemory(props.projectPath, content)
      if (err) console.warn('[ai] 保存项目记忆失败:', err)
    } catch (e) {
      console.warn('[ai] 保存项目记忆异常:', e)
    }
  }, 800)
}

watch(() => props.projectPath, () => { loadProjectMemory() }, { immediate: true })
watch(projectMemory, (v) => { saveProjectMemoryDebounced(v || '') })

let ignoreChunks = false

function clearChat() {
  messages.value = []
  activeAssistantMsg.value = null
}

function stopGenerate() {
  ignoreChunks = true
  loading.value = false
  activeAssistantMsg.value = null
}

function insertFileContext() {
  if (!props.getCurrentFile) return
  const file = props.getCurrentFile()
  if (!file || !file.content) {
    messages.value.push({ role: 'assistant', content: '⚠️ 当前没有打开的文件。' })
    return
  }
  const snippet = file.content.length > 8000 ? file.content.slice(0, 8000) + '\n...（文件过长，已截断前8000字符）' : file.content
  const block = `\n\n【当前文件：${file.name}】\n\`\`\`egou\n${snippet}\n\`\`\`\n`
  input.value = input.value + block
}

let aiChunkUnsub = null
let toolConfirmUnsub = null

// P2-11 危险工具人机确认状态
const pendingToolCall = ref(null) // { id, tool, summary, params, risk }

onMounted(() => {
  aiChunkUnsub = Events.On('ide:ai-chunk', (ev) => {
    const data = ev?.data || ev
    if (!data) return
    if (ignoreChunks) {
      if (data.done) ignoreChunks = false
      return
    }
    if (data.error) {
      if (activeAssistantMsg.value) {
        activeAssistantMsg.value.content += '\n\n❌ 错误: ' + data.error
      } else {
        messages.value.push({ role: 'assistant', content: '❌ 错误: ' + data.error })
      }
      loading.value = false
      activeAssistantMsg.value = null
      return
    }
    if (data.done) {
      loading.value = false
      activeAssistantMsg.value = null
      return
    }
    if (data.content) {
      if (activeAssistantMsg.value) {
        activeAssistantMsg.value.content += data.content
      } else {
        const newMsg = { role: 'assistant', content: data.content }
        messages.value.push(newMsg)
        activeAssistantMsg.value = newMsg
      }
    }
  })

  // P2-11 订阅危险工具确认事件
  toolConfirmUnsub = Events.On('ai-tool-confirm', (ev) => {
    const data = ev?.data || ev
    if (!data || !data.id) return
    pendingToolCall.value = data
  })
})
onUnmounted(() => {
  if (aiChunkUnsub) {
    if (typeof aiChunkUnsub === 'function') aiChunkUnsub()
  }
  if (toolConfirmUnsub) {
    if (typeof toolConfirmUnsub === 'function') toolConfirmUnsub()
  }
})

function acceptAI() {
  aiAccepted.value = true
  localStorage.setItem('eg-ai-accepted', '1')
}

// P2-11 确认/拒绝工具调用
async function approveToolCall() {
  const tc = pendingToolCall.value
  if (!tc) return
  try {
    await IDEService.ConfirmToolCall(tc.id, true)
  } catch (e) {}
  pendingToolCall.value = null
}

async function rejectToolCall() {
  const tc = pendingToolCall.value
  if (!tc) return
  try {
    await IDEService.ConfirmToolCall(tc.id, false)
  } catch (e) {}
  pendingToolCall.value = null
}

function riskLabel(risk) {
  const map = { safe: '安全', moderate: '中等', dangerous: '危险' }
  return map[risk] || risk
}

function goToSettings() {
  emit('open-settings')
}

function scrollToBottom() {
  nextTick(() => {
    const el = chatRef.value
    if (el) {
      el.scrollTop = el.scrollHeight
    }
  })
}

watch(messages, scrollToBottom, { deep: true })

function onKeyDown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

async function executeSkill(skillId, params = {}) {
  try {
    switch (skillId) {
      case 'get_current_file':
        if (props.getCurrentFile) {
          const file = await props.getCurrentFile()
          if (file && file.content) {
            return formatSkillResult('获取当前文件',
              `文件路径: ${file.path || '未保存'}\n文件内容:\n\`\`\`eg\n${file.content}\n\`\`\``)
          }
          return formatSkillResult('获取当前文件', '当前没有打开文件')
        }
        return formatSkillResult('获取当前文件', '无法获取当前文件')
      case 'get_project_structure':
        if (!props.projectPath) {
          return formatSkillResult('获取项目结构', '未打开项目')
        }
        try {
          const tree = await IDEService.ListProjectDir(props.projectPath)
          if (!Array.isArray(tree) || tree.length === 0) {
            return formatSkillResult('获取项目结构', '项目目录为空')
          }
          const lines = []
          const walk = (nodes, depth) => {
            if (!Array.isArray(nodes)) return
            for (const n of nodes) {
              const indent = '  '.repeat(depth)
              const icon = n.IsDir ? '📁' : '📄'
              lines.push(`${indent}${icon} ${n.Name}`)
              // 限制深度 3 层，避免大项目输出过长
              if (n.IsDir && depth < 3 && Array.isArray(n.Children) && n.Children.length > 0) {
                walk(n.Children, depth + 1)
              }
            }
          }
          walk(tree, 0)
          let out = lines.join('\n')
          // 截断保护，避免占满 AI 上下文
          if (out.length > 3000) out = out.slice(0, 3000) + '\n... (已截断，项目较大)'
          return formatSkillResult('获取项目结构', `项目路径: ${props.projectPath}\n${out}`)
        } catch (e) {
          return formatSkillResult('获取项目结构', '获取失败: ' + (e.message || String(e)))
        }
      case 'get_support_libs':
        return formatSkillResult('获取支持库',
          'EGOU 核心内置命令:\n' +
          '【输出】调试输出(文本)、信息框(标题,文本)\n' +
          '【转换】到文本(任意)、到整数(文本)、到小数(文本)\n' +
          '【文本】取文本长度(文本)、取文本左边(文本,长度)、取文本右边(文本,长度)、取文本中间(文本,起始,长度)、寻找文本(文本,被找)、子文本替换(文本,被替,替为)\n' +
          '【算术】四舍五入(小数,位数)、取绝对值(数)、取整(数)、求次方(数,次方)、开平方(数)\n' +
          '【时间】取现行时间()、时间到文本(时间)、取年份(时间)、取月份(时间)、取日(时间)\n' +
          '【文件】读入文本(路径)、写出文本(路径,内容)、文件是否存在(路径)、删除文件(路径)')
      case 'explain_syntax':
        const kw = params.keyword || ''
        const syntax = EG_SYNTAX_REFERENCE[kw]
        if (syntax) {
          return formatSkillResult(`语法解释: ${kw}`,
            `语法:\n${syntax.syntax}\n\n示例:\n${syntax.example}\n\n说明: ${syntax.note || '无'}`)
        }
        const keys = Object.keys(EG_SYNTAX_REFERENCE).join('、')
        return formatSkillResult('语法解释', `可查询的语法关键词: ${keys}\n\n请指定具体关键词，如"如果"、"循环"等。`)
      default:
        return formatSkillResult(skillId, '技能暂未实现')
    }
  } catch (e) {
    return formatSkillResult(skillId, '执行出错: ' + (e.message || String(e)))
  }
}

async function send() {
  const text = input.value.trim()
  if (!text || loading.value) return

  ignoreChunks = false
  messages.value.push({ role: 'user', content: text })
  input.value = ''
  loading.value = true
  activeAssistantMsg.value = null

  const currentModel = props.models.find(m => m.id === activeModelIdLocal.value) || props.models[0]
  if (!currentModel || !currentModel.endpoint || !currentModel.apiKey || !currentModel.model) {
    loading.value = false
    messages.value.push({
      role: 'assistant',
      content: '⚠️ AI 模型尚未配置。\n\n请在「系统设置 → AI → 模型管理」中添加并配置模型。'
    })
    return
  }
  const cfg = currentModel

  // 自动选择智能体
  let agentId = currentAgentId.value
  if (props.autoPickAgent && messages.value.length <= 2) {
    const picked = autoSelectAgent(text, allAgents.value)
    if (picked && picked !== agentId) {
      agentId = picked
      currentAgentId.value = picked
      const pickedAgent = allAgents.value.find(a => a.id === picked)
      if (pickedAgent) {
        messages.value.push({
          role: 'assistant',
          content: `💡 已自动切换到【${pickedAgent.emoji || ''} ${pickedAgent.name}】`
        })
      }
    }
  }
  const agent = allAgents.value.find(a => a.id === agentId) || allAgents.value[0]

  // 自动收集技能上下文
  let skillContext = ''
  const matchedSkillId = matchSkill(text, availableSkills.value)
  if (matchedSkillId) {
    const skill = availableSkills.value.find(s => s.id === matchedSkillId)
    if (skill) {
      messages.value.push({
        role: 'assistant',
        content: `🔧 正在使用技能【${skill.icon || ''} ${skill.name}】...`
      })
      skillContext = await executeSkill(matchedSkillId, text.includes('当前文件') ? {} : { keyword: text.match(/["']?([^"']+)["']?/)?.[1] || text })
    }
  }

  // 自动附带当前文件上下文（如果用户提到代码相关问题）
  if (props.getCurrentFile && /代码|写|函数|方法|报错|错误|bug|修复|帮我看|看一下|这段/.test(text) && !matchedSkillId) {
    try {
      const file = await props.getCurrentFile()
      if (file && file.content && file.content.length < 8000) {
        skillContext += '\n\n' + formatSkillResult('当前文件上下文',
          `当前打开文件: ${file.path || '未保存'}\n\n\`\`\`eg\n${file.content}\n\`\`\`\n\n用户问题与当前文件相关，请基于以上代码回答。`)
      }
    } catch (e) {}
  }

  const rawMessages = messages.value
    .filter(m => !m._memoryInjected && m.content && !m.content.startsWith('⚠️') && !m.content.startsWith('💡') && !m.content.startsWith('❌') && !m.content.startsWith('🔧'))
    .map(m => ({ role: m.role, content: m.content }))

  let apiMessages = rawMessages
  let compressNotice = ''
  const threshold = (cfg.compressThreshold || 6000)
  const keepRecent = cfg.keepRecent || 8
  const totalChars = rawMessages.reduce((sum, m) => sum + m.content.length, 0)

  if (totalChars > threshold && rawMessages.length > keepRecent + 1) {
    const removed = rawMessages.length - keepRecent
    const recentMsgs = rawMessages.slice(-keepRecent)
    const olderMsgs = rawMessages.slice(0, removed)
    // v0.11.0：本地保留简短摘要 + 异步调用后端 CompactConversation 持久化完整摘要到 .eg/memory/summary.md
    const olderSummary = olderMsgs
      .map(m => (m.role === 'user' ? '用户' : '助手') + '：' + m.content.slice(0, 80) + (m.content.length > 80 ? '...' : ''))
      .join('\n')
    compressNotice = `以下是之前对话的摘要（共 ${removed} 条消息因长度限制被压缩）：\n${olderSummary}\n\n（以下是最近的对话）`
    apiMessages = [{ role: 'system', content: compressNotice }, ...recentMsgs]

    // 异步触发后端 AI 压缩（不阻塞当前对话，失败静默忽略，下次启动会从 summary.md 读取）
    if (props.projectPath && cfg.endpoint && cfg.apiKey && cfg.model) {
      try {
        IDEService.CompactConversation(
          cfg.endpoint,
          cfg.apiKey,
          cfg.model,
          olderMsgs.map(m => ({ role: m.role, content: m.content })),
          props.projectPath
        )
      } catch (e) {
        console.warn('[ai] 触发会话压缩失败:', e)
      }
    }
  }

  // P2-10 按需注入技能 prompt（trigger 匹配时才注入，不占上下文 token）
  apiMessages = injectSkillPrompt(text, apiMessages)

  // 合并系统提示词（智能体prompt + 技能上下文 + 项目记忆）
  let systemPrompt = (agent && agent.prompt) || ''
  if (skillContext) {
    systemPrompt += '\n\n【技能/上下文信息】\n' + skillContext
  }

  try {
    await IDEService.AIChat(
      cfg.endpoint,
      cfg.apiKey,
      cfg.model,
      apiMessages,
      cfg.temperature || 0.7,
      cfg.maxTokens || 4096,
      systemPrompt,
      projectMemory.value || '',
      props.projectPath || ''
    )
  } catch (e) {
    loading.value = false
    activeAssistantMsg.value = null
    messages.value.push({
      role: 'assistant',
      content: '❌ 调用失败: ' + (e.message || String(e))
    })
  }
}

defineExpose({
  getProjectMemory: () => projectMemory.value,
  getCurrentAgent: () => currentAgent.value
})
</script>

<style scoped>
.ai-thinking {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 100%;
  min-height: 200px;
  color: var(--text-dim);
  font-size: 12px;
}

.ai-thinking-bubble {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-thinking-dots {
  display: inline-flex;
  gap: 3px;
}

.ai-thinking-dots span {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--accent-color);
  animation: ai-thinking-bounce 1.4s infinite ease-in-out both;
}

.ai-thinking-dots span:nth-child(1) {
  animation-delay: -0.32s;
}

.ai-thinking-dots span:nth-child(2) {
  animation-delay: -0.16s;
}

@keyframes ai-thinking-bounce {
  0%, 80%, 100% {
    transform: scale(0.6);
    opacity: 0.4;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
}

.ai-thinking-text {
  color: var(--text-dim);
  font-size: 11px;
}

.ai-panel {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  height: 100%;
}

.panel-header {
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.ai-agent-select {
  flex: 1;
  min-width: 0;
}

.ai-header-btn {
  flex-shrink: 0;
  padding: 0 6px !important;
  font-size: 14px;
}

.ai-capabilities {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 4px 12px 6px;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-secondary);
}

.ai-cap-badge {
  display: inline-block;
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
  line-height: 1.5;
  background: var(--bg-hover);
  color: var(--text-dim);
  border: 1px solid var(--border-color);
}

.ai-cap-badge.cap-chat       { background: color-mix(in srgb, var(--cap-chat) 18%, transparent);      color: var(--cap-chat);      border-color: color-mix(in srgb, var(--cap-chat) 35%, transparent); }
.ai-cap-badge.cap-code_gen   { background: color-mix(in srgb, var(--cap-code_gen) 18%, transparent);  color: var(--cap-code_gen);  border-color: color-mix(in srgb, var(--cap-code_gen) 35%, transparent); }
.ai-cap-badge.cap-debug      { background: color-mix(in srgb, var(--cap-debug) 18%, transparent);     color: var(--cap-debug);     border-color: color-mix(in srgb, var(--cap-debug) 35%, transparent); }
.ai-cap-badge.cap-explain    { background: color-mix(in srgb, var(--cap-explain) 18%, transparent);   color: var(--cap-explain);   border-color: color-mix(in srgb, var(--cap-explain) 35%, transparent); }
.ai-cap-badge.cap-plan       { background: color-mix(in srgb, var(--cap-plan) 18%, transparent);      color: var(--cap-plan);      border-color: color-mix(in srgb, var(--cap-plan) 35%, transparent); }
.ai-cap-badge.cap-ui_design  { background: color-mix(in srgb, var(--cap-ui_design) 18%, transparent); color: var(--cap-ui_design); border-color: color-mix(in srgb, var(--cap-ui_design) 35%, transparent); }

.ai-history-panel {
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-secondary);
  max-height: 200px;
  overflow-y: auto;
  flex-shrink: 0;
}

.ai-history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border-color);
}

.ai-history-list {
  padding: 4px 0;
}

.ai-history-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s;
  gap: 8px;
}

.ai-history-item:hover {
  background: var(--bg-tertiary);
}

.ai-history-preview {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-secondary);
}

.ai-history-empty {
  padding: 12px;
  text-align: center;
  color: var(--text-dim);
  font-size: 11px;
}

.ai-chat {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  min-height: 0;
}

.ai-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 200px;
}

.ai-permission {
  width: 100%;
  max-width: 260px;
  padding: 16px 12px;
}

.ai-perm-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 10px;
  text-align: center;
}

.ai-perm-body {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.6;
}

.ai-perm-body p {
  margin: 0 0 8px 0;
}

.ai-perm-body ul {
  margin: 0;
  padding-left: 18px;
}

.ai-perm-body li {
  margin-bottom: 4px;
}

.ai-perm-actions {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
}

.ai-messages {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ai-message {
  display: flex;
}

.ai-message.user {
  justify-content: flex-end;
}

.ai-message.assistant {
  justify-content: flex-start;
}

.ai-bubble {
  max-width: 90%;
  padding: 8px 10px;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.5;
}

.ai-message.user .ai-bubble {
  background: var(--accent-bg);
  color: var(--text-primary);
}

.ai-message.assistant .ai-bubble {
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

.ai-text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--ide-code-font);
  font-size: 12px;
}

.ai-memory-panel {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  margin-bottom: 8px;
  overflow: hidden;
}

.ai-memory-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-bottom: 1px solid var(--border-color);
  font-size: 11px;
}

.ai-memory-title {
  font-weight: 600;
  color: var(--text-secondary);
}

.ai-memory-hint {
  flex: 1;
  color: var(--text-dim);
  font-size: 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-memory-close {
  color: var(--text-dim);
  padding: 0 4px !important;
}

.ai-memory-textarea {
  padding: 6px 8px !important;
}

.ai-input-area {
  border-top: 1px solid var(--border-color);
  padding: 8px;
  flex-shrink: 0;
}

.ai-input-box {
  margin-bottom: 6px;
}

.ai-textarea {
  resize: none;
}

.ai-input-tools {
  display: flex;
  align-items: center;
  gap: 4px;
}

.ai-model-select-bottom {
  width: 110px;
  flex-shrink: 0;
}

.ai-tools-spacer {
  flex: 1;
}

.ai-tool-btn {
  font-size: 16px;
  flex-shrink: 0;
}

.ai-tool-active {
  color: var(--accent-color) !important;
  background: var(--accent-bg) !important;
  border-radius: 4px;
}

.ai-send-btn {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  padding: 0 !important;
  font-size: 14px;
  border-radius: 6px;
  background: var(--accent-color) !important;
  color: white !important;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
}

.ai-send-btn:disabled {
  opacity: 0.5;
  background: var(--accent-color) !important;
  color: white !important;
}

.ai-stop-btn {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  padding: 0 !important;
  font-size: 14px;
  border-radius: 6px;
  background: var(--error-color, #e74c3c) !important;
  color: white !important;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* P2-11 危险工具人机确认对话框 */
.ai-tool-confirm {
  margin: 8px 12px;
  padding: 10px;
  border-radius: 8px;
  border: 1px solid var(--border-color);
  background: var(--bg-secondary);
  box-shadow: var(--shadow-md);
}

.ai-tool-confirm-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
}

.ai-tool-confirm-icon {
  font-size: 14px;
}

.ai-tool-confirm-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}

.ai-tool-risk {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
  font-weight: 500;
}

.ai-tool-risk.risk-safe      { background: var(--color-success); color: var(--color-success); opacity: 0.7; }
.ai-tool-risk.risk-moderate  { background: var(--color-warning); color: var(--color-warning); opacity: 0.7; }
.ai-tool-risk.risk-dangerous { background: var(--color-danger); color: var(--color-danger); opacity: 0.7; }

.ai-tool-confirm-body {
  margin-bottom: 8px;
}

.ai-tool-name {
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 4px;
}

.ai-tool-summary {
  font-size: 12px;
  color: var(--text-primary);
  margin-bottom: 6px;
  word-break: break-all;
}

.ai-tool-params {
  padding: 6px 8px;
  background: var(--bg-hover);
  border-radius: 4px;
  font-size: 11px;
}

.ai-tool-param {
  display: flex;
  gap: 4px;
  margin-bottom: 2px;
}

.ai-tool-param .param-key {
  color: var(--text-dim);
  flex-shrink: 0;
}

.ai-tool-param .param-val {
  color: var(--text-primary);
  word-break: break-all;
}

.ai-tool-confirm-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
</style>
