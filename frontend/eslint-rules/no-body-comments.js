const DIRECTIVE =
  /^\s*(eslint-|@ts-|istanbul|c8 |prettier-ignore|@vitest-|vitest-|v8 ignore|#__PURE__|@__PURE__|<reference)/
const WHY_WITH_REASON = /^\s*why:\s*\S/
const TAG_ONLY_LINE = /^\s*(\*\s*)?(@\w+.*)?$/
const DOC_CAP = 10

const BODY_MESSAGE =
  'Comment inside a function body: rename, extract a helper or name the constant instead. ' +
  'A fact no name can carry may use "why: <reason>".'
const DOC_MESSAGE =
  'Function doc comment is longer than the cap: split the function or trim the doc to its contract.'

function lines(comment) {
  return comment.value.split('\n').map((line) => line.replace(/^\s*\*+\s?/, '').trim())
}

function isAllowedInBody(comment) {
  const text = comment.value
  if (DIRECTIVE.test(text) || WHY_WITH_REASON.test(text.replace(/^\s*\*?/, ''))) return true
  const isDocBlock = comment.type === 'Block' && text.startsWith('*')
  return isDocBlock && text.split('\n').every((line) => TAG_ONLY_LINE.test(line))
}

function proseLines(comment) {
  return lines(comment).filter((line) => line !== '' && !line.startsWith('@')).length
}

function docOwner(node) {
  const parent = node.parent
  const exported = parent && /^Export(Named|Default)Declaration$/.test(parent.type)
  return exported ? parent : node
}

export default {
  meta: { type: 'suggestion', schema: [] },
  create(context) {
    const source = context.sourceCode
    const reported = new Set()

    function report(comment, message) {
      if (reported.has(comment)) return
      reported.add(comment)
      context.report({ loc: comment.loc, message })
    }

    function checkBody(node) {
      if (node.body.type !== 'BlockStatement') return
      for (const comment of source.getAllComments()) {
        const inside =
          comment.range[0] > node.body.range[0] && comment.range[1] < node.body.range[1]
        if (inside && !isAllowedInBody(comment)) report(comment, BODY_MESSAGE)
      }
    }

    function checkDoc(node) {
      const before = source.getCommentsBefore(docOwner(node))
      const doc = before[before.length - 1]
      if (doc && proseLines(doc) > DOC_CAP) report(doc, DOC_MESSAGE)
    }

    return {
      FunctionDeclaration(node) {
        checkDoc(node)
        checkBody(node)
      },
      FunctionExpression: checkBody,
      ArrowFunctionExpression: checkBody,
    }
  },
}
