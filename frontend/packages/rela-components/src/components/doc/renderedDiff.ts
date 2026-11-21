import { Marked, type Token, type Tokens } from 'marked'

/**
 * A rendered, block-aware diff of two markdown documents.
 *
 * Both sides are lexed into top-level blocks (paragraph, heading, list, ...).
 * Blocks are matched by their source text; a run of removed blocks followed by
 * added blocks is paired in order, and a paired text block is diffed word by
 * word. The result is a list of segments whose `html` is ready for `v-html`.
 *
 * The HTML is safe to inject: raw HTML in the markdown is escaped, and link and
 * image URLs are limited to http(s), mailto and relative targets.
 */

export type BlockOp = 'equal' | 'add' | 'del' | 'change'

export interface DiffBlock {
  op: BlockOp
  html: string
}

export type DiffSegment =
  | { kind: 'block'; block: DiffBlock }
  | { kind: 'collapsed'; blocks: DiffBlock[] }

// Private-use characters survive marked untouched and never occur in prose.
const INS_OPEN = ''
const INS_CLOSE = ''
const DEL_OPEN = ''
const DEL_CLOSE = ''

const SAFE_URL = /^(?:https?:|mailto:|\/|#|\.{0,2}\/|[^:]*$)/i

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

const marked = new Marked({
  gfm: true,
  renderer: {
    html: ({ text }: Tokens.HTML | Tokens.Tag) => escapeHtml(text),
    link({ href, title, tokens }: Tokens.Link) {
      const text = this.parser.parseInline(tokens)
      if (!SAFE_URL.test(href)) return text
      const t = title ? ` title="${escapeHtml(title)}"` : ''
      return `<a href="${escapeHtml(href)}"${t} rel="noopener noreferrer">${text}</a>`
    },
    image({ href, text }: Tokens.Image) {
      if (!SAFE_URL.test(href)) return escapeHtml(text)
      return `<img src="${escapeHtml(href)}" alt="${escapeHtml(text)}">`
    },
  },
})

/** Wraps the private-use markers into real tags, after marked has escaped the text. */
function markersToTags(html: string): string {
  return html
    .split(INS_OPEN).join('<ins class="rl-rd__ins">')
    .split(INS_CLOSE).join('</ins>')
    .split(DEL_OPEN).join('<del class="rl-rd__del">')
    .split(DEL_CLOSE).join('</del>')
}

function blocksOf(markdown: string): Token[] {
  return marked.lexer(markdown).filter((t) => t.type !== 'space')
}

function renderBlock(token: Token): string {
  return markersToTags(marked.parser([token]) as string)
}

type EditOp = { op: 'equal' | 'del' | 'add'; i: number; j: number }

/** Above this many table cells the quadratic LCS is skipped (a UI freeze guard). */
const MAX_LCS_CELLS = 2_000_000

/**
 * Edit script between two arrays of keys. The common prefix and suffix are
 * trimmed first; if what is left is too large for the quadratic table, the
 * middle is reported as one removal followed by one addition.
 */
function lcsOps(a: string[], b: string[]): EditOp[] {
  let lo = 0
  while (lo < a.length && lo < b.length && a[lo] === b[lo]) lo++
  let hiA = a.length
  let hiB = b.length
  while (hiA > lo && hiB > lo && a[hiA - 1] === b[hiB - 1]) {
    hiA--
    hiB--
  }
  const out: EditOp[] = []
  for (let k = 0; k < lo; k++) out.push({ op: 'equal', i: k, j: k })
  const n = hiA - lo
  const m = hiB - lo
  if (n * m > MAX_LCS_CELLS) {
    for (let i = lo; i < hiA; i++) out.push({ op: 'del', i, j: lo })
    for (let j = lo; j < hiB; j++) out.push({ op: 'add', i: hiA, j })
  } else {
    const w = m + 1
    const dp = new Int32Array((n + 1) * w)
    for (let i = n - 1; i >= 0; i--) {
      for (let j = m - 1; j >= 0; j--) {
        dp[i * w + j] =
          a[lo + i] === b[lo + j] ? dp[(i + 1) * w + j + 1] + 1 : Math.max(dp[(i + 1) * w + j], dp[i * w + j + 1])
      }
    }
    let i = 0
    let j = 0
    while (i < n && j < m) {
      if (a[lo + i] === b[lo + j]) out.push({ op: 'equal', i: lo + i++, j: lo + j++ })
      else if (dp[(i + 1) * w + j] >= dp[i * w + j + 1]) out.push({ op: 'del', i: lo + i++, j: lo + j })
      else out.push({ op: 'add', i: lo + i, j: lo + j++ })
    }
    while (i < n) out.push({ op: 'del', i: lo + i++, j: lo + j })
    while (j < m) out.push({ op: 'add', i: lo + i, j: lo + j++ })
  }
  for (let k = 0; k < a.length - hiA; k++) out.push({ op: 'equal', i: hiA + k, j: hiB + k })
  return out
}

const WORD_DIFFABLE = new Set(['paragraph', 'heading', 'list', 'blockquote', 'table'])

function tokenize(s: string): string[] {
  return s.match(/\s+|\S+/g) ?? []
}

/** Backslash-escapes markdown punctuation so removed text shows exactly as it was written. */
function escapeMd(w: string): string {
  return w.replace(/[*_`~[\]()<>\\|]/g, '\\$&')
}

const LINE_PREFIX = /^(?:\s*(?:[-*+]|\d+[.)]|>)\s+|#{1,6}\s+)*/

/** Applies `wrap` to the content of one line, leaving list, quote, heading and table syntax outside. */
function wrapLine(line: string, wrap: (text: string) => string): string {
  if (line.trimStart().startsWith('|')) {
    return line
      .split('|')
      .map((cell, i, all) => (i === 0 || i === all.length - 1 || !cell.trim() ? cell : ` ${wrap(cell.trim())} `))
      .join('|')
  }
  const prefix = LINE_PREFIX.exec(line)?.[0] ?? ''
  const rest = line.slice(prefix.length)
  return rest.trim() ? prefix + wrap(rest) : line
}

/** Word diff of two lines, as one markdown line with change markers. */
function wordDiffLine(before: string, after: string): string {
  const prefix = LINE_PREFIX.exec(after)?.[0] ?? ''
  const a = tokenize(before.slice(LINE_PREFIX.exec(before)?.[0].length ?? 0))
  const b = tokenize(after.slice(prefix.length))
  let out = prefix
  let mode: 'equal' | 'del' | 'add' = 'equal'
  // Trailing whitespace stays outside the highlight.
  const close = () => {
    const tag = mode === 'add' ? INS_CLOSE : mode === 'del' ? DEL_CLOSE : ''
    if (!tag) return
    let end = out.length
    while (end > 0 && /\s/.test(out[end - 1])) end--
    out = out.slice(0, end) + tag + out.slice(end)
  }
  for (const o of lcsOps(a, b)) {
    const word = o.op === 'add' ? b[o.j] : a[o.i]
    const isSpace = /^\s+$/.test(word)
    // Whitespace between two changed words stays inside the highlight.
    if (o.op !== mode && !(isSpace && o.op === 'equal' && mode !== 'equal')) {
      close()
      mode = o.op
      if (mode === 'add') out += INS_OPEN
      if (mode === 'del') out += DEL_OPEN
    }
    out += o.op === 'del' ? escapeMd(word) : word
  }
  close()
  return out
}

/** Merges a diff of two blocks' source into one markdown string with markers: lines first, then words. */
function wordDiffMarkdown(before: string, after: string): string {
  const a = before.trimEnd().split('\n')
  const b = after.trimEnd().split('\n')
  const ops = lcsOps(a, b)
  const out: string[] = []
  let k = 0
  while (k < ops.length) {
    if (ops[k].op === 'equal') {
      out.push(b[ops[k++].j])
      continue
    }
    const dels: string[] = []
    const adds: string[] = []
    while (k < ops.length && ops[k].op !== 'equal') {
      if (ops[k].op === 'del') dels.push(a[ops[k].i])
      else adds.push(b[ops[k].j])
      k++
    }
    const paired = Math.min(dels.length, adds.length)
    for (let p = 0; p < paired; p++) out.push(wordDiffLine(dels[p], adds[p]))
    for (let p = paired; p < dels.length; p++) {
      out.push(wrapLine(dels[p], (t) => DEL_OPEN + escapeMd(t) + DEL_CLOSE))
    }
    for (let p = paired; p < adds.length; p++) {
      out.push(wrapLine(adds[p], (t) => INS_OPEN + t + INS_CLOSE))
    }
  }
  return out.join('\n')
}

/** Markdown whose inline syntax carries attributes or code, where change markers would corrupt the markup. */
const MARKER_UNSAFE = /\]\(|\]\[|^\s*\[[^\]]+\]:|https?:\/\/|www\.|<[^>\s]+>|`|\\\|/m

function columns(line: string): number {
  return line.split('|').length
}

/** Whether two paired blocks can be word-diffed without breaking their markup. */
function canWordDiff(before: Token, after: Token): boolean {
  if (before.type !== after.type || !WORD_DIFFABLE.has(after.type)) return false
  if (MARKER_UNSAFE.test(before.raw) || MARKER_UNSAFE.test(after.raw)) return false
  if (after.type === 'table') {
    const x = before.raw.trimEnd().split('\n')
    const y = after.raw.trimEnd().split('\n')
    if (columns(x[0]) !== columns(y[0]) || x[1]?.trim() !== y[1]?.trim()) return false
  }
  return similar(before.raw, after.raw)
}

/** Blocks with little in common are a removal plus an addition, not an edit. */
function similar(a: string, b: string): boolean {
  const x = new Set(a.split(/\s+/).filter(Boolean))
  const y = new Set(b.split(/\s+/).filter(Boolean))
  let shared = 0
  for (const w of x) if (y.has(w)) shared++
  const union = x.size + y.size - shared
  return union === 0 || shared / union >= 0.3
}

/** The word-diffed block, or null when nothing visible changed (a hard break, a heading level). */
function renderChange(before: Token, after: Token): string | null {
  const merged = wordDiffMarkdown(before.raw, after.raw)
  if (!/[\uE000-\uE003]/.test(merged)) return null
  return markersToTags(marked.parse(merged, { async: false }) as string)
}

/** Diffs two markdown documents into renderable blocks, in document order. */
export function diffBlocks(rawBefore: string, rawAfter: string): DiffBlock[] {
  // The markers are private-use characters; text that already holds them must not fake a change.
  const before = rawBefore.replace(/[\uE000-\uE003]/g, '')
  const after = rawAfter.replace(/[\uE000-\uE003]/g, '')
  const a = blocksOf(before)
  const b = blocksOf(after)
  const ops = lcsOps(
    a.map((t) => t.raw.trim()),
    b.map((t) => t.raw.trim()),
  )
  const blocks: DiffBlock[] = []
  let k = 0
  while (k < ops.length) {
    if (ops[k].op === 'equal') {
      blocks.push({ op: 'equal', html: renderBlock(b[ops[k].j]) })
      k++
      continue
    }
    const dels: Token[] = []
    const adds: Token[] = []
    while (k < ops.length && ops[k].op !== 'equal') {
      if (ops[k].op === 'del') dels.push(a[ops[k].i])
      else adds.push(b[ops[k].j])
      k++
    }
    const paired = Math.min(dels.length, adds.length)
    for (let p = 0; p < paired; p++) {
      const html = canWordDiff(dels[p], adds[p]) ? renderChange(dels[p], adds[p]) : null
      if (html !== null) blocks.push({ op: 'change', html })
      else {
        blocks.push({ op: 'del', html: renderBlock(dels[p]) })
        blocks.push({ op: 'add', html: renderBlock(adds[p]) })
      }
    }
    for (let q = paired; q < dels.length; q++) blocks.push({ op: 'del', html: renderBlock(dels[q]) })
    for (let q = paired; q < adds.length; q++) blocks.push({ op: 'add', html: renderBlock(adds[q]) })
  }
  return blocks
}

/** Whether the two documents differ in anything the diff would show. */
export function hasChanges(before: string, after: string): boolean {
  return diffBlocks(before, after).some((b) => b.op !== 'equal')
}

/**
 * Folds runs of unchanged blocks that are longer than `2 * context` into one
 * collapsed segment, keeping `context` blocks next to every change.
 */
export function collapseUnchanged(blocks: DiffBlock[], context = 1): DiffSegment[] {
  const keep = new Array<boolean>(blocks.length).fill(false)
  blocks.forEach((b, i) => {
    if (b.op === 'equal') return
    for (let d = -context; d <= context; d++) {
      if (i + d >= 0 && i + d < blocks.length) keep[i + d] = true
    }
  })
  const segments: DiffSegment[] = []
  let run: DiffBlock[] = []
  const flush = () => {
    if (run.length === 1) segments.push({ kind: 'block', block: run[0] })
    else if (run.length > 1) segments.push({ kind: 'collapsed', blocks: run })
    run = []
  }
  blocks.forEach((b, i) => {
    if (keep[i]) {
      flush()
      segments.push({ kind: 'block', block: b })
    } else run.push(b)
  })
  flush()
  return segments
}
