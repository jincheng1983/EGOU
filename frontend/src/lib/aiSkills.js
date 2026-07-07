// EGOU AI 技能系统 - AI可调用的IDE工具

// 内置技能定义
export const BUILTIN_SKILLS = [
  {
    id: 'get_current_file',
    name: '获取当前文件',
    desc: '获取编辑器中当前打开文件的内容和路径',
    icon: '📄',
    parameters: {
      type: 'object',
      properties: {}
    },
    builtin: true,
    autoTrigger: ['当前文件', '打开的文件', '现在编辑']
  },
  {
    id: 'get_project_structure',
    name: '获取项目结构',
    desc: '获取当前项目的目录结构和文件列表',
    icon: '📁',
    parameters: {
      type: 'object',
      properties: {}
    },
    builtin: true,
    autoTrigger: ['项目结构', '目录', '文件列表', '有哪些文件']
  },
  {
    id: 'read_file',
    name: '读取文件',
    desc: '读取项目中指定文件的内容',
    icon: '📖',
    parameters: {
      type: 'object',
      properties: {
        path: { type: 'string', description: '文件路径（相对于项目根目录）' }
      },
      required: ['path']
    },
    builtin: true
  },
  {
    id: 'search_code',
    name: '搜索代码',
    desc: '在项目中搜索代码内容',
    icon: '🔍',
    parameters: {
      type: 'object',
      properties: {
        pattern: { type: 'string', description: '搜索关键词或正则表达式' },
        filePattern: { type: 'string', description: '文件匹配模式（如 *.eg）' }
      },
      required: ['pattern']
    },
    builtin: true,
    autoTrigger: ['搜索', '查找', '找一下', '哪里用到了']
  },
  {
    id: 'get_errors',
    name: '获取错误列表',
    desc: '获取当前编译错误和诊断信息',
    icon: '⚠️',
    parameters: {
      type: 'object',
      properties: {}
    },
    builtin: true,
    autoTrigger: ['错误', '报错', '问题', '诊断']
  },
  {
    id: 'get_support_libs',
    name: '获取支持库',
    desc: '获取所有可用的内置支持库命令列表',
    icon: '📚',
    parameters: {
      type: 'object',
      properties: {}
    },
    builtin: true,
    autoTrigger: ['支持库', '命令列表', '有哪些命令', '可用函数']
  },
  {
    id: 'insert_code_snippet',
    name: '插入代码片段',
    desc: '在编辑器当前光标位置插入代码',
    icon: '✏️',
    parameters: {
      type: 'object',
      properties: {
        code: { type: 'string', description: '要插入的EGOU代码' }
      },
      required: ['code']
    },
    builtin: true
  },
  {
    id: 'create_new_file',
    name: '创建新文件',
    desc: '在项目中创建新的源码文件',
    icon: '📝',
    parameters: {
      type: 'object',
      properties: {
        name: { type: 'string', description: '文件名（如 我的模块.eg）' },
        type: { type: 'string', enum: ['source', 'module', 'class', 'window'], description: '文件类型' },
        content: { type: 'string', description: '初始内容' }
      },
      required: ['name', 'type']
    },
    builtin: true
  },
  {
    id: 'explain_syntax',
    name: '语法解释',
    desc: '查询EGOU特定语法的用法和示例',
    icon: '❓',
    parameters: {
      type: 'object',
      properties: {
        keyword: { type: 'string', description: '语法关键词（如 如果、循环、函数等）' }
      },
      required: ['keyword']
    },
    builtin: true,
    autoTrigger: ['怎么用', '语法', '是什么意思', '如何写']
  }
]

// EGOU 语法参考（内置）
export const EG_SYNTAX_REFERENCE = {
  '变量': {
    syntax: '定义 变量名 类型 = 初始值',
    example: '定义 计数 整数 = 0\n定义 姓名 文本 = "张三"\n定义 价格 小数 = 9.9\n定义 是否成功 逻辑 = 真',
    note: '类型包括：整数、文本、小数、逻辑、字节集'
  },
  '常量': {
    syntax: '常量 常量名 类型 = 值',
    example: '常量 PI 小数 = 3.14159\n常量 版本号 文本 = "1.0.0"',
    note: '常量值不可修改'
  },
  '函数': {
    syntax: '函数 函数名(参数列表) 返回值类型 {\n  // 函数体\n  返回 返回值\n}',
    example: '函数 相加(a 整数, b 整数) 整数 {\n  返回 a + b\n}',
    note: '无返回值时省略返回值类型'
  },
  '方法': {
    syntax: '方法 方法名(参数列表) 返回值类型 {\n  // 方法体\n}',
    note: '方法属于类，在 类...结束类 内部定义'
  },
  '如果': {
    syntax: '如果 (条件) {\n  // 条件为真时执行\n} 否则 {\n  // 条件为假时执行\n}\n结束如果',
    example: '如果 (分数 >= 60) {\n  信息框("结果", "及格了")\n} 否则 {\n  信息框("结果", "没及格")\n}\n结束如果',
    note: '否则 部分可以省略'
  },
  '判断循环': {
    syntax: '判断循环 (条件) {\n  // 循环体\n}\n结束循环',
    example: '定义 i 整数 = 0\n判断循环 (i < 10) {\n  调试输出(到文本(i))\n  i = i + 1\n}\n结束循环',
    note: '先判断条件，为真则执行循环体（while循环）'
  },
  '循环': {
    syntax: '循环 (初始化;条件;增量) {\n  // 循环体\n}\n结束循环',
    example: '定义 i 整数\n循环 (i = 1; i <= 10; i++) {\n  调试输出("第" + 到文本(i) + "次")\n}\n结束循环',
    note: '标准for循环'
  },
  '选择': {
    syntax: '选择 表达式 {\n  情况 值1:\n    // 代码\n  情况 值2:\n    // 代码\n  默认:\n    // 默认代码\n}\n结束选择',
    example: '选择 等级 {\n  情况 "A":\n    信息框("结果", "优秀")\n  情况 "B":\n    信息框("结果", "良好")\n  默认:\n    信息框("结果", "继续努力")\n}\n结束选择',
    note: '多分支选择（switch语句）'
  },
  '类': {
    syntax: '类 类名\n  定义 成员变量 类型\n  \n  方法 初始化() {\n    // 构造方法\n  }\n  \n  方法 其他方法() {\n    // 方法体\n  }\n结束类',
    note: '类定义在 types/ 目录下的 .eg 文件中'
  },
  '模块': {
    syntax: '模块 模块名\n\n// 公开函数（可被外部调用）\n函数 公开函数() {\n}\n\n// 私有函数（仅模块内部使用）',
    note: '模块定义在 modules/ 目录下的 .eg 文件中'
  }
}

// 技能自动匹配
export function matchSkill(userMessage, availableSkills) {
  if (!userMessage) return null
  const msg = userMessage.toLowerCase()
  for (const skill of availableSkills) {
    if (!skill.autoTrigger) continue
    for (const keyword of skill.autoTrigger) {
      if (msg.includes(keyword.toLowerCase())) {
        return skill.id
      }
    }
  }
  return null
}

// 技能调用结果类型
export const SKILL_RESULT_PREFIX = '📎【技能调用结果】'

// 格式化技能结果给AI
export function formatSkillResult(skillName, result) {
  return `${SKILL_RESULT_PREFIX} ${skillName}\n${result}\n${SKILL_RESULT_PREFIX}结束`
}

// 默认技能目录
export const DEFAULT_SKILLS_DIR = 'skills'

// 技能文件格式（项目skills/目录下.json文件）：
// {
//   "id": "custom_skill",
//   "name": "自定义技能名",
//   "desc": "技能描述",
//   "icon": "🔧",
//   "parameters": {
//     "type": "object",
//     "properties": { ... },
//     "required": [ ... ]
//   },
//   "prompt": "使用此技能时的提示词",
//   "builtin": false
// }

// ===== P2-10 AI 技能按需加载 =====
//
// 设计目标：
//   - 技能定义不占上下文 token（只有 trigger 匹配时才注入 prompt）
//   - 区分 builtin/custom 来源（builtin 不可删除，custom 持久化到 localStorage）
//   - 支持 SaveCustomSkill / DeleteCustomSkill / Load / Unload 全生命周期
//   - injectSkillPrompt 按需把技能 prompt 注入到对话历史

const CUSTOM_SKILLS_STORAGE_KEY = 'eg-custom-skills'
const LOADED_SKILLS_STORAGE_KEY = 'eg-loaded-skills'

// 已加载的技能 ID 集合（运行时状态，决定哪些技能可被自动触发）
let loadedSkillIds = new Set()

// 初始化：默认所有内置技能都是 loaded 状态
;(() => {
  try {
    const saved = JSON.parse(localStorage.getItem(LOADED_SKILLS_STORAGE_KEY) || '[]')
    if (Array.isArray(saved) && saved.length > 0) {
      loadedSkillIds = new Set(saved)
    } else {
      // 默认加载所有内置技能
      BUILTIN_SKILLS.forEach(s => loadedSkillIds.add(s.id))
    }
  } catch (e) {
    BUILTIN_SKILLS.forEach(s => loadedSkillIds.add(s.id))
  }
})()

function persistLoadedSkills() {
  localStorage.setItem(LOADED_SKILLS_STORAGE_KEY, JSON.stringify([...loadedSkillIds]))
}

// 读取 localStorage 中的自定义技能列表
function readCustomSkills() {
  try {
    const data = JSON.parse(localStorage.getItem(CUSTOM_SKILLS_STORAGE_KEY) || '[]')
    return Array.isArray(data) ? data : []
  } catch (e) { return [] }
}

function writeCustomSkills(skills) {
  localStorage.setItem(CUSTOM_SKILLS_STORAGE_KEY, JSON.stringify(skills))
}

// SaveCustomSkill：保存或更新自定义技能（id 冲突时覆盖）
export function SaveCustomSkill(skill) {
  if (!skill || !skill.id || !skill.name) {
    return { success: false, error: '技能 id 和 name 必填' }
  }
  if (BUILTIN_SKILLS.some(s => s.id === skill.id)) {
    return { success: false, error: '不能覆盖内置技能' }
  }
  const skills = readCustomSkills()
  const idx = skills.findIndex(s => s.id === skill.id)
  const newSkill = {
    id: skill.id,
    name: skill.name,
    desc: skill.desc || '',
    icon: skill.icon || '🔧',
    parameters: skill.parameters || { type: 'object', properties: {} },
    prompt: skill.prompt || '',
    builtin: false,
    autoTrigger: skill.autoTrigger || []
  }
  if (idx >= 0) skills[idx] = newSkill
  else skills.push(newSkill)
  writeCustomSkills(skills)
  return { success: true, skill: newSkill }
}

// DeleteCustomSkill：删除自定义技能（不能删除内置）
export function DeleteCustomSkill(skillId) {
  if (BUILTIN_SKILLS.some(s => s.id === skillId)) {
    return { success: false, error: '不能删除内置技能' }
  }
  const skills = readCustomSkills()
  const filtered = skills.filter(s => s.id !== skillId)
  writeCustomSkills(filtered)
  loadedSkillIds.delete(skillId)
  persistLoadedSkills()
  return { success: true }
}

// Load：启用某技能（参与自动触发）
export function LoadSkill(skillId) {
  loadedSkillIds.add(skillId)
  persistLoadedSkills()
  return { success: true }
}

// Unload：禁用某技能（不参与自动触发，但定义仍保留）
export function UnloadSkill(skillId) {
  loadedSkillIds.delete(skillId)
  persistLoadedSkills()
  return { success: true }
}

// 获取所有已加载的技能（builtin + custom 中 loaded=true 的）
export function GetLoadedSkills() {
  const custom = readCustomSkills()
  const all = [...BUILTIN_SKILLS, ...custom]
  return all.filter(s => loadedSkillIds.has(s.id))
}

// 获取所有自定义技能
export function ListCustomSkills() {
  return readCustomSkills()
}

// injectSkillPrompt：把技能 prompt 按需注入到对话历史
// 仅当技能被加载且 trigger 匹配时注入，避免占用上下文 token
export function injectSkillPrompt(userMessage, history) {
  if (!userMessage) return history
  const loaded = GetLoadedSkills()
  const matched = []
  for (const skill of loaded) {
    if (!skill.autoTrigger || skill.autoTrigger.length === 0) continue
    const msg = userMessage.toLowerCase()
    for (const kw of skill.autoTrigger) {
      if (msg.includes(kw.toLowerCase())) {
        matched.push(skill)
        break
      }
    }
  }
  if (matched.length === 0) return history
  // 把匹配到的技能 prompt 注入为 system message（放在 history 开头）
  const injection = matched.map(s =>
    `【已加载技能：${s.name}】\n${s.prompt || s.desc || ''}`
  ).join('\n\n')
  return [
    { role: 'system', content: '以下技能已按需加载，可参考其说明回答用户：\n\n' + injection },
    ...history
  ]
}
