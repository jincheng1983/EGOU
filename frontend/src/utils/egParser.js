// 简易 .eg 源码解析器：把文本拆分为全局区段、全局变量、常量和函数/方法区段，
// 供项目树展示、Monaco 自动补全和跳转到函数定义使用。

const FUNC_START_RE = /^(函数|方法)\s+(.+)$/
const METHOD_START_RE = /^方法\s+\(\s*(\S+)\s+(\S+)\s*\)\s*([^\s(]+)\s*(\(.*\))?\s*(\S*)$/
const FUNC_SIG_RE = /^函数\s+([^\s(]+)\s*(\(.*\))?\s*(\S*)$/

function parseVarDecl(decl) {
  // "姓名, 文本型" 或 "姓名 文本型"
  const parts = decl.split(',').map(s => s.trim()).filter(Boolean)
  if (parts.length >= 2) {
    return { name: parts[0], type: parts[1] }
  }
  const spaceParts = decl.trim().split(/\s+/)
  if (spaceParts.length >= 2) {
    return { name: spaceParts[0], type: spaceParts[1] }
  }
  return { name: decl.trim(), type: '' }
}

function parseParams(paramsText) {
  if (!paramsText) return []
  const inner = paramsText.trim()
  if (inner.startsWith('(') && inner.endsWith(')')) {
    paramsText = inner.slice(1, -1)
  }
  // "参数 a 整数型, 参数 b 整数型" 或 "参数 a, 整数型, 参数 b, 整数型"
  const rawItems = paramsText.split(',').map(s => s.trim()).filter(Boolean)
  const params = []
  let i = 0
  while (i < rawItems.length) {
    const item = rawItems[i]
    if (item.startsWith('参数 ')) {
      const rest = item.slice(3).trim()
      const fields = rest.split(/\s+/)
      if (fields.length >= 2) {
        params.push({ name: fields[0], type: fields[1] })
        i++
      } else if (fields.length === 1 && i + 1 < rawItems.length) {
        // "参数 a, 整数型" 形式：名字在逗号前一段，类型在下一段
        params.push({ name: fields[0], type: rawItems[i + 1] })
        i += 2
      } else {
        params.push({ name: rest, type: '' })
        i++
      }
    } else {
      params.push(parseVarDecl(item))
      i++
    }
  }
  return params
}

// 解析 变量(...) 或 常量(...) 块中的声明行
function parseBlockDecls(lines, startIdx, blockKind) {
  const decls = []
  let i = startIdx + 1
  while (i < lines.length) {
    const trimmed = lines[i].trim()
    if (trimmed === ')') break
    if (trimmed && !trimmed.startsWith('//')) {
      const v = parseVarDecl(trimmed)
      if (v.name) {
        decls.push({ ...v, kind: blockKind, line: i })
      }
    }
    i++
  }
  return { decls, endIdx: i }
}

export function parseEg(source) {
  const lines = source.split('\n')
  const globalLines = []
  const functions = []
  const globalVars = []
  const constants = []

  let i = 0
  while (i < lines.length) {
    const raw = lines[i]
    const trimmed = raw.trim()
    const match = trimmed.match(FUNC_START_RE)

    if (match) {
      const kind = match[1]
      let name, receiverType, receiverName, paramsText, returnType

      if (kind === '方法') {
        const m = trimmed.match(METHOD_START_RE)
        if (m) {
          receiverName = m[1]
          receiverType = m[2]
          name = m[3] || ''
          paramsText = m[4] || ''
          returnType = m[5] || ''
        }
      } else {
        const m = trimmed.match(FUNC_SIG_RE)
        if (m) {
          name = m[1]
          paramsText = m[2] || ''
          returnType = m[3] || ''
        }
      }

      const bodyLines = []
      let j = i + 1
      while (j < lines.length && lines[j].trim() !== '结束函数' && lines[j].trim() !== '结束方法') {
        bodyLines.push(lines[j])
        j++
      }
      const endLine = j < lines.length ? lines[j] : ''
      const endKind = endLine.trim() === '结束方法' ? '方法' : '函数'

      // 去掉函数体首行缩进，便于编辑器跳转时正确定位行号
      const indentMatch = bodyLines[0]?.match(/^(\s*)/)
      const baseIndent = indentMatch ? indentMatch[1] : ''

      functions.push({
        kind,
        name,
        receiverName,
        receiverType,
        params: parseParams(paramsText),
        returnType,
        startLine: i,
        endLine: j,
        endKind,
        bodyIndent: baseIndent
      })

      i = j + 1
      continue
    }

    // 全局变量块（顶层 变量( ... ) 块，单行 变量=xxx 在函数体内已跳过）
    if (trimmed === '变量 (' || trimmed === '变量(') {
      const { decls, endIdx } = parseBlockDecls(lines, i, '变量')
      globalVars.push(...decls)
      i = endIdx + 1
      continue
    }

    // 常量块（顶层 常量( ... ) 块）
    if (trimmed === '常量 (' || trimmed === '常量(') {
      const { decls, endIdx } = parseBlockDecls(lines, i, '常量')
      constants.push(...decls)
      i = endIdx + 1
      continue
    }

    globalLines.push(raw)
    i++
  }

  return {
    global: globalLines.join('\n'),
    functions,
    globalVars,
    constants
  }
}
