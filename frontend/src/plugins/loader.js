// IDE 插件加载器（G5 接口 + G6 加载机制 + G8 命令注册）
//
// 插件目录结构：
//   <exe目录>/plugins/<插件名>/
//     package.json   # { name, version, author, description, main }
//     main.js        # 插件主入口，导出 activate(api) 函数
//
// 插件 main.js 示例：
//   export function activate(api) {
//     api.output('插件已加载: ' + api.name)
//     api.registerCommand('my.hello', '我的命令:你好', () => {
//       api.output('你好！')
//       api.setStatus('已执行 my.hello')
//     })
//   }
//
// 加载机制：
//   1. IDE 启动时调用 ScanPlugins() 获取插件列表
//   2. 对每个插件，调用 ReadPluginFile(name, main) 读取 main.js 内容
//   3. 用 new Function() 包装成模块，执行 activate(api)
//   4. 错误隔离：单个插件加载失败不影响其他插件

import { ref } from 'vue'

// 已加载插件列表（供插件管理器 UI 显示）
const loadedPlugins = ref([])
// 插件注册的命令（供命令面板调用）
const pluginCommands = ref([]) // [{ id, label, handler, pluginName }]
// 插件注册的菜单项（供菜单显示）
const pluginMenuItems = ref([]) // [{ id, label, handler, pluginName }]
// 插件注册的补全项（供编辑器补全）
const pluginCompletions = ref([]) // [{ label, insertText, detail, pluginName }]
// 插件注册的面板（G7 预留）
const pluginPanels = ref([]) // [{ id, icon, label, render, pluginName }]
// 插件注册的窗口设计器组件（G9 预留）
const pluginComponents = ref([]) // [{ type, label, icon, defaultProps, pluginName }]

// 创建插件 API 对象（每个插件独立一份，避免状态串扰）
function createPluginAPI(pluginInfo, hooks) {
  return {
    // 插件自身信息
    name: pluginInfo.name,
    version: pluginInfo.version,
    dir: pluginInfo.dir,

    // 输出到输出面板
    output(text) {
      if (hooks && hooks.output) hooks.output('[' + pluginInfo.name + '] ' + text)
    },

    // 状态栏
    getStatus() { return hooks && hooks.getStatus ? hooks.getStatus() : '' },
    setStatus(msg) { if (hooks && hooks.setStatus) hooks.setStatus(msg) },

    // 命令注册（G8）
    registerCommand(id, label, handler) {
      pluginCommands.value.push({ id, label, handler, pluginName: pluginInfo.name })
    },

    // 菜单项注册
    registerMenuItem(id, label, handler) {
      pluginMenuItems.value.push({ id, label, handler, pluginName: pluginInfo.name })
    },

    // 代码补全注册
    registerCompletion(item) {
      pluginCompletions.value.push({ ...item, pluginName: pluginInfo.name })
    },

    // 自定义面板注册（G7 预留）
    registerPanel(id, icon, label, render) {
      pluginPanels.value.push({ id, icon, label, render, pluginName: pluginInfo.name })
    },

    // 窗口设计器组件注册（G9 预留）
    registerComponent(def) {
      pluginComponents.value.push({ ...def, pluginName: pluginInfo.name })
    },

    // 文件操作
    getActiveFile() { return hooks && hooks.getActiveFile ? hooks.getActiveFile() : null },
    openFile(path) { if (hooks && hooks.openFile) hooks.openFile(path) },

    // 项目操作
    getProjectPath() { return hooks && hooks.getProjectPath ? hooks.getProjectPath() : '' },

    // 调用后端 IDEService
    callBackend(method, ...args) {
      if (hooks && hooks.callBackend) return hooks.callBackend(method, ...args)
      return Promise.reject(new Error('callBackend 不可用'))
    }
  }
}

// 加载单个插件
async function loadPlugin(pluginInfo, hooks) {
  try {
    if (!window.IDEService || !window.IDEService.ReadPluginFile) {
      console.warn('[plugin] IDEService.ReadPluginFile 不可用，跳过插件 ' + pluginInfo.name)
      return null
    }
    const code = await window.IDEService.ReadPluginFile(pluginInfo.dir, pluginInfo.main)
    if (!code) {
      console.warn('[plugin] 插件 ' + pluginInfo.name + ' 的 ' + pluginInfo.main + ' 为空或不存在')
      return null
    }

    const api = createPluginAPI(pluginInfo, hooks)

    // 用 new Function 包装插件代码，提供 export/activate 机制
    // 插件 main.js 可以用 `export function activate(api) {...}` 或 `function activate(api) {...}`
    // 用 ES module 方式更自然，但运行时无法直接 import 本地文件，所以用 Function 包装
    const wrapper = new Function('api', `
      "use strict";
      let __activate = null;
      // 模拟 export 机制
      const module = { exports: {} };
      const exports = module.exports;
      try {
        ${code}
      } catch(e) {
        throw new Error('插件 ' + ${JSON.stringify(pluginInfo.name)} + ' 执行失败: ' + e.message);
      }
      // 支持 export function activate / module.exports.activate / function activate
      if (typeof activate === 'function') return activate(api);
      if (module.exports && typeof module.exports.activate === 'function') return module.exports.activate(api);
      if (typeof __activate === 'function') return __activate(api);
      throw new Error('插件 ' + ${JSON.stringify(pluginInfo.name)} + ' 未导出 activate(api) 函数');
    `)

    const activateResult = wrapper(api)
    // activate 可能返回 Promise（异步初始化）或直接返回
    if (activateResult && typeof activateResult.then === 'function') {
      await activateResult
    }

    loadedPlugins.value.push(pluginInfo)
    if (hooks && hooks.output) {
      hooks.output('[plugin] 插件已加载: ' + pluginInfo.name + ' v' + pluginInfo.version)
    }
    return pluginInfo
  } catch (e) {
    console.error('[plugin] 加载插件 ' + pluginInfo.name + ' 失败:', e)
    if (hooks && hooks.output) {
      hooks.output('[plugin] 加载插件 ' + pluginInfo.name + ' 失败: ' + e.message)
    }
    return null
  }
}

// 加载所有插件（IDE 启动时调用）
async function loadAllPlugins(hooks) {
  loadedPlugins.value = []
  pluginCommands.value = []
  pluginMenuItems.value = []
  pluginCompletions.value = []
  pluginPanels.value = []
  pluginComponents.value = []

  if (!window.IDEService || !window.IDEService.ScanPlugins) {
    return { ok: true, count: 0, reason: 'no IDEService' }
  }

  let plugins
  try {
    plugins = await window.IDEService.ScanPlugins()
  } catch (e) {
    return { ok: false, reason: String(e) }
  }

  if (!Array.isArray(plugins) || plugins.length === 0) {
    return { ok: true, count: 0 }
  }

  let count = 0
  for (const p of plugins) {
    if (!p || !p.enabled) continue
    const result = await loadPlugin(p, hooks)
    if (result) count++
  }

  return { ok: true, count }
}

// 执行插件注册的命令
function executePluginCommand(commandId) {
  const cmd = pluginCommands.value.find(c => c.id === commandId)
  if (!cmd) {
    console.warn('[plugin] 命令不存在: ' + commandId)
    return false
  }
  try {
    cmd.handler()
    return true
  } catch (e) {
    console.error('[plugin] 执行命令 ' + commandId + ' 失败:', e)
    return false
  }
}

// 获取所有插件命令（供命令面板显示）
function getPluginCommands() {
  return pluginCommands.value.slice()
}

// 获取所有插件菜单项
function getPluginMenuItems() {
  return pluginMenuItems.value.slice()
}

// 获取所有插件补全项
function getPluginCompletions() {
  return pluginCompletions.value.slice()
}

// 获取已加载插件列表
function getLoadedPlugins() {
  return loadedPlugins.value.slice()
}

export {
  loadAllPlugins,
  loadPlugin,
  executePluginCommand,
  getPluginCommands,
  getPluginMenuItems,
  getPluginCompletions,
  getLoadedPlugins,
  pluginCommands,
  pluginMenuItems,
  pluginCompletions,
  pluginPanels,
  pluginComponents,
  loadedPlugins
}
