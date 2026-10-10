/**
 * Shared state for the track-changes mockups: the suggested changes of one
 * edit session, their accept/reject status, and the body as it looks with
 * them drawn inline.
 */
import { computed, ref, watch, type Ref } from 'vue'
import { diffMarkdown, splitPrefix, type Change, type Diff } from './trackChanges'

export type Status = 'pending' | 'accepted' | 'rejected'
export type View = 'markup' | 'final' | 'original'

export interface ChangeSet {
  id: string
  author: string
  when: string
  base: string
  edited: string
}

export interface Run {
  text: string
  mark?: 'ins' | 'del' | 'fmt'
  change?: string
}
export interface RenderLine {
  kind: 'h1' | 'h2' | 'h3' | 'li' | 'task' | 'quote' | 'p'
  checked?: boolean
  runs: Run[]
  whole?: { change: string; op: 'ins' | 'del' }
}
export interface Block {
  kind: RenderLine['kind'] | 'ul'
  lines: RenderLine[]
}

/** Inline markdown, for display only. */
export function inline(text: string): { text: string; fmt?: 'b' | 'i' | 'code' | 'a' }[] {
  const out: { text: string; fmt?: 'b' | 'i' | 'code' | 'a' }[] = []
  const re = /\*\*([^*]+)\*\*|(?<![\w])_([^_]+)_(?![\w])|`([^`]+)`|\[([^\]]*)\]\([^)]*\)/g
  let last = 0
  for (const m of text.matchAll(re)) {
    if (m.index > last) out.push({ text: text.slice(last, m.index) })
    if (m[1]) out.push({ text: m[1], fmt: 'b' })
    else if (m[2]) out.push({ text: m[2], fmt: 'i' })
    else if (m[3]) out.push({ text: m[3], fmt: 'code' })
    else out.push({ text: m[4], fmt: 'a' })
    last = m.index + m[0].length
  }
  if (last < text.length) out.push({ text: text.slice(last) })
  return out
}

/*
 * Only the newest change set is diffed against the body; overlapping sets
 * from several reviewers are a real design question this does not answer.
 */
export function useTrackedChanges(sets: Ref<ChangeSet[]>, view: Ref<View> = ref('markup')) {
  const status = ref<Record<string, Status>>({})
  const current = computed(() => sets.value[sets.value.length - 1])
  const diff = computed<Diff>(() =>
    current.value
      ? diffMarkdown(current.value.base, current.value.edited, `${current.value.id}-`)
      : { lines: [], changes: [] }
  )

  watch(
    diff,
    (d) => {
      for (const c of d.changes) status.value[c.id] ??= 'pending'
    },
    { immediate: true }
  )

  function resolved(id: string): Status {
    if (view.value === 'original') return 'rejected'
    if (view.value === 'final') return 'accepted'
    return status.value[id] ?? 'pending'
  }

  /** A formatting change is drawn as the new formatting alone, as Word does. */
  const formatOnly = computed(
    () => new Set(diff.value.changes.filter((c) => c.kind === 'formatted').map((c) => c.id))
  )

  interface Run {
    text: string
    mark?: 'ins' | 'del' | 'fmt'
    change?: string
  }
  interface RenderLine {
    kind: 'h1' | 'h2' | 'h3' | 'li' | 'task' | 'quote' | 'p'
    checked?: boolean
    runs: Run[]
    whole?: { change: string; op: 'ins' | 'del' }
  }
  interface Block {
    kind: RenderLine['kind'] | 'ul'
    lines: RenderLine[]
  }

  function lineKind(prefix: string): Pick<RenderLine, 'kind' | 'checked'> {
    const p = prefix.trim()
    if (p.startsWith('###')) return { kind: 'h3' }
    if (p.startsWith('##')) return { kind: 'h2' }
    if (p.startsWith('#')) return { kind: 'h1' }
    if (p.startsWith('>')) return { kind: 'quote' }
    if (/\[[xX]\]$/.test(p)) return { kind: 'task', checked: true }
    if (/\[ \]$/.test(p)) return { kind: 'task', checked: false }
    if (p) return { kind: 'li' }
    return { kind: 'p' }
  }

  /** The body as it should look in the current view, as lines of runs. */
  const renderLines = computed<(RenderLine | null)[]>(() => {
    const out: (RenderLine | null)[] = []
    for (const line of diff.value.lines) {
      if (line.type === 'blank') {
        out.push(null)
      } else if (line.type === 'same') {
        const [prefix, body] = splitPrefix(line.text)
        out.push({ ...lineKind(prefix), runs: [{ text: body }] })
      } else if (line.type === 'whole') {
        const s = resolved(line.change)
        const kept = line.op === 'ins' ? s !== 'rejected' : s !== 'accepted'
        if (!kept) continue
        const [prefix, body] = splitPrefix(line.text)
        const pending = s === 'pending'
        out.push({
          ...lineKind(prefix),
          runs: [{ text: body, mark: pending ? line.op : undefined, change: line.change }],
          whole: pending ? { change: line.change, op: line.op } : undefined,
        })
      } else {
        const runs: Run[] = []
        for (const seg of line.segs) {
          if ('text' in seg) {
            runs.push({ text: seg.text })
            continue
          }
          const s = resolved(seg.change)
          if (s === 'pending' && formatOnly.value.has(seg.change)) {
            runs.push({ text: seg.ins, mark: 'fmt', change: seg.change })
          } else if (s === 'pending') {
            if (seg.del) runs.push({ text: seg.del, mark: 'del', change: seg.change })
            if (seg.ins) runs.push({ text: seg.ins, mark: 'ins', change: seg.change })
          } else {
            runs.push({ text: s === 'accepted' ? seg.ins : seg.del, change: seg.change })
          }
        }
        out.push({ ...lineKind(line.prefix), runs })
      }
    }
    return out
  })

  const blocks = computed<Block[]>(() => {
    const out: Block[] = []
    let open: Block | null = null
    for (const line of renderLines.value) {
      if (!line) {
        open = null
        continue
      }
      const listy = line.kind === 'li' || line.kind === 'task'
      const kind: Block['kind'] = listy ? 'ul' : line.kind
      if (open && open.kind === kind && (kind === 'ul' || kind === 'p' || kind === 'quote')) {
        open.lines.push(line)
      } else {
        open = { kind, lines: [line] }
        out.push(open)
      }
    }
    return out
  })

  function formatLabel(c: Change): string {
    const added = (marker: string) => c.ins.includes(marker) && !c.del.includes(marker)
    const removed = (marker: string) => c.del.includes(marker) && !c.ins.includes(marker)
    if (added('**')) return 'Formatted: bold'
    if (removed('**')) return 'Formatted: not bold'
    if (added('`')) return 'Formatted: code'
    if (added('_')) return 'Formatted: italic'
    if (added('](')) return 'Formatted: link'
    return 'Formatted'
  }

  function label(c: Change): string {
    if (c.kind === 'formatted') return formatLabel(c)
    if (c.wholeLine) {
      const [prefix] = splitPrefix(c.ins || c.del)
      const what = lineKind(prefix).kind.startsWith('h')
        ? 'heading'
        : prefix.trim()
          ? 'list item'
          : 'paragraph'
      return c.kind === 'inserted' ? `Added ${what}` : `Deleted ${what}`
    }
    return { inserted: 'Inserted', deleted: 'Deleted', replaced: 'Replaced' }[c.kind]
  }

  const pending = computed(() => diff.value.changes.filter((c) => status.value[c.id] === 'pending'))

  function setStatus(id: string, s: Status) {
    status.value[id] = s
  }

  function resolveAll(s: Status) {
    for (const c of pending.value) status.value[c.id] = s
  }

  return { status, current, diff, blocks, pending, label, setStatus, resolveAll }
}

export function initials(name: string): string {
  return name
    .split(/\s+/)
    .map((p) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
}
