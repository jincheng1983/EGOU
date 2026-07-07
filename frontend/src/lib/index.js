// 支持库聚合加载器（ycIDE ycmd 风格）
//
// 两层数据：
//   1. 内置库：Vite 构建时静态导入，krnln + egou
//   2. 项目库：IDE 运行时通过 IDEService 扫 <项目>/libs/*.elib/commands.json
//
// 暴露的 API 与旧版 supportCommands.js 兼容：
//   - supportCommands / supportTree / labelToKey / commandMeta  // 内置库静态
//   - getMergedCommands() / getMergedTree() / getMergedLabelToKey() / getMergedCommandMeta()
//     // 合并视图，每次调用都返回最新（内置 + 项目）
//   - loadProjectLibs(projectPath) / unloadProjectLibs() / getProjectLibsSummary()
//   - libVersion // Vue ref，每次 load/unload 后 +1，组件 watch 它刷新

import { ref } from 'vue'
import krnlnLib from './krnln/library.json'
import krnlnCmds from './krnln/commands.json'
import egouLib from './egou/library.json'
import egouCore from './egou/core.commands.json'
import egouUi from './egou/ui.commands.json'
import egouNet from './egou/net.commands.json'

// ============================================================
// 内置库（构建时静态加载）
// ============================================================

const builtinLibraries = [krnlnLib, egouLib]
const builtinCommandSets = [
  { library: krnlnLib, file: 'krnln/commands.json', commands: krnlnCmds.commands || [], origin: 'builtin' },
  { library: egouLib, file: 'egou/core.commands.json', commands: egouCore.commands || [] },
  { library: egouLib, file: 'egou/ui.commands.json', commands: egouUi.commands || [] },
  { library: egouLib, file: 'egou/net.commands.json', commands: egouNet.commands || [] }
].map(s => ({ ...s, origin: 'builtin' }))

function buildHelpString(cmd) {
  const params = (cmd.params || []).map(p => p.optional ? `[${p.name}]` : p.name).join(', ')
  const ret = cmd.returnType && cmd.returnType !== '无返回值' ? ` ${cmd.returnType}：` : '：'
  return `${cmd.displayName || cmd.englishName || cmd.commandId}(${params})${ret}${cmd.summary || ''}`
}

function buildFromCommandSets(commandSets) {
  const commandMeta = {}
  const labelToKey = {}
  const supportCommands = {}
  const childrenByLib = new Map()
  for (const set of commandSets) {
    if (!childrenByLib.has(set.library.library)) {
      childrenByLib.set(set.library.library, { library: set.library, items: [] })
    }
    const bucket = childrenByLib.get(set.library.library)
    for (const cmd of set.commands) {
      const key = cmd.englishName || cmd.commandId.split('.').pop()
      commandMeta[key] = cmd
      if (cmd.displayName) labelToKey[cmd.displayName] = key
      supportCommands[key] = buildHelpString(cmd)
      bucket.items.push({
        label: cmd.displayName || key,
        key,
        meta: cmd,
        category: cmd.category || '其他'
      })
    }
  }
  const supportTree = []
  for (const lib of builtinLibraries) {
    const bucket = childrenByLib.get(lib.library)
    supportTree.push({
      label: lib.displayName || lib.library,
      key: lib.library,
      children: bucket ? bucket.items : []
    })
  }
  return { commandMeta, labelToKey, supportCommands, supportTree }
}

const built = buildFromCommandSets(builtinCommandSets)
const commandMeta = built.commandMeta
const labelToKey = built.labelToKey
const supportCommands = built.supportCommands
const supportTree = built.supportTree

// ============================================================
// 项目库（运行时动态加载）
// ============================================================

// libVersion 每次 load/unload +1，组件 watch 它刷新
const libVersion = ref(0)

// projectLibSets 持有当前项目加载到的所有 .elib。
// 每项形同 builtinCommandSets 的元素：{ library, file, commands, origin: 'project' }
const projectLibSets = []
// projectLibSummary 存 .elib 元数据（package.json + commands.json 里的元信息），用于支持库面板 hover 提示
const projectLibSummary = []
// 当前项目根目录，IDE 关项目时用来清理
let currentProjectRoot = null

// ============================================================
// 全局库（exe 同级 libs/，所有项目共享，G1）
// ============================================================

// globalLibSets 与 projectLibSets 结构相同，但 origin: 'global'。
// IDE 启动时 loadGlobalLibs() 扫描 exe 同级 libs/ 填充，项目切换时不清理。
const globalLibSets = []
const globalLibSummary = []

// loadGlobalLibs 扫描 exe 同级 libs/ 目录，加载所有 .elib 扩展包。
// 在 IDE 启动时调用一次，所有项目共享。失败不阻断（全局 libs 目录可能不存在）。
async function loadGlobalLibs() {
  globalLibSets.length = 0
  globalLibSummary.length = 0
  if (typeof window === 'undefined' || !window.IDEService || !window.IDEService.ScanGlobalLibs) {
    libVersion.value++
    return { ok: true, count: 0, reason: 'no IDEService' }
  }
  let entries
  try {
    entries = await window.IDEService.ScanGlobalLibs()
  } catch (e) {
    libVersion.value++
    return { ok: false, reason: String(e) }
  }
  if (!Array.isArray(entries)) {
    libVersion.value++
    return { ok: true, count: 0 }
  }
  let count = 0
  for (const entry of entries) {
    if (!entry || !entry.commands) continue
    const library = {
      library: entry.name,
      displayName: entry.displayName || entry.name,
      version: entry.version || '0.0.0',
      description: entry.description || '',
      author: entry.author || '',
      origin: 'global',
      packageDir: entry.path
    }
    globalLibSets.push({ library, file: entry.path + '/commands.json', commands: entry.commands, origin: 'global' })
    globalLibSummary.push({
      dir: entry.dir,
      path: entry.path,
      name: entry.name,
      displayName: entry.displayName,
      version: entry.version,
      description: entry.description,
      author: entry.author,
      commandCount: entry.commandCount
    })
    count++
  }
  libVersion.value++
  return { ok: true, count }
}

function getGlobalLibsSummary() {
  return globalLibSummary.slice()
}

// IDE 主进程的 .elib 扫描/读取入口
async function loadProjectLibs(projectPath) {
  if (!projectPath) return { ok: false, reason: 'projectPath empty' }
  unloadProjectLibs()
  currentProjectRoot = projectPath
  const libsRoot = joinPath(projectPath, 'libs')
  const entries = await safeListDir(libsRoot)
  if (!entries) {
    libVersion.value++
    return { ok: true, count: 0, path: libsRoot }
  }
  let count = 0
  for (const entry of entries) {
    if (!entry.isDir) continue
    const pkgDir = joinPath(libsRoot, entry.name)
    const cmdsFile = joinPath(pkgDir, 'commands.json')
    const cmdsJson = await safeReadFile(cmdsFile)
    if (!cmdsJson) continue
    try {
      const parsed = JSON.parse(cmdsJson)
      const commands = parsed.commands || []
      // 尝试读 package.json 拿 name/version/author/description
      const pkgJson = await safeReadFile(joinPath(pkgDir, 'package.json'))
      let pkgMeta = {}
      if (pkgJson) {
        try { pkgMeta = JSON.parse(pkgJson) } catch {}
      }
      // 库元数据 fallback：commands.json 顶层的 library/libraryDisplayName 作为最小库
      const library = {
        library: parsed.library || pkgMeta.name || entry.name,
        displayName: parsed.libraryDisplayName || parsed.library || entry.name,
        version: parsed.libraryVersion || pkgMeta.version || '0.0.0',
        description: parsed.description || pkgMeta.description || '',
        author: parsed.author || pkgMeta.author || '',
        origin: 'project',
        packageDir: pkgDir
      }
      projectLibSets.push({ library, file: cmdsFile, commands, origin: 'project' })
      projectLibSummary.push({
        dir: entry.name,
        path: pkgDir,
        name: library.library,
        displayName: library.displayName,
        version: library.version,
        description: library.description,
        author: library.author,
        commandCount: commands.length
      })
      count++
    } catch (e) {
      // 单个 .elib 解析失败不影响其它
      console.warn(`[lib] ${entry.name}/commands.json 解析失败:`, e)
    }
  }
  libVersion.value++
  return { ok: true, count, path: libsRoot, libs: projectLibSummary.slice() }
}

function unloadProjectLibs() {
  projectLibSets.length = 0
  projectLibSummary.length = 0
  currentProjectRoot = null
  libVersion.value++
}

function getProjectLibsSummary() {
  return projectLibSummary.slice()
}

function getCurrentProjectRoot() {
  return currentProjectRoot
}

// ============================================================
// 合并视图：内置 + 全局 + 项目
// ============================================================

function buildMergedTree() {
  // 合并顺序：内置 → 全局（exe 同级 libs/）→ 项目（<项目>/libs/）
  // 项目级优先级最高，同名命令覆盖全局和内置。
  const merged = supportTree.slice()
  // 全局库
  for (const set of globalLibSets) {
    const lib = set.library
    const children = []
    for (const cmd of set.commands) {
      const key = cmd.englishName || cmd.commandId.split('.').pop()
      children.push({
        label: cmd.displayName || key,
        key,
        meta: cmd,
        category: cmd.category || '其他'
      })
    }
    merged.push({
      label: lib.displayName || lib.library,
      key: 'global:' + lib.library,
      children,
      origin: 'global',
      projectMeta: lib
    })
  }
  // 项目库
  for (const set of projectLibSets) {
    const lib = set.library
    const children = []
    for (const cmd of set.commands) {
      const key = cmd.englishName || cmd.commandId.split('.').pop()
      children.push({
        label: cmd.displayName || key,
        key,
        meta: cmd,
        category: cmd.category || '其他'
      })
    }
    merged.push({
      label: lib.displayName || lib.library,
      key: 'project:' + lib.library,
      children,
      origin: 'project',
      projectMeta: lib
    })
  }
  return merged
}

function buildMergedCommands() {
  // 合并优先级：内置 < 全局 < 项目（后者覆盖前者同名 key）
  const out = Object.assign({}, supportCommands)
  for (const set of globalLibSets) {
    for (const cmd of set.commands) {
      const key = cmd.englishName || cmd.commandId.split('.').pop()
      out[key] = buildHelpString(cmd)
    }
  }
  for (const set of projectLibSets) {
    for (const cmd of set.commands) {
      const key = cmd.englishName || cmd.commandId.split('.').pop()
      out[key] = buildHelpString(cmd)
    }
  }
  return out
}

function buildMergedLabelToKey() {
  const out = Object.assign({}, labelToKey)
  for (const set of globalLibSets) {
    for (const cmd of set.commands) {
      if (cmd.displayName) out[cmd.displayName] = cmd.englishName || cmd.commandId.split('.').pop()
    }
  }
  for (const set of projectLibSets) {
    for (const cmd of set.commands) {
      if (cmd.displayName) out[cmd.displayName] = cmd.englishName || cmd.commandId.split('.').pop()
    }
  }
  return out
}

function buildMergedCommandMeta() {
  const out = Object.assign({}, commandMeta)
  for (const set of globalLibSets) {
    for (const cmd of set.commands) {
      const key = cmd.englishName || cmd.commandId.split('.').pop()
      out[key] = cmd
    }
  }
  for (const set of projectLibSets) {
    for (const cmd of set.commands) {
      const key = cmd.englishName || cmd.commandId.split('.').pop()
      out[key] = cmd
    }
  }
  return out
}

// 这些 getter 每次都返回新对象（避免在 Vue computed 里被依赖同一个引用）
function getMergedTree() { return buildMergedTree() }
function getMergedCommands() { return buildMergedCommands() }
function getMergedLabelToKey() { return buildMergedLabelToKey() }
function getMergedCommandMeta() { return buildMergedCommandMeta() }

// ============================================================
// 旧 API 兼容 + 帮助/查找
// ============================================================

function findHelp(word) {
  if (!word) return null
  const merged = buildMergedCommandMeta()
  const mergedCmds = buildMergedCommands()
  if (mergedCmds[word]) {
    return { name: word, help: mergedCmds[word], meta: merged[word] }
  }
  const key = buildMergedLabelToKey()[word]
  if (key && mergedCmds[key]) {
    return { name: key, help: mergedCmds[key], meta: merged[key] }
  }
  return null
}

function getCommandMeta(key) {
  return buildMergedCommandMeta()[key] || null
}

function getLibraries() {
  return builtinLibraries.slice()
}

// 旧版 supportCommands（内置）保留直接导出，不带项目 lib；新版组件用 getter
export {
  supportCommands,
  supportTree,
  labelToKey,
  findHelp,
  getCommandMeta,
  getLibraries,
  commandMeta,
  builtinLibraries,
  builtinCommandSets,
  // 新版合并视图
  getMergedTree,
  getMergedCommands,
  getMergedLabelToKey,
  getMergedCommandMeta,
  // 项目库管理
  loadProjectLibs,
  unloadProjectLibs,
  getProjectLibsSummary,
  getCurrentProjectRoot,
  // 全局库管理（G1：exe 同级 libs/，所有项目共享）
  loadGlobalLibs,
  getGlobalLibsSummary,
  libVersion
}

// ============================================================
// 工具：路径拼装 + IDEService 文件访问
// ============================================================

function joinPath(a, b) {
  if (!a) return b
  if (!b) return a
  const sep = a.includes('\\') ? '\\' : '/'
  if (a.endsWith('\\') || a.endsWith('/')) return a + b
  return a + sep + b
}

// 用 IDEService（IDE 主进程）访问磁盘；浏览器端走 fetch('/api/...') 兜底
async function safeReadFile(path) {
  try {
    if (typeof window !== 'undefined' && window.IDEService && window.IDEService.ReadProjectFile) {
      const r = await window.IDEService.ReadProjectFile(path)
      if (r && !r.error && r.content) return r.content
    }
    if (typeof window !== 'undefined' && window.IDEService && window.IDEService.ReadFile) {
      const r = await window.IDEService.ReadFile(path)
      if (r && !r.error && typeof r === 'string') return r
    }
  } catch {}
  return null
}

async function safeListDir(path) {
  try {
    if (typeof window !== 'undefined' && window.IDEService && window.IDEService.ListProjectDir) {
      // ListProjectDir 是递归的，我们只取一层
      const nodes = await window.IDEService.ListProjectDir(path)
      if (Array.isArray(nodes)) return nodes.map(n => ({ name: n.Name, isDir: n.IsDir, path: n.Path }))
    }
  } catch {}
  return null
}
