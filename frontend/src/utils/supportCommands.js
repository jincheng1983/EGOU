// 兼容层：保留旧 import 路径，导出与原 supportCommands.js 同名同语义的对象。
// 真实数据从 src/lib/index.js 聚合加载而来。

export {
  supportCommands,
  supportTree,
  labelToKey,
  findHelp,
  getCommandMeta,
  getLibraries,
  // 新增：合并视图（内置 + 项目 libs）
  getMergedTree,
  getMergedCommands,
  getMergedLabelToKey,
  getMergedCommandMeta,
  loadProjectLibs,
  unloadProjectLibs,
  getProjectLibsSummary,
  getCurrentProjectRoot,
  loadGlobalLibs,
  getGlobalLibsSummary,
  libVersion
} from '../lib/index.js'
