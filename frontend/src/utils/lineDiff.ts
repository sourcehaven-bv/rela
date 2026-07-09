// A tiny line-level diff (LCS-based) for showing what changed between two
// versions of an entity's markdown content. Kept dependency-free and small —
// the panel only needs added/removed/unchanged line classification, not a full
// patch format.

export type DiffOp = 'equal' | 'add' | 'del'

export interface DiffLine {
  op: DiffOp
  text: string
}

// maxLcsCells bounds the O(n·m) LCS table so a large document can't freeze the
// tab (a full table for two 5000-line bodies is ~25M cells / hundreds of MB,
// synchronous on the main thread). Above this, lineDiff falls back to a coarse
// "everything removed then everything added" diff, which is still correct —
// just not a minimal edit script.
const maxLcsCells = 2_000_000

/**
 * lineDiff returns a line-by-line diff of `before` → `after`. Equal lines are
 * emitted once (op 'equal'); a changed region emits its removed lines ('del')
 * then its added lines ('add'). Empty inputs are handled (all-add / all-del).
 *
 * For very large inputs (n·m over maxLcsCells after trimming the common
 * prefix/suffix) it degrades to a coarse block diff rather than allocating an
 * unbounded table — a UI-DoS guard, not a correctness change.
 */
export function lineDiff(before: string, after: string): DiffLine[] {
  const aFull = before.length ? before.split('\n') : []
  const bFull = after.length ? after.split('\n') : []

  // Trim the common prefix/suffix first: unchanged head/tail lines are emitted
  // as 'equal' directly and kept out of the O(n·m) core, which both speeds up
  // the common "one edit in a big file" case and shrinks the table.
  let lo = 0
  while (lo < aFull.length && lo < bFull.length && aFull[lo] === bFull[lo]) lo++
  let aHi = aFull.length
  let bHi = bFull.length
  while (aHi > lo && bHi > lo && aFull[aHi - 1] === bFull[bHi - 1]) {
    aHi--
    bHi--
  }
  const prefix: DiffLine[] = aFull.slice(0, lo).map((text) => ({ op: 'equal' as const, text }))
  const suffix: DiffLine[] = aFull.slice(aHi).map((text) => ({ op: 'equal' as const, text }))
  const a = aFull.slice(lo, aHi)
  const b = bFull.slice(lo, bHi)

  const n = a.length
  const m = b.length

  // Guard: if the changed region is still too large for an LCS table, degrade
  // to a coarse block diff (all removed, then all added).
  if (n * m > maxLcsCells) {
    const coarse: DiffLine[] = [
      ...a.map((text) => ({ op: 'del' as const, text })),
      ...b.map((text) => ({ op: 'add' as const, text })),
    ]
    return [...prefix, ...coarse, ...suffix]
  }

  // LCS length table.
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
  return [...prefix, ...out, ...suffix]
}

export interface PropertyChange {
  key: string
  op: 'add' | 'del' | 'change'
  before?: unknown
  after?: unknown
}

// stableStringify serializes a value with object keys sorted recursively, so
// equality comparison is independent of key order — {a:1,b:2} and {b:2,a:1}
// compare equal. Without this, a property whose value is an object could report
// a phantom "change" purely from a key-order difference between the two sides
// (Go's encoding/json sorts map keys, but the live-entity path may not).
function stableStringify(v: unknown): string {
  if (v === null || typeof v !== 'object') return JSON.stringify(v) ?? 'null'
  if (Array.isArray(v)) return '[' + v.map(stableStringify).join(',') + ']'
  const obj = v as Record<string, unknown>
  const keys = Object.keys(obj).sort()
  return '{' + keys.map((k) => JSON.stringify(k) + ':' + stableStringify(obj[k])).join(',') + '}'
}

/**
 * propertyDiff compares two property maps and returns the keys that were added,
 * removed, or changed (stable-sorted by key). Value equality is key-order-
 * independent (see stableStringify) so object-valued properties don't produce
 * phantom changes.
 */
export function propertyDiff(
  before: Record<string, unknown>,
  after: Record<string, unknown>
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
    } else if (stableStringify(before[key]) !== stableStringify(after[key])) {
      changes.push({ key, op: 'change', before: before[key], after: after[key] })
    }
  }
  return changes
}
