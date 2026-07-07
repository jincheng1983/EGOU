// 公共颜色常量 — 供 ProjectExplorer / FileExplorer / SupportPanel 共享
// 彩虹流程线颜色（深一档用于 SupportPanel，与 ProjectExplorer/FileExplorer 区分层次）

// 标准彩虹色（ProjectExplorer / FileExplorer 使用）
export const FLOW_RAINBOW = [
  '#ff6b6b', // 红
  '#ffa94d', // 橙
  '#ffd43b', // 黄
  '#69db7c', // 绿
  '#4dabf7', // 蓝
  '#9775fa', // 紫
  '#f783ac'  // 粉
]

// 深一档彩虹色（SupportPanel 使用，与项目树区分层次）
export const FLOW_RAINBOW_DARK = [
  '#e03131', // 深红
  '#e8590c', // 深橙
  '#f08c00', // 深黄
  '#2f9e44', // 深绿
  '#1971c2', // 深蓝
  '#7048e8', // 深紫
  '#c2255c'  // 深粉
]

// 项目树节点类型颜色（ProjectExplorer 使用）
export const TYPE_COLOR = {
  source:     '#5b8def',
  srcfile:    '#5b8def',
  assembly:   '#5b8def',
  winassembly:'#a371f7',
  window:     '#a371f7',
  module:     '#1aab6a',
  class:      '#e08e3f',
  func:       '#3aa0ff',
  method:     '#3aa0ff',
  var:        '#d96b6d',
  constant:   '#d2a23a',
  dll:        '#bf6ad1',
  resource:   '#d2a23a',
  sound:      '#e05a5a',
  image:      '#4dabf7',
  libref:     '#9775fa',
}

// 组件分组标识色（WindowDesigner 使用，同组组件共享虚线 outline 颜色）
export const GROUP_COLORS = ['#ff6b6b', '#4dabf7', '#51cf66', '#fcc419', '#9775fa', '#ff922b', '#20c997', '#f06595']
