/**
 * Diffing for the track-changes mockup.
 *
 * The mockup's model is the one proposed for rela: the author edits the whole
 * body, and on save the difference between the two markdown texts becomes a
 * list of suggestions. Lines are aligned first, so an untouched paragraph never
 * produces a change. A changed line is then diffed word by word, keeping its
 * markdown prefix (`## `, `- `) out of the comparison, so a reworded list item
 * stays a list item.
 */

export type ChangeKind = 'inserted' | 'deleted' | 'replaced' | 'formatted'

export interface Change {
  id: string
  kind: ChangeKind
  /** The text removed, as markdown. Empty for an insertion. */
  del: string
  /** The text added, as markdown. Empty for a deletion. */
  ins: string
  /** The change adds or removes a whole line, such as a list item. */
  wholeLine: boolean
}

export type Seg = { text: string } | { change: string; del: string; ins: string }

export type DiffLine =
  | { type: 'same'; text: string }
  | { type: 'blank' }
  | { type: 'whole'; change: string; op: 'ins' | 'del'; text: string }
  | { type: 'mod'; prefix: string; segs: Seg[] }

export interface Diff {
  lines: DiffLine[]
  changes: Change[]
}

type Op<T> = { op: 'eq'; a: T; b: T } | { op: 'del'; a: T } | { op: 'ins'; b: T }

/** Longest-common-subsequence alignment. The documents are small enough for O(n·m). */
function align<T>(a: T[], b: T[]): Op<T>[] {
  const n = a.length
  const m = b.length
  const table: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      table[i][j] =
        a[i] === b[j] ? table[i + 1][j + 1] + 1 : Math.max(table[i + 1][j], table[i][j + 1])
    }
  }
  const out: Op<T>[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ op: 'eq', a: a[i++], b: b[j++] })
    } else if (table[i + 1][j] >= table[i][j + 1]) {
      out.push({ op: 'del', a: a[i++] })
    } else {
      out.push({ op: 'ins', b: b[j++] })
    }
  }
  while (i < n) out.push({ op: 'del', a: a[i++] })
  while (j < m) out.push({ op: 'ins', b: b[j++] })
  return out
}

const PREFIX = /^(#{1,6}\s+|\s*(?:[-*+]|\d+\.)\s+(?:\[[ xX]\]\s+)?|>\s?)/

export function splitPrefix(line: string): [string, string] {
  const match = PREFIX.exec(line)
  const prefix = match ? match[0] : ''
  return [prefix, line.slice(prefix.length)]
}

/** Formatted spans stay whole, so making two words bold is one change. */
const TOKEN = /\*\*[^*]+\*\*|_[^_]+_|`[^`]+`|\[[^\]]*\]\([^)]*\)|\s+|[^\s*_`[]+|./g

export function stripMarkers(text: string): string {
  return text.replace(/\*\*|`|(^|\s)_|_(\s|$)/g, '$1$2').replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
}

function kindOf(del: string, ins: string): ChangeKind {
  if (!del) return 'inserted'
  if (!ins) return 'deleted'
  return stripMarkers(del).trim() === stripMarkers(ins).trim() ? 'formatted' : 'replaced'
}

function lines(text: string): string[] {
  const out = text
    .replace(/\r\n/g, '\n')
    .split('\n')
    .map((l) => l.trimEnd())
  while (out.length && out[out.length - 1] === '') out.pop()
  return out
}

export function diffMarkdown(base: string, edited: string, idPrefix = 'c'): Diff {
  const result: DiffLine[] = []
  const changes: Change[] = []
  let next = 1
  const newId = () => `${idPrefix}${next++}`

  const whole = (op: 'ins' | 'del', text: string) => {
    if (text === '') {
      result.push({ type: 'blank' })
      return
    }
    const id = newId()
    changes.push({
      id,
      kind: op === 'ins' ? 'inserted' : 'deleted',
      del: op === 'del' ? text : '',
      ins: op === 'ins' ? text : '',
      wholeLine: true,
    })
    result.push({ type: 'whole', change: id, op, text })
  }

  const modified = (oldLine: string, newLine: string): boolean => {
    const [oldPrefix, oldBody] = splitPrefix(oldLine)
    const [newPrefix, newBody] = splitPrefix(newLine)
    if (oldPrefix !== newPrefix) return false
    const ops = align(oldBody.match(TOKEN) ?? [], newBody.match(TOKEN) ?? [])
    if (!ops.some((o) => o.op === 'eq' && o.a.trim() !== '')) return false

    // Merge hunks separated only by a short word, so "The setup script
    // installs" to "Run `just setup` to install" reads as one change rather
    // than four.
    for (let i = 1; i < ops.length - 1; i++) {
      const o = ops[i]
      if (o.op !== 'eq' || ops[i - 1].op === 'eq') continue
      let j = i
      while (j < ops.length && ops[j].op === 'eq') j++
      if (j === ops.length) break
      const words = ops.slice(i, j).filter((e) => e.op === 'eq' && e.a.trim() !== '')
      if (words.length > 1 || words.some((e) => e.op === 'eq' && e.a.length > 4)) continue
      const bridged: Op<string>[] = []
      for (const e of ops.slice(i, j)) {
        if (e.op === 'eq') bridged.push({ op: 'del', a: e.a }, { op: 'ins', b: e.b })
      }
      ops.splice(i, j - i, ...bridged)
      i += bridged.length - 1
    }

    const segs: Seg[] = []
    let del = ''
    let ins = ''
    const flush = () => {
      if (!del && !ins) return
      const id = newId()
      changes.push({ id, kind: kindOf(del, ins), del, ins, wholeLine: false })
      segs.push({ change: id, del, ins })
      del = ''
      ins = ''
    }
    for (const o of ops) {
      if (o.op === 'eq') {
        flush()
        segs.push({ text: o.a })
      } else if (o.op === 'del') {
        del += o.a
      } else {
        ins += o.b
      }
    }
    flush()
    result.push({ type: 'mod', prefix: newPrefix, segs })
    return true
  }

  const ops = align(lines(base), lines(edited))
  let k = 0
  while (k < ops.length) {
    const o = ops[k]
    if (o.op === 'eq') {
      result.push(o.a === '' ? { type: 'blank' } : { type: 'same', text: o.a })
      k++
      continue
    }
    // A run of removed and added lines. Pair them in order, so a reworded
    // line becomes one word-level change instead of a delete and an insert.
    const dels: string[] = []
    const inss: string[] = []
    while (k < ops.length && ops[k].op !== 'eq') {
      const r = ops[k++]
      if (r.op === 'del') dels.push(r.a)
      else if (r.op === 'ins') inss.push(r.b)
    }
    const oldText = dels.filter((l) => l !== '')
    const newText = inss.filter((l) => l !== '')
    const paired = Math.min(oldText.length, newText.length)
    const leftOld: string[] = []
    const leftNew: string[] = []
    for (let p = 0; p < paired; p++) {
      if (!modified(oldText[p], newText[p])) {
        leftOld.push(oldText[p])
        leftNew.push(newText[p])
      }
    }
    leftOld.push(...oldText.slice(paired))
    leftNew.push(...newText.slice(paired))
    leftOld.forEach((l) => whole('del', l))
    if (leftNew.length && inss[0] === '') result.push({ type: 'blank' })
    leftNew.forEach((l) => whole('ins', l))
  }
  return { lines: result, changes }
}
