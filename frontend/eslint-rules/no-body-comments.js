const DIRECTIVE =
  /^\s*(eslint-|@ts-|istanbul|c8 |prettier-ignore|@vitest-|vitest-|v8 ignore|coverage-ignore:|#__PURE__|@__PURE__|<reference)/
const WHY_WITH_REASON = /^\s*why:\s*\S/
const TYPE_TAG_LINE = /^\s*(\*\s*)?(@(type|satisfies|typedef|template)\b.*)?$/
const DOC_CAP = 10

const BODY_MESSAGE =
  'Comment inside a function body: rename, extract a helper or name the constant instead. ' +
  'A fact no name can carry may use "why: <reason>".'
const DOC_MESSAGE =
  'Function doc comment is longer than the cap: split the function or trim the doc to its contract.'

function proseLines(comment) {
  return comment.value
    .split('\n')
    .map((line) => line.replace(/^\s*\*+\s?/, '').trim())
    .filter((line) => line !== '').length
}

function isDirective(comment) {
  return DIRECTIVE.test(comment.value)
}

function isTypeTagBlock(comment) {
  const isDocBlock = comment.type === 'Block' && comment.value.startsWith('*')
  return isDocBlock && comment.value.split('\n').every((line) => TYPE_TAG_LINE.test(line))
}

function startsWithWhy(comment) {
  return WHY_WITH_REASON.test(comment.value.replace(/^\s*\*?/, ''))
}

function groupAdjacent(comments) {
  const groups = []
  for (const comment of comments) {
    const last = groups[groups.length - 1]
    const previous = last && last[last.length - 1]
    const adjacent =
      previous &&
      previous.type === 'Line' &&
      comment.type === 'Line' &&
      comment.loc.start.line === previous.loc.end.line + 1
    if (adjacent) last.push(comment)
    else groups.push([comment])
  }
  return groups
}

function isAllowedGroup(group) {
  if (startsWithWhy(group[0])) return true
  return group.every((comment) => isDirective(comment) || isTypeTagBlock(comment))
}

function exportWrapper(node) {
  const parent = node.parent
  return parent && /^Export(Named|Default)Declaration$/.test(parent.type) ? parent : node
}

function docOwner(node) {
  if (node.type === 'FunctionDeclaration') return exportWrapper(node)
  const parent = node.parent
  if (!parent) return null
  if (parent.type === 'VariableDeclarator' && parent.parent.type === 'VariableDeclaration') {
    return exportWrapper(parent.parent)
  }
  const isMember = /^(MethodDefinition|Property|PropertyDefinition)$/.test(parent.type)
  return isMember && parent.value === node ? parent : null
}

function docComments(source, owner) {
  const before = source.getCommentsBefore(owner)
  const doc = []
  let anchor = owner.loc.start.line
  for (let i = before.length - 1; i >= 0; i--) {
    const comment = before[i]
    if (comment.loc.end.line !== anchor - 1) break
    anchor = comment.loc.start.line
    if (!isDirective(comment)) doc.push(comment)
    if (comment.type === 'Block' && !isDirective(comment)) break
  }
  return doc
}

export default {
  meta: { type: 'suggestion', schema: [] },
  create(context) {
    const source = context.sourceCode

    function checkBody(node) {
      if (node.body.type !== 'BlockStatement') return
      const inside = source
        .getAllComments()
        .filter((c) => c.range[0] > node.body.range[0] && c.range[1] < node.body.range[1])
      for (const group of groupAdjacent(inside)) {
        if (!isAllowedGroup(group)) context.report({ loc: group[0].loc, message: BODY_MESSAGE })
      }
    }

    function checkDoc(node) {
      const owner = docOwner(node)
      if (!owner) return
      const doc = docComments(source, owner)
      const lines = doc.reduce((sum, comment) => sum + proseLines(comment), 0)
      if (lines > DOC_CAP) {
        context.report({ loc: doc[doc.length - 1].loc, message: DOC_MESSAGE })
      }
    }

    const checkFunction = (node) => {
      checkDoc(node)
      checkBody(node)
    }
    return {
      FunctionDeclaration: checkFunction,
      FunctionExpression: checkFunction,
      ArrowFunctionExpression: checkFunction,
    }
  },
}
