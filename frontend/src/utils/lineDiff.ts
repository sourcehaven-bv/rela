// A tiny line-level diff (LCS-based) for showing what changed between two
// versions of an entity's markdown content. Kept dependency-free and small —
// the panel only needs added/removed/unchanged line classification, not a full
// patch format.

export type DiffOp = 'equal' | 'add' | 'del'

export interface DiffLine {
  op: DiffOp
  text: string
}

/**
 * lineDiff returns a line-by-line diff of `before` → `after`. Equal lines are
 * emitted once (op 'equal'); a changed region emits its removed lines ('del')
 * then its added lines ('add'). Empty inputs are handled (all-add / all-del).
 */
export function lineDiff(before: string, after: string): DiffLine[] {
  const a = before.length ? before.split('\n') : []
  const b = after.length ? after.split('\n') : []

  // LCS length table.
  const n = a.length
  const m = b.length
  const lcs: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1])
    }
  }

  // Walk the table to produce the diff.
  const out: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ op: 'equal', text: a[i] })
      i++
      j++
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      out.push({ op: 'del', text: a[i] })
      i++
    } else {
      out.push({ op: 'add', text: b[j] })
      j++
    }
  }
  while (i < n) {
    out.push({ op: 'del', text: a[i] })
    i++
  }
  while (j < m) {
    out.push({ op: 'add', text: b[j] })
    j++
  }
  return out
}

export interface PropertyChange {
  key: string
  op: 'add' | 'del' | 'change'
  before?: unknown
  after?: unknown
}

/**
 * propertyDiff compares two property maps and returns the keys that were added,
 * removed, or changed (stable-sorted by key). Values are compared by JSON
 * serialization — adequate for the scalar/list/object property shapes rela uses.
 */
export function propertyDiff(
  before: Record<string, unknown>,
  after: Record<string, unknown>,
): PropertyChange[] {
  const keys = new Set<string>([...Object.keys(before ?? {}), ...Object.keys(after ?? {})])
  const changes: PropertyChange[] = []
  for (const key of [...keys].sort()) {
    const inB = before && key in before
    const inA = after && key in after
    if (inB && !inA) {
      changes.push({ key, op: 'del', before: before[key] })
    } else if (!inB && inA) {
      changes.push({ key, op: 'add', after: after[key] })
    } else if (JSON.stringify(before[key]) !== JSON.stringify(after[key])) {
      changes.push({ key, op: 'change', before: before[key], after: after[key] })
    }
  }
  return changes
}
