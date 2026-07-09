<template>
  <div ref="hostRef" class="editor-host"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as monaco from 'monaco-editor'
import { IDEService } from '../../bindings/egou/internal/app'
import {
  KEYWORDS, TYPE_KEYWORDS, SNIPPETS, SUPPORT_ALIASES,
  BLOCK_STARTS, BLOCK_ENDS, BLOCK_MIDS,
  isKeyword, isTypeKeyword, isBlockStart, isBlockEnd, isSupportAlias
} from '../utils/egKeywords.js'
import { t } from '../i18n/index.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  projectPath: { type: String, default: '' },
  suggestions: { type: Array, default: () => [] },
  isDark: { type: Boolean, default: true },
  editorTheme: { type: String, default: 'auto' },
  fileId: { type: String, default: '' },
  minimapEnabled: { type: Boolean, default: false },
  fontSize: { type: Number, default: 14 },
  fontFamily: { type: String, default: "'IdeFont', 'Consolas', 'Courier New', monospace" },
  autoConvertSymbols: { type: Boolean, default: true },
  lineNumbersEnabled: { type: Boolean, default: true },
  lineHeight: { type: Number, default: 0 },
  tabSize: { type: Number, default: 2 },
  wordWrap: { type: Boolean, default: false },
  renderWhitespace: { type: String, default: 'selection' },
  cursorBlinking: { type: String, default: 'blink' },
  cursorSmoothCaretAnimation: { type: Boolean, default: true },
  cursorWidth: { type: Number, default: 0 },
  bracketPairColorization: { type: Boolean, default: true },
  guidesBracketPairs: { type: Boolean, default: false },
  fontLigatures: { type: Boolean, default: false },
  lineNumbersMinChars: { type: Number, default: 3 },
  renderFinalNewline: { type: Boolean, default: true },
  minimapShowSlider: { type: String, default: 'mouseover' },
  minimapRenderCharacters: { type: Boolean, default: true },
  minimapMaxColumn: { type: Number, default: 120 },
  projectPath: { type: String, default: '' }
})

const emit = defineEmits([
  'update:modelValue', 'cursor-change', 'show-help',
  'goto-def', 'find-refs', 'rename-symbol', 'font-size-change',
  'open-file-at',  // 跨文件跳转：{ file, line, col, word }
  'toggle-breakpoint',  // 调试断点切换：line
  'edit-breakpoint-condition'  // 编辑断点条件：line, currentCond
])

const hostRef = ref(null)
let editor = null
let resizeObserver = null

const CJK_MAP = {
  '\uFF08': '(', '\uFF09': ')', '\u3010': '[', '\u3011': ']', '\uFF5B': '{', '\uFF5D': '}',
  '\uFF0C': ',', '\u3002': '.', '\uFF1B': ';', '\uFF1A': ':',
  '\u201C': '"', '\u201D': '"', '\u2018': "'", '\u2019': "'",
  '\u300A': '<', '\u300B': '>', '\uFF1F': '?'
}

// KEYWORDS / TYPE_KEYWORDS / SNIPPETS / SUPPORT_ALIASES 已从 ../utils/egKeywords.js 导入
// 该文件是前后端关键字表的单一真源，与 internal/transpiler/lexer.go + transpiler.go 保持同步

function resolveTheme() {
  const t = props.editorTheme || 'auto'
  if (t === 'auto') return props.isDark ? 'vs-dark' : 'vs'
  if (t === 'dark') return 'vs-dark'
  if (t === 'light') return 'vs'
  return t
}

let symbolsConverted = false

onMounted(() => {
  if (!hostRef.value) return

  // 注册语言
  if (!monaco.languages.getLanguages().some(l => l.id === 'egou')) {
    monaco.languages.register({ id: 'egou' })
    monaco.languages.setMonarchTokensProvider('egou', {
      keywords: KEYWORDS,
      typeKeywords: TYPE_KEYWORDS,
      tokenizer: {
        root: [
          [/#.*$/, 'comment'],
          [/"(?:[^"\\]|\\.)*"/, 'string'],
          [/@\w+/, 'keyword'],
          [/[a-zA-Z_\u4e00-\u9fa5][a-zA-Z0-9_\u4e00-\u9fa5]*/, {
            cases: {
              '@typeKeywords': 'type',
              '@keywords': 'keyword',
              '@default': 'identifier'
            }
          }],
          [/[0-9]+/, 'number'],
          [/[{}()\[\]]/, '@brackets'],
          [/[;,.]/, 'delimiter'],
        ]
      }
    })
    monaco.languages.setLanguageConfiguration('egou', {
      autoClosingPairs: [],
      surroundingPairs: []
    })
  }

  editor = monaco.editor.create(hostRef.value, {
    value: props.modelValue || '',
    language: 'egou',
    theme: resolveTheme(),
    automaticLayout: true,
    fontSize: props.fontSize,
    fontFamily: props.fontFamily,
    minimap: { enabled: props.minimapEnabled },
    lineNumbers: props.lineNumbersEnabled ? 'on' : 'off',
    tabSize: props.tabSize,
    insertSpaces: true,
    wordWrap: props.wordWrap ? 'on' : 'off',
    scrollBeyondLastLine: false,
    roundedSelection: true,
    padding: { top: 8 },
    contextmenu: false,
    glyphMargin: true,
    multiCursorModifier: 'alt',
    bracketPairColorization: { enabled: true },
    matchBrackets: 'always',
    renderLineHighlight: 'all',
    selectionHighlight: true,
    occurrencesHighlight: 'singleFile',
    wordBasedSuggestions: 'currentDocument',
    guidesIndentation: true,
    guidesBracketPairs: true
  })

  // ===== 自定义右键菜单（中文化+格式化+注释） =====
  editor.addAction({
    id: 'format-document',
    label: '格式化文档',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyF],
    contextMenuGroupId: '9_cutcopyinsert',
    contextMenuOrder: 1.5,
    run: async (ed) => {
      const model = ed.getModel()
      if (!model) return
      const src = model.getValue()
      if (!src.trim()) return

      try {
         // 1. 先转译 EGOU → Go
         const transpileResp = await IDEService.Transpile(src)
         if (transpileResp.error) {
           console.warn('转译失败:', transpileResp.error)
           return
         }
         // 2. 格式化 Go 代码
         const formatResp = await IDEService.FormatCode(transpileResp.go)
         if (formatResp.error) {
           console.warn('格式化失败:', formatResp.error)
           return
         }
         if (formatResp.code) {
           // 显示格式化后的 Go 代码（替代编辑器内容）
           const fullRange = model.getFullModelRange()
           model.applyEdits([{ range: fullRange, text: formatResp.code }])
         }
      } catch (e) {
        console.warn('格式化异常:', e)
      }
    }
  })
  editor.addAction({
    id: 'format-selection',
    label: '格式化所选代码',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyK, monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyF],
    contextMenuGroupId: '9_cutcopyinsert',
    contextMenuOrder: 1.6,
    run: (ed) => ed.getAction('editor.action.formatSelection').run()
  })
  editor.addAction({
    id: 'toggle-comment',
    label: '注释/取消注释',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeySlash],
    contextMenuGroupId: '9_cutcopyinsert',
    contextMenuOrder: 1.7,
    run: (ed) => ed.getAction('editor.action.commentLine').run()
  })
  editor.addAction({
    id: 'add-line-comment',
    label: '添加行注释',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyA],
    contextMenuGroupId: '9_cutcopyinsert',
    contextMenuOrder: 1.8,
    run: (ed) => ed.getAction('editor.action.addCommentLine').run()
  })
  editor.addAction({
    id: 'remove-line-comment',
    label: '移除行注释',
    contextMenuGroupId: '9_cutcopyinsert',
    contextMenuOrder: 1.9,
    run: (ed) => ed.getAction('editor.action.removeCommentLine').run()
  })
  editor.addAction({
    id: 'block-comment',
    label: '添加块注释',
    keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyB],
    contextMenuGroupId: '9_cutcopyinsert',
    contextMenuOrder: 2.0,
    run: (ed) => {
      const selection = ed.getSelection()
      const model = ed.getModel()
      if (!selection || !model) return
      const text = model.getValueInRange(selection)
      const newText = '/* ' + text + ' */'
      ed.executeEdits('', [{ range: selection, text: newText }])
    }
  })

  // ===== 书签系统（P3-1 编辑器功能补完） =====
  // 书签存储：fileId -> Set<lineNumber>
  // 持久化：localStorage key = 'eg-bookmarks-' + fileId
  // glyph margin 显示：用装饰器在行号左侧绘制 ◆ 标记
  let bookmarkDecorations = []

  function loadBookmarks() {
    if (!props.fileId) return new Set()
    try {
      const saved = JSON.parse(localStorage.getItem('eg-bookmarks-' + props.fileId) || '[]')
      return new Set(Array.isArray(saved) ? saved : [])
    } catch (e) { return new Set() }
  }

  function saveBookmarks() {
    if (!props.fileId) return
    localStorage.setItem('eg-bookmarks-' + props.fileId, JSON.stringify([...bookmarks]))
  }

  function updateBookmarkDecorations() {
    if (!editor) return
    const model = editor.getModel()
    if (!model) return
    bookmarkDecorations = editor.deltaDecorations(bookmarkDecorations,
      [...bookmarks].map(line => ({
        range: new monaco.Range(line, 1, line, 1),
        options: {
          isWholeLine: false,
          glyphMarginClassName: 'eg-bookmark-glyph',
          glyphMarginHoverMessage: { value: t('editor.bookmark', { line }) },
          stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
        }
      }))
    )
  }

  const bookmarks = loadBookmarks()

  function toggleBookmark(line) {
    if (!editor) return
    const ln = line || editor.getPosition().lineNumber
    if (bookmarks.has(ln)) {
      bookmarks.delete(ln)
    } else {
      bookmarks.add(ln)
    }
    saveBookmarks()
    updateBookmarkDecorations()
  }

  function nextBookmark() {
    if (!editor || bookmarks.size === 0) return
    const cur = editor.getPosition().lineNumber
    const sorted = [...bookmarks].sort((a, b) => a - b)
    const next = sorted.find(l => l > cur)
    const target = next || sorted[0]
    editor.revealLineInCenter(target)
    editor.setPosition({ lineNumber: target, column: 1 })
    editor.focus()
  }

  function prevBookmark() {
    if (!editor || bookmarks.size === 0) return
    const cur = editor.getPosition().lineNumber
    const sorted = [...bookmarks].sort((a, b) => b - a)
    const prev = sorted.find(l => l < cur)
    const target = prev || sorted[0]
    editor.revealLineInCenter(target)
    editor.setPosition({ lineNumber: target, column: 1 })
    editor.focus()
  }

  function clearAllBookmarks() {
    bookmarks.clear()
    saveBookmarks()
    updateBookmarkDecorations()
  }

  // 初始渲染书签装饰器
  setTimeout(updateBookmarkDecorations, 100)

  // 书签快捷键：Ctrl+F2 切换 / Alt+F2 下一个 / Alt+Shift+F2 上一个 / F2 留给重命名
  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.F2, () => toggleBookmark())
  editor.addCommand(monaco.KeyMod.Alt | monaco.KeyCode.F2, () => nextBookmark())
  editor.addCommand(monaco.KeyMod.Alt | monaco.KeyMod.Shift | monaco.KeyCode.F2, () => prevBookmark())

  // glyph margin 点击：Shift+点击切换断点，右键编辑条件，普通点击切换书签
  editor.onMouseDown((e) => {
    if (e.target.type === monaco.editor.MouseTargetType.GUTTER_GLYPH_MARGIN) {
      const line = e.target.position.lineNumber
      if (!line) return
      // 右键：编辑断点条件（已有断点）或添加条件断点（无断点时先创建）
      if (e.event.rightButton) {
        e.event.preventDefault()
        if (!debugBreakpoints.has(line)) {
          debugBreakpoints.set(line, { cond: '' })
          updateBreakpointDecorations()
          emit('toggle-breakpoint', line)
        }
        emit('edit-breakpoint-condition', line, getBreakpointCondition(line))
        return
      }
      if (e.event.shiftKey) {
        emit('toggle-breakpoint', line)
      } else {
        toggleBookmark(line)
      }
    }
  })

  // ===== 断点 + 当前执行行（P2 调试器集成）=====
  // 断点：Map<line, {cond: string}>（cond 为空表示普通断点，非空表示条件断点）
  // 当前执行行（由 App.vue 通过 setCurrentLine 设置，调试暂停时高亮）
  let bpDecorations = []
  let currentLineDecoration = []
  let debugBreakpoints = new Map()
  let debugCurrentLine = 0

  function updateBreakpointDecorations() {
    if (!editor) return
    const entries = [...debugBreakpoints.entries()]
    bpDecorations = editor.deltaDecorations(bpDecorations,
      entries.map(([line, info]) => ({
        range: new monaco.Range(line, 1, line, 1),
        options: {
          isWholeLine: false,
          glyphMarginClassName: info.cond ? 'eg-breakpoint-conditional-glyph' : 'eg-breakpoint-glyph',
          glyphMarginHoverMessage: { value: info.cond ? t('editor.conditionalBreakpoint', { line, cond: info.cond }) : t('editor.breakpoint', { line }) },
          stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
        }
      }))
    )
  }

  function updateCurrentLineDecoration() {
    if (!editor) return
    if (debugCurrentLine > 0) {
      currentLineDecoration = editor.deltaDecorations(currentLineDecoration, [{
        range: new monaco.Range(debugCurrentLine, 1, debugCurrentLine, 1),
        options: {
          isWholeLine: true,
          className: 'eg-debug-current-line',
          stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
        }
      }, {
        // glyph margin 黄色箭头标记（VS Code 风格）
        range: new monaco.Range(debugCurrentLine, 1, debugCurrentLine, 1),
        options: {
          isWholeLine: false,
          glyphMarginClassName: 'eg-debug-current-glyph',
          glyphMarginHoverMessage: { value: t('editor.currentLine') },
          stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
        }
      }])
    } else {
      currentLineDecoration = editor.deltaDecorations(currentLineDecoration, [])
    }
  }

  function setBreakpoints(bps) {
    // 兼容两种格式：行号数组 [1,2,3] 或对象数组 [{line, cond}]
    debugBreakpoints = new Map()
    if (Array.isArray(bps)) {
      bps.forEach(item => {
        if (typeof item === 'number') {
          debugBreakpoints.set(item, { cond: '' })
        } else if (item && typeof item.line === 'number') {
          debugBreakpoints.set(item.line, { cond: item.cond || '' })
        }
      })
    }
    updateBreakpointDecorations()
  }

  function toggleBreakpointLine(line) {
    if (debugBreakpoints.has(line)) {
      debugBreakpoints.delete(line)
    } else {
      debugBreakpoints.set(line, { cond: '' })
    }
    updateBreakpointDecorations()
    return debugBreakpoints.has(line)
  }

  // 设置断点条件（不改变断点存在状态，仅更新条件）
  function setBreakpointCondition(line, cond) {
    if (debugBreakpoints.has(line)) {
      debugBreakpoints.set(line, { cond: cond || '' })
      updateBreakpointDecorations()
    }
  }

  function getBreakpointCondition(line) {
    return debugBreakpoints.get(line)?.cond || ''
  }

  function hasBreakpoint(line) {
    return debugBreakpoints.has(line)
  }

  function setCurrentLine(line) {
    debugCurrentLine = line || 0
    updateCurrentLineDecoration()
    if (debugCurrentLine > 0) {
      editor?.revealLineInCenter(debugCurrentLine)
    }
  }

  function clearDebugState() {
    // v0.9.13：只清当前执行行，不清用户断点（调试结束后断点应保留）
    debugCurrentLine = 0
    updateCurrentLineDecoration()
  }

  function clearCurrentLineOnly() {
    debugCurrentLine = 0
    updateCurrentLineDecoration()
  }

  // F9 切换断点（标准调试快捷键）
  editor.addCommand(monaco.KeyCode.F9, () => {
    const line = editor.getPosition().lineNumber
    toggleBreakpointLine(line)
    emit('toggle-breakpoint', line)
  })

  // ===== 代码片段 + 补全提示（关键字/类型/代码片段/支持库命令）=====
  // SNIPPETS / SUPPORT_ALIASES 已从 egKeywords.js 导入
  // 旧版本 SNIPPETS 里的 {} 与易语言风格冲突（转译器会自动加 Go 的 {}），已修正

  monaco.languages.registerCompletionItemProvider('egou', {
    triggerCharacters: ['.', '(', ' '],
    provideCompletionItems: async (model, position) => {
      const word = model.getWordUntilPosition(position)
      const range = {
        startLineNumber: position.lineNumber,
        endLineNumber: position.lineNumber,
        startColumn: word.startColumn,
        endColumn: word.endColumn
      }
      const suggestions = KEYWORDS.map(kw => ({
        label: kw,
        kind: monaco.languages.CompletionItemKind.Keyword,
        insertText: kw,
        detail: t('editor.keyword'),
        range
      }))
      TYPE_KEYWORDS.forEach(tk => suggestions.push({
        label: tk,
        kind: monaco.languages.CompletionItemKind.TypeParameter,
        insertText: tk,
        detail: t('editor.dataType'),
        range
      }))
      // 代码片段补全项
      Object.entries(SNIPPETS).forEach(([trigger, template]) => {
        suggestions.push({
          label: trigger + ' (' + t('editor.snippet') + ')',
          kind: monaco.languages.CompletionItemKind.Snippet,
          insertText: template,
          insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
          detail: t('editor.snippet'),
          range
        })
      })
      // 支持库命令补全（中文别名 → 显示签名）
      Object.entries(SUPPORT_ALIASES).forEach(([alias, sig]) => {
        suggestions.push({
          label: alias,
          kind: monaco.languages.CompletionItemKind.Function,
          insertText: alias + '(${1})',
          insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
          detail: t('editor.libCommand'),
          documentation: { value: '`' + sig + '`' },
          range
        })
      })
      // 用户函数补全：从当前文件符号中提取函数/方法
      try {
        const src = model.getValue()
        const resp = await IDEService.ListSymbols(src)
        if (resp && resp.symbols) {
          resp.symbols.forEach(sym => {
            if (sym.kind === 'function' || sym.kind === 'method') {
              let params = ''
              if (sym.params && sym.params.length > 0) {
                params = sym.params.map(p => p.name + ' ' + p.type).join(', ')
              }
              let insert = sym.name + '(' + params + ')'
              if (params) {
                insert = sym.name + '(${1:' + params + '})'
              }
              suggestions.push({
                label: sym.name,
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: insert,
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                detail: (sym.kind === 'method' ? t('editor.method') : t('editor.function')) + (sym.returnType ? ' → ' + sym.returnType : ''),
                documentation: { value: sym.returnType ? '返回: ' + sym.returnType : '' },
                range
              })
            } else if (sym.kind === 'type') {
              suggestions.push({
                label: sym.name,
                kind: monaco.languages.CompletionItemKind.Class,
                insertText: sym.name,
                detail: t('editor.type'),
                range
              })
            }
          })
        }
      } catch (e) {}
      // 项目级补全：如果有项目路径，添加项目中所有文件的函数
      if (props.projectPath) {
        try {
          const allResp = await IDEService.ListAllSymbols(props.projectPath)
          if (allResp && allResp.symbols) {
            // 去重
            const added = new Set()
            suggestions.forEach(s => added.add(s.label))
            allResp.symbols.forEach(sym => {
              if ((sym.kind === 'function' || sym.kind === 'method') && !added.has(sym.name)) {
                let params = ''
                if (sym.params && sym.params.length > 0) {
                  params = sym.params.map(p => p.name + ' ' + p.type).join(', ')
                }
                let insert = sym.name + '(' + params + ')'
                if (params) {
                  insert = sym.name + '(${1:' + params + '})'
                }
                suggestions.push({
                  label: sym.name,
                  kind: monaco.languages.CompletionItemKind.Function,
                  insertText: insert,
                  insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                  detail: (sym.kind === 'method' ? t('editor.method') : t('editor.function')) + (sym.returnType ? ' → ' + sym.returnType : ''),
                  documentation: { value: (sym.file ? '文件: ' + sym.file + '\n' : '') + (sym.returnType ? '返回: ' + sym.returnType : '') },
                  range
                })
              }
            })
          }
        } catch (e) {}
      }
      return { suggestions }
    }
  })

  // ===== 内容变化双向绑定 + 中文符号转换 + 实时诊断 =====
  let diagTimer = null
  function updateDiagnostics() {
    if (!editor) return
    const model = editor.getModel()
    if (!model) return
    const src = model.getValue()
    if (!src || src.trim().length === 0) {
      monaco.editor.setModelMarkers(model, 'eg-parser', [])
      return
    }
    IDEService.GetDiagnostics(src).then(resp => {
      if (!resp || !resp.diagnostics || resp.diagnostics.length === 0) {
        monaco.editor.setModelMarkers(model, 'eg-parser', [])
        return
      }
      const markers = resp.diagnostics.map(d => ({
        startLineNumber: d.line,
        startColumn: d.col,
        endLineNumber: d.endLine || d.line,
        endColumn: d.endCol || (d.col + 1),
        message: d.message,
        severity: d.severity === 'error' ? monaco.MarkerSeverity.Error
                 : d.severity === 'warning' ? monaco.MarkerSeverity.Warning
                 : monaco.MarkerSeverity.Info,
        source: d.source || 'eg-parser'
      }))
      monaco.editor.setModelMarkers(model, 'eg-parser', markers)
    }).catch(() => {
      // 后端调用失败，清空 markers 避免误报
      monaco.editor.setModelMarkers(model, 'eg-parser', [])
    })
  }

  editor.onDidChangeModelContent(() => {
    const v = editor.getValue()
    if (v !== props.modelValue) {
      emit('update:modelValue', v)
    }

    // 实时诊断（debounce 500ms，避免每次按键都调用后端）
    if (diagTimer) clearTimeout(diagTimer)
    diagTimer = setTimeout(updateDiagnostics, 500)

    // 中文符号自动转换
    if (props.autoConvertSymbols && !symbolsConverted) {
      const model = editor.getModel()
      if (!model) return
      const changes = []
      const lineCount = model.getLineCount()
      for (let i = 1; i <= lineCount; i++) {
        const line = model.getLineContent(i)
        let newLine = line
        for (const [cjk, ascii] of Object.entries(CJK_MAP)) {
          newLine = newLine.split(cjk).join(ascii)
        }
        if (newLine !== line) {
          changes.push({
            range: new monaco.Range(i, 1, i, line.length + 1),
            text: newLine
          })
        }
      }
      if (changes.length > 0) {
        symbolsConverted = true
        editor.executeEdits('auto-convert', changes)
        setTimeout(() => { symbolsConverted = false }, 100)
      }
    }
  })

  // 光标变化
  editor.onDidChangeCursorPosition((e) => {
    emit('cursor-change', {
      line: e.position.lineNumber,
      column: e.position.column
    })
  })

  // Ctrl+滚轮缩放
  editor.onDidChangeConfiguration((e) => {
    if (e.hasChanged(monaco.editor.EditorOption.fontSize)) {
      emit('font-size-change', editor.getOption(monaco.editor.EditorOption.fontSize))
    }
  })

  // ===== AST 驱动的语言特性（基于 IDEService 后端符号索引） =====
  // F12 跳转定义 / Shift+F12 查找引用 / F2 重命名 / Ctrl+Hover 悬停签名
  // 后端：internal/transpiler/{ast.go, parser.go, symbols.go} → IDEService.ListSymbols/FindDefinition/FindReferences

  // 1. 跳转定义（F12）
  //    优先调用后端 AST；同文件找不到再 emit 跨文件 .elib 搜索
  monaco.languages.registerDefinitionProvider('egou', {
    provideDefinition: async (model, position) => {
      const word = model.getWordAtPosition(position)
      if (!word || !word.word) return null
      try {
        const src = model.getValue()
        const def = await IDEService.FindDefinition(src, word.word)
        if (def && def.line > 0) {
          return {
            uri: model.uri,
            range: new monaco.Range(def.line, def.col || 1, def.line, (def.col || 1) + word.word.length)
          }
        }
      } catch (e) {
        // 后端调用失败，回退到正则模式
      }
      // 回退：正则匹配同文件函数/方法定义
      const lineCount = model.getLineCount()
      const reFunc = new RegExp('^\\s*函数\\s+' + word.word + '\\s*\\(')
      const reMethod = new RegExp('^\\s*方法\\s+' + word.word + '\\s*\\(')
      for (let i = 1; i <= lineCount; i++) {
        const line = model.getLineContent(i)
        if (reFunc.test(line) || reMethod.test(line)) {
          return { uri: model.uri, range: new monaco.Range(i, 1, i, 1) }
        }
      }
      // 同文件找不到，调后端跨文件查找（项目 .eg + 项目 libs + 全局 libs）
      if (props.projectPath) {
        try {
          const cross = await IDEService.FindDefCrossFile(props.projectPath, word.word)
          if (cross && cross.found && cross.file) {
            // 通过 emit 通知父组件打开文件并跳转
            emit('open-file-at', {
              file: cross.file,
              line: cross.line,
              col: cross.col || 1,
              word: word.word,
              source: cross.source,
              pkgName: cross.pkgName
            })
            // 返回当前文件位置（Monaco 不会自己打开别的文件，由父组件处理）
            return null
          }
        } catch (e) {
          // 后端跨文件查找失败，继续回退
        }
      }
      // 最终回退：通过 emit 通知父组件用旧机制查找 .elib
      emit('goto-def', { word: word.word, key: word.word, from: props.fileId })
      return null
    }
  })

  // 2. 查找引用（Shift+F12）
  //    优先同文件查找；找不到再调后端跨文件查找（项目 .eg + 项目 libs + 全局 libs）
  monaco.languages.registerReferenceProvider('egou', {
    provideReferences: async (model, position) => {
      const word = model.getWordAtPosition(position)
      if (!word || !word.word) return []
      // 同文件查找
      try {
        const src = model.getValue()
        const resp = await IDEService.FindReferences(src, word.word)
        if (resp && resp.refs && resp.refs.length > 0) {
          return resp.refs.map(r => ({
            uri: model.uri,
            range: new monaco.Range(r.line, r.col, r.line, r.col + r.length)
          }))
        }
      } catch (e) {
        // 同文件查找失败
      }
      // 跨文件查找
      if (props.projectPath) {
        try {
          const cross = await IDEService.FindRefsCrossFile(props.projectPath, word.word)
          if (cross && cross.refs && cross.refs.length > 0) {
            return cross.refs.map(r => ({
              uri: monaco.Uri.parse('file:///' + r.file.replace(/\\/g, '/')),
              range: new monaco.Range(r.line, r.col, r.line, r.col + r.length)
            }))
          }
        } catch (e) {
          // 跨文件查找失败
        }
      }
      return []
    }
  })

  // 3. 文档符号（Ctrl+Shift+O 大纲视图，也供右侧大纲面板使用）
  //    返回当前文件所有顶层符号
  monaco.languages.registerDocumentSymbolProvider('egou', {
    provideDocumentSymbols: async (model) => {
      try {
        const src = model.getValue()
        const resp = await IDEService.ListSymbols(src)
        if (!resp || !resp.symbols) return []
        return resp.symbols.map(s => {
          const kindMap = {
            function: monaco.languages.SymbolKind.Function,
            method: monaco.languages.SymbolKind.Method,
            type: monaco.languages.SymbolKind.Struct,
            const: monaco.languages.SymbolKind.Constant,
            var: monaco.languages.SymbolKind.Variable
          }
          const endLine = s.endLine > 0 ? s.endLine : s.line
          const endCol = s.endCol > 0 ? s.endCol : (s.col + s.name.length)
          return {
            name: s.name,
            detail: s.returnType ? ': ' + s.returnType : '',
            kind: kindMap[s.kind] || monaco.languages.SymbolKind.Variable,
            range: new monaco.Range(s.line, s.col, endLine, endCol),
            selectionRange: new monaco.Range(s.line, s.col, s.line, s.col + s.name.length)
          }
        })
      } catch (e) {
        return []
      }
    }
  })

  // 4. 重命名（F2）
  //    调用 FindReferences 拿到所有引用位置，返回 WorkspaceEdit
  monaco.languages.registerRenameProvider('egou', {
    provideRenameEdits: async (model, position, newName) => {
      const word = model.getWordAtPosition(position)
      if (!word || !word.word) return null
      if (!newName) return null
      try {
        const src = model.getValue()
        const resp = await IDEService.FindReferences(src, word.word)
        if (!resp || !resp.refs || resp.refs.length === 0) return null
        const edits = resp.refs.map(r => ({
          resource: model.uri,
          range: new monaco.Range(r.line, r.col, r.line, r.col + r.length),
          text: newName
        }))
        return { edits }
      } catch (e) {
        return null
      }
    }
  })

  // 5. 悬停提示（Ctrl+Hover 显示符号签名）
  //    支持库命令直接用前端 SUPPORT_ALIASES；用户函数调用后端 ListSymbols 匹配
  monaco.languages.registerHoverProvider('egou', {
    provideHover: async (model, position) => {
      const word = model.getWordAtPosition(position)
      if (!word || !word.word) return null
      const wd = word.word
      // 5.1 支持库命令：直接用前端别名表
      if (isSupportAlias(wd)) {
        const sig = SUPPORT_ALIASES[wd]
        return {
          range: new monaco.Range(position.lineNumber, word.startColumn, position.lineNumber, word.endColumn),
          contents: [
            { value: t('editor.libCommandMd', { name: wd }) },
            { value: '```eg\n' + sig + '\n```' }
          ]
        }
      }
      // 5.2 关键字提示
      if (isKeyword(wd)) {
        return {
          range: new monaco.Range(position.lineNumber, word.startColumn, position.lineNumber, word.endColumn),
          contents: [{ value: t('editor.keywordMd', { name: wd }) }]
        }
      }
      if (isTypeKeyword(wd)) {
        return {
          range: new monaco.Range(position.lineNumber, word.startColumn, position.lineNumber, word.endColumn),
          contents: [{ value: t('editor.dataTypeMd', { name: wd }) }]
        }
      }
      // 5.3 用户符号：调后端 ListSymbols 找到匹配
      try {
        const src = model.getValue()
        const resp = await IDEService.ListSymbols(src)
        if (resp && resp.symbols) {
          const sym = resp.symbols.find(s => s.name === wd)
          if (sym) {
            const kindLabel = { function: t('editor.function'), method: t('editor.method'), type: t('editor.type'), const: t('editor.const'), var: t('editor.var') }[sym.kind] || sym.kind
            let md = '**' + kindLabel + ' ' + sym.name + '**'
            if (sym.params && sym.params.length > 0) {
              md += '\n\n```eg\n' + t('editor.params') + '\n' + sym.params.map(p => '  ' + p.name + ': ' + p.type).join('\n') + '\n```'
            }
            if (sym.returnType) {
              md += '\n\n' + t('editor.returnType') + '`' + sym.returnType + '`'
            }
            if (sym.fields && sym.fields.length > 0) {
              md += '\n\n```eg\n' + t('editor.fields') + '\n' + sym.fields.map(f => '  ' + f.name + ': ' + f.type).join('\n') + '\n```'
            }
            return {
              range: new monaco.Range(position.lineNumber, word.startColumn, position.lineNumber, word.endColumn),
              contents: [{ value: md }]
            }
          }
        }
      } catch (e) {}
      return null
    }
  })

  // 6. 参数提示（输入函数调用时弹出参数签名）
  //    触发字符：`(` 和 `,`
  //    优先级：支持库命令 > 用户函数（来自后端 ListSymbols）
  monaco.languages.registerSignatureHelpProvider('egou', {
    signatureHelpTriggerCharacters: ['(', ','],
    signatureHelpRetriggerCharacters: [','],
    provideSignatureHelp: async (model, position) => {
      // 找到当前光标所在的函数调用：从光标向前找最近的 ( 之前的标识符
      const lineUp = position.lineNumber
      const col = position.column
      let triggerLine = lineUp
      let triggerCol = col - 1
      let depth = 0
      let funcName = ''
      // 向左扫描找未闭合的 (
      while (triggerLine >= 1) {
        const line = model.getLineContent(triggerLine)
        const runes = [...line]
        if (triggerLine === lineUp) {
          // 只扫描到 col-1
          runes.length = Math.min(runes.length, col - 1)
        }
        for (let i = runes.length - 1; i >= 0; i--) {
          const ch = runes[i]
          if (ch === ')') depth++
          else if (ch === '(') {
            if (depth === 0) {
              // 找到调用 ( ，再向左提取标识符
              let j = i - 1
              const identRe = /[\u4e00-\u9fa5A-Za-z0-9_]/
              while (j >= 0 && identRe.test(runes[j])) j--
              funcName = runes.slice(j + 1, i).join('')
              triggerLine = triggerLine
              triggerCol = i + 1
              break
            }
            depth--
          }
        }
        if (funcName) break
        triggerLine--
      }
      if (!funcName) return null

      // 计算当前参数位置（光标之前有多少个逗号）
      const argIdx = countCommasBefore(model, triggerLine, triggerCol, position.lineNumber, position.column)

      // 构造签名信息
      let label = ''
      let docStr = ''
      let params = []
      if (isSupportAlias(funcName)) {
        label = SUPPORT_ALIASES[funcName] || (funcName + '(...)')
        docStr = t('editor.libCommand')
        // 简单从签名提取参数：foo(a, b) -> ['a', 'b']
        const m = label.match(/\(([^)]*)\)/)
        if (m && m[1].trim()) {
          params = m[1].split(',').map(s => ({ label: s.trim() }))
        }
      } else {
        // 用户函数：调后端
        try {
          const src = model.getValue()
          const resp = await IDEService.ListSymbols(src)
          if (resp && resp.symbols) {
            const sym = resp.symbols.find(s => s.name === funcName && (s.kind === 'function' || s.kind === 'method'))
            if (sym && sym.params) {
              const paramList = sym.params.map(p => p.name + ': ' + p.type).join(', ')
              label = funcName + '(' + paramList + ')'
              if (sym.returnType) label += ': ' + sym.returnType
              docStr = sym.kind === 'method' ? t('editor.method') : t('editor.function')
              params = sym.params.map(p => ({ label: p.name + ': ' + p.type }))
            }
          }
        } catch (e) {
          return null
        }
      }
      if (!label) return null

      return {
        dispose: () => {},
        value: {
          signatures: [{
            label,
            documentation: docStr,
            parameters: params,
            activeParameter: Math.min(argIdx, Math.max(0, params.length - 1))
          }],
          activeSignature: 0,
          activeParameter: Math.min(argIdx, Math.max(0, params.length - 1))
        }
      }
    }
  })

  // countCommasBefore 计算 (triggerLine,triggerCol) 到 (curLine,curCol) 之间的逗号数（同层）
  function countCommasBefore(model, startLine, startCol, endLine, endCol) {
    let count = 0
    let depth = 0
    for (let l = startLine; l <= endLine; l++) {
      const line = model.getLineContent(l)
      const runes = [...line]
      let i0 = 0
      let i1 = runes.length
      if (l === startLine) i0 = startCol - 1
      if (l === endLine) i1 = endCol - 1
      for (let i = i0; i < i1; i++) {
        const ch = runes[i]
        if (ch === '(') depth++
        else if (ch === ')') depth--
        else if (ch === ',' && depth === 1) count++
      }
    }
    return count
  }

  // ===== 折叠状态持久化（按 fileId） =====
  // 监听折叠状态变化，存到 localStorage
  let foldSaveTimer = null
  editor.onDidChangeFoldRegions(() => {
    if (!props.fileId) return
    if (foldSaveTimer) clearTimeout(foldSaveTimer)
    foldSaveTimer = setTimeout(() => {
      if (!editor) return
      const model = editor.getModel()
      if (!model) return
      // 获取所有折叠区域
      const foldingRegions = editor.getContribution('editor.contrib.folding')?.getFoldingRanges()
      if (!foldingRegions) return
      // 只保存折叠状态（collapsed = true）
      const collapsed = []
      for (let i = 0; i < foldingRegions.length; i++) {
        const region = foldingRegions[i]
        if (region && region.isCollapsed) {
          collapsed.push({ start: region.startLineNumber, end: region.endLineNumber })
        }
      }
      localStorage.setItem('eg-folds-' + props.fileId, JSON.stringify(collapsed))
    }, 300)
  })

  // 恢复折叠状态（规约记忆：延迟 150ms 确保 folding model 就绪）
  setTimeout(() => {
    if (!editor || !props.fileId) return
    try {
      const saved = JSON.parse(localStorage.getItem('eg-folds-' + props.fileId) || '[]')
      if (!Array.isArray(saved) || saved.length === 0) return
      const foldingController = editor.getContribution('editor.contrib.folding')
      if (!foldingController) return
      saved.forEach(({ start, end }) => {
        if (start && end) {
          editor.trigger('restore-fold', 'fold', {
            selectionLines: Array.from({ length: end - start + 1 }, (_, i) => start + i)
          })
        }
      })
    } catch (e) {}
  }, 150)

  // 延迟强制布局
  setTimeout(() => {
    if (editor) editor.layout()
    // 初始加载时跑一次诊断
    updateDiagnostics()
  }, 200)

  // ResizeObserver 兜底
  resizeObserver = new ResizeObserver(() => {
    if (editor) editor.layout()
  })
  resizeObserver.observe(hostRef.value)

  // 保存引用供 defineExpose 使用
  editor._bookmarks = bookmarks
  editor._toggleBookmark = toggleBookmark
  editor._nextBookmark = nextBookmark
  editor._prevBookmark = prevBookmark
  editor._clearAllBookmarks = clearAllBookmarks
  // 调试器：断点 + 当前执行行（v0.9.12：这些函数定义在 onMounted 内，
  // 必须挂到 editor 上才能被 defineExpose 访问，否则 ReferenceError）
  editor._setBreakpoints = setBreakpoints
  editor._toggleBreakpointLine = toggleBreakpointLine
  editor._setCurrentLine = setCurrentLine
  editor._clearDebugState = clearDebugState
  editor._clearCurrentLineOnly = clearCurrentLineOnly
  editor._debugBreakpoints = debugBreakpoints
})

onUnmounted(() => {
  if (diagTimer) {
    clearTimeout(diagTimer)
    diagTimer = null
  }
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (editor) {
    editor.dispose()
    editor = null
  }
})

// 外部值变化时更新编辑器
watch(() => props.modelValue, (v) => {
  if (editor && editor.getValue() !== v) {
    editor.setValue(v || '')
    // 文件切换后重新诊断
    if (diagTimer) clearTimeout(diagTimer)
    diagTimer = setTimeout(updateDiagnostics, 300)
  }
})

// v0.9.13：切换文件时清除当前执行行高亮（当前行只属于被调试的文件）
// 断点不在此清除——由 App.vue 通过 setBreakpoints 按文件同步
watch(() => props.fileId, () => {
  editor?._clearCurrentLineOnly?.()
})

watch(() => props.isDark, () => {
  if (editor) monaco.editor.setTheme(resolveTheme())
})

watch(() => props.editorTheme, () => {
  if (editor) monaco.editor.setTheme(resolveTheme())
})

watch(() => props.fontSize, (v) => {
  if (editor) editor.updateOptions({ fontSize: v })
})

watch(() => props.fontFamily, (v) => {
  if (editor) editor.updateOptions({ fontFamily: v })
})

// 暴露给父组件的方法
defineExpose({
  gotoLine: (line) => {
    if (!editor) return
    editor.revealLineInCenter(line)
    editor.setPosition({ lineNumber: line, column: 1 })
    editor.focus()
  },
  getValue: () => editor ? editor.getValue() : '',
  setMarkers: (markers) => {
    if (!editor) return
    const model = editor.getModel()
    if (!model) return
    monaco.editor.setModelMarkers(model, 'egou', (markers || []).map(m => ({
      startLineNumber: m.line,
      endLineNumber: m.line,
      startColumn: 1,
      endColumn: 999,
      message: m.message,
      severity: monaco.MarkerSeverity.Error
    })))
  },
  // 书签系统
  toggleBookmark: () => editor?._toggleBookmark?.(),
  nextBookmark: () => editor?._nextBookmark?.(),
  prevBookmark: () => editor?._prevBookmark?.(),
  clearBookmarks: () => editor?._clearAllBookmarks?.(),
  // 调试器：断点 + 当前执行行（v0.9.12：通过 editor 引用，避免 ReferenceError）
  setBreakpoints: (bps) => editor?._setBreakpoints?.(bps),
  toggleBreakpointLine: (line) => editor?._toggleBreakpointLine?.(line),
  setBreakpointCondition: (line, cond) => editor?._setBreakpointCondition?.(line, cond),
  getBreakpointCondition: (line) => editor?._getBreakpointCondition?.(line) || '',
  hasBreakpoint: (line) => editor?._hasBreakpoint?.(line) || false,
  setCurrentLine: (line) => editor?._setCurrentLine?.(line),
  clearDebugState: () => editor?._clearDebugState?.(),
  getBreakpoints: () => [...(editor?._debugBreakpoints?.entries?.() || [])].map(([line, info]) => ({ line, cond: info.cond || '' })),
  getCurrentLine: () => editor ? editor.getPosition().lineNumber : 0,
  flashLine: () => {},
  runAction: (id) => {
    if (!editor) return
    const action = editor.getAction(id)
    if (action) { editor.focus(); action.run() }
  },
  toggleFindHistory: () => {},
  getUserSnippets: () => [],
  addUserSnippet: () => {},
  removeUserSnippet: () => {},
  importUserSnippets: () => {}
})
</script>

<style scoped>
.editor-host {
  width: 100%;
  height: 100%;
  min-height: 200px;
  overflow: hidden;
}
</style>

<!-- 全局样式：书签 glyph 标记（scoped 不影响 Monaco 内部 DOM） -->
<style>
.eg-bookmark-glyph {
  background: var(--accent-color, #007acc);
  border-radius: 3px;
  margin-left: 4px;
  width: 8px !important;
  height: 8px !important;
  margin-top: 6px;
  cursor: pointer;
}
.eg-bookmark-glyph:hover {
  background: var(--accent-hover, #1f8ad2);
  transform: scale(1.2);
}
/* 断点 glyph（红色实心圆） */
.eg-breakpoint-glyph {
  background: var(--error-color, #f14c4c);
  border-radius: 50%;
  margin-left: 3px;
  width: 10px !important;
  height: 10px !important;
  margin-top: 5px;
  cursor: pointer;
  box-shadow: 0 0 3px rgba(241, 76, 76, 0.5);
}
.eg-breakpoint-glyph:hover {
  transform: scale(1.15);
}
/* 条件断点 glyph（红色菱形 + "?" 标记，区别于普通红圆） */
.eg-breakpoint-conditional-glyph {
  background: var(--error-color, #f14c4c);
  border-radius: 3px;
  margin-left: 2px;
  width: 11px !important;
  height: 11px !important;
  margin-top: 4px;
  cursor: pointer;
  box-shadow: 0 0 3px rgba(241, 76, 76, 0.5);
  position: relative;
}
.eg-breakpoint-conditional-glyph:hover {
  transform: scale(1.15);
}
.eg-breakpoint-conditional-glyph::after {
  content: '?';
  position: absolute;
  top: -3px;
  left: 50%;
  transform: translateX(-50%);
  color: #fff;
  font-size: 9px;
  font-weight: 700;
  line-height: 1;
}
/* 调试当前执行行高亮（VS Code 风格：黄色背景 + 左侧箭头标记） */
.eg-debug-current-line {
  background: rgba(255, 213, 79, 0.15);
  border-left: 3px solid #ffd54f;
}
.eg-debug-current-glyph {
  width: 12px !important;
  height: 12px !important;
  margin-left: 2px;
  margin-top: 4px;
  background: transparent;
  position: relative;
}
.eg-debug-current-glyph::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  width: 0;
  height: 0;
  border-left: 10px solid #ffd54f;
  border-top: 6px solid transparent;
  border-bottom: 6px solid transparent;
  filter: drop-shadow(0 0 2px rgba(255, 213, 79, 0.6));
}
</style>
