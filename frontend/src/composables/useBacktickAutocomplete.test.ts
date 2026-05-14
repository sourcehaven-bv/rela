import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { searchEntities, listEntities } from '@/api'
import type { Entity, EntityType, ListResponse } from '@/types'
import {
  buildPrefixList,
  useBacktickAutocomplete,
  type EasyMdeLike,
  OPEN_DELAY_MS,
} from './useBacktickAutocomplete'

vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return { ...actual, searchEntities: vi.fn(), listEntities: vi.fn() }
})

const searchSpy = searchEntities as unknown as ReturnType<typeof vi.fn>
const listSpy = listEntities as unknown as ReturnType<typeof vi.fn>

interface Pos {
  line: number
  ch: number
}

/**
 * Hand-rolled CodeMirror shim. The composable touches a handful of
 * methods (`getCursor`, `getTokenAt`, `getRange`, `replaceRange`,
 * `charCoords`, plus the event emitter). The shim records calls and
 * lets tests drive the editor like a state machine without standing up
 * real EasyMDE/CodeMirror.
 */
function makeFakeEditor(opts: {
  text?: string
  cursor?: Pos
  tokenAt?: (pos: Pos) => { type: string | null; string: string }
} = {}): {
  editor: EasyMdeLike
  fire: (event: string, ...args: unknown[]) => void
  setCursor: (pos: Pos) => void
  setText: (text: string) => void
  getText: () => string
  replaceRangeMock: ReturnType<typeof vi.fn>
  charCoordsMock: ReturnType<typeof vi.fn>
} {
  let text = opts.text ?? ''
  let cursor: Pos = opts.cursor ?? { line: 0, ch: 0 }
  const handlers = new Map<string, Array<(...args: unknown[]) => void>>()
  const tokenAt =
    opts.tokenAt ??
    (() => ({ type: null, string: '' }))

  const replaceRangeMock = vi.fn(
    (newText: string, _from: Pos, _to?: Pos) => {
      // Minimal model — assume single-line edits since tests stay on line 0.
      const from = _from
      const to = _to ?? _from
      const lines = text.split('\n')
      const line = lines[from.line] ?? ''
      lines[from.line] =
        line.substring(0, from.ch) + newText + line.substring(to.ch)
      text = lines.join('\n')
      cursor = { line: from.line, ch: from.ch + newText.length }
    },
  )

  const charCoordsMock = vi.fn(() => ({ left: 100, top: 200, bottom: 220, right: 110 }))

  const cm = {
    getCursor: (_side?: 'from' | 'to' | 'anchor' | 'head') => ({ ...cursor }),
    getTokenAt: (pos: Pos) => tokenAt(pos),
    getRange: (from: Pos, to: Pos) => {
      const lines = text.split('\n')
      if (from.line !== to.line) return ''
      const line = lines[from.line] ?? ''
      return line.substring(from.ch, to.ch)
    },
    replaceRange: replaceRangeMock,
    charCoords: charCoordsMock,
    on: (event: string, handler: (...args: unknown[]) => void) => {
      if (!handlers.has(event)) handlers.set(event, [])
      handlers.get(event)!.push(handler)
    },
    off: (event: string, handler: (...args: unknown[]) => void) => {
      const list = handlers.get(event)
      if (!list) return
      const idx = list.indexOf(handler)
      if (idx >= 0) list.splice(idx, 1)
    },
  }

  // The fake doesn't bother implementing CodeMirror's overloaded
  // `on`/`off` typed event signatures — a single generic handler suffices
  // for the test shim. Cast through `unknown` to align with EasyMdeLike's
  // typed shape.
  const editor = { codemirror: cm } as unknown as EasyMdeLike
  return {
    editor,
    fire: (event, ...args) => {
      const list = handlers.get(event)
      if (!list) return
      // Iterate a copy so handlers that unsubscribe during dispatch
      // don't shift the iteration.
      for (const h of [...list]) h(...args)
    },
    setCursor: (pos) => {
      cursor = pos
    },
    setText: (newText) => {
      text = newText
    },
    getText: () => text,
    replaceRangeMock,
    charCoordsMock,
  }
}

function makeEntityTypes(): Map<string, EntityType> {
  const types = new Map<string, EntityType>()
  types.set('ticket', {
    label: 'Ticket',
    id_prefix: 'TKT-',
    id_type: 'short',
    properties: {},
  } as EntityType)
  types.set('feature', {
    label: 'Feature',
    id_prefix: 'FEAT-',
    id_type: 'short',
    properties: {},
  } as EntityType)
  types.set('decision', {
    label: 'Decision',
    id_prefix: 'DEC-',
    id_type: 'short',
    properties: {},
  } as EntityType)
  types.set('concept', {
    label: 'Concept',
    id_type: 'manual',
    properties: {},
  } as EntityType)
  return types
}

function makeListResponse(entities: Entity[]): ListResponse<Entity> {
  return {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
  }
}

describe('buildPrefixList', () => {
  it('emits one entry per id_prefix and per id_prefixes element', () => {
    const types = new Map<string, EntityType>()
    types.set('ticket', { label: 'Ticket', id_prefix: 'TKT-' } as EntityType)
    types.set('multi', {
      label: 'Multi',
      id_prefixes: ['A-', 'B-'],
    } as EntityType)
    const list = buildPrefixList(types)
    expect(list).toHaveLength(3)
    expect(list.map((p) => p.prefix).sort()).toEqual(['A-', 'B-', 'TKT-'])
  })

  it('emits a single manual entry for id_type: manual', () => {
    const types = new Map<string, EntityType>()
    types.set('concept', {
      label: 'Concept',
      id_type: 'manual',
    } as EntityType)
    const list = buildPrefixList(types)
    expect(list).toEqual([
      { prefix: '', type: 'concept', label: 'Concept', isManual: true },
    ])
  })

  it('sorts non-manual entries alphabetically before manual entries', () => {
    const types = new Map<string, EntityType>()
    types.set('ticket', { label: 'Ticket', id_prefix: 'TKT-' } as EntityType)
    types.set('concept', { label: 'Concept', id_type: 'manual' } as EntityType)
    types.set('decision', { label: 'Decision', id_prefix: 'DEC-' } as EntityType)
    const list = buildPrefixList(types)
    expect(list.map((p) => p.prefix)).toEqual(['DEC-', 'TKT-', ''])
  })

  it('dedupes prefixes that appear in both id_prefix and id_prefixes', () => {
    // The backend mirrors `id_prefix` into `id_prefixes` for short-id
    // types — both fields hold `["BUG-"]`. Without dedup the list would
    // show two `Bug / BUG-*` rows.
    const types = new Map<string, EntityType>()
    types.set('bug', {
      label: 'Bug',
      id_prefix: 'BUG-',
      id_prefixes: ['BUG-'],
    } as EntityType)
    const list = buildPrefixList(types)
    expect(list).toEqual([
      { prefix: 'BUG-', type: 'bug', label: 'Bug', isManual: false },
    ])
  })
})

describe('useBacktickAutocomplete', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    searchSpy.mockReset()
    listSpy.mockReset()
    searchSpy.mockResolvedValue(makeListResponse([]))
    listSpy.mockResolvedValue(makeListResponse([]))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('trigger detection', () => {
    it('opens in prose context after the open delay', async () => {
      // Token AFTER is `formatting` (CM marker for inline-code boundary);
      // Token BEFORE is null (start of line / prose text).
      const fake = makeFakeEditor({
        cursor: { line: 0, ch: 1 },
        tokenAt: (pos) =>
          pos.ch === 1
            ? { type: 'formatting formatting-code comment', string: '`' }
            : { type: null, string: '' },
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      expect(ctl.state.phase).toBe('pending')
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      expect(ctl.state.phase).toBe('prefix')
      expect(ctl.state.prefixItems.length).toBeGreaterThan(0)
      ctl.dispose()
    })

    it('suppresses inside a fenced code block', () => {
      // Token AFTER is `comment` (overlay-painted content) with no
      // `formatting`. Composable must NOT transition out of idle.
      const fake = makeFakeEditor({
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })

    it('suppresses inside a link URL', () => {
      const fake = makeFakeEditor({
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'url', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })

    it('suppresses when closing an open code span', () => {
      // Token AFTER does have `formatting` (CM paints both boundaries),
      // but token BEFORE is `comment` (the code-span content). Composable
      // must use the BEFORE side to reject this case.
      const fake = makeFakeEditor({
        text: 'see `foo`',
        cursor: { line: 0, ch: 9 },
        tokenAt: (pos) => {
          if (pos.ch === 9 || pos.ch === 4) {
            return { type: 'formatting formatting-code comment', string: '`' }
          }
          return { type: 'comment', string: 'foo' }
        },
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 8 } })
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })

    it('does not transition on a non-backtick keystroke', () => {
      const fake = makeFakeEditor({
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: 'a' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['a'], from: { line: 0, ch: 0 } })
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })
  })

  describe('open delay', () => {
    it('cancels the open when a non-ID character is typed during the delay', () => {
      const fake = makeFakeEditor({
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      expect(ctl.state.phase).toBe('pending')
      // User types a space — buffer becomes "` " and cursor moves to ch:2.
      fake.setText('` ')
      fake.setCursor({ line: 0, ch: 2 })
      fake.fire('change')
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })

    it('cancels the open when Escape is pressed during the delay', () => {
      const fake = makeFakeEditor({
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      const event = new KeyboardEvent('keydown', { key: 'Escape' })
      vi.spyOn(event, 'preventDefault')
      fake.fire('keydown', null, event)
      expect(ctl.state.phase).toBe('idle')
      expect(event.preventDefault).toHaveBeenCalled()
      ctl.dispose()
    })
  })

  describe('phase transitions', () => {
    it('transitions to phase id when typed text equals a prefix', async () => {
      const seed: Entity = { id: 'TKT-1', type: 'ticket', properties: {} }
      listSpy.mockResolvedValueOnce(makeListResponse([seed]))
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      expect(ctl.state.phase).toBe('prefix')
      // User types TKT- after the trigger.
      fake.setText('`TKT-')
      fake.setCursor({ line: 0, ch: 5 })
      fake.fire('change')
      expect(ctl.state.phase).toBe('id')
      expect(ctl.state.resolvedPrefix?.prefix).toBe('TKT-')
      // Debounced search fires after 150ms. Empty post-prefix query
      // calls listEntities (the typed-listing pathway) rather than
      // searchEntities to avoid Bleve scoring (RR-UNAK).
      vi.advanceTimersByTime(200)
      await vi.runAllTimersAsync()
      expect(listSpy).toHaveBeenCalled()
      const [type] = listSpy.mock.calls[0]
      expect(type).toBe('ticket')
      ctl.dispose()
    })

    it('uses searchEntities once the partial id query has characters', async () => {
      const seed: Entity = { id: 'TKT-1', type: 'ticket', properties: {} }
      searchSpy.mockResolvedValueOnce(makeListResponse([seed]))
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      // Skip ahead — user types TKT-1
      fake.setText('`TKT-1')
      fake.setCursor({ line: 0, ch: 6 })
      fake.fire('change')
      expect(ctl.state.phase).toBe('id')
      vi.advanceTimersByTime(200)
      await vi.runAllTimersAsync()
      expect(searchSpy).toHaveBeenCalled()
      const [q, type] = searchSpy.mock.calls[0]
      expect(q).toBe('1')
      expect(type).toBe('ticket')
      ctl.dispose()
    })

    it('filters prefix list by typed substring', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      expect(ctl.state.prefixItems.length).toBeGreaterThan(1)
      fake.setText('`F')
      fake.setCursor({ line: 0, ch: 2 })
      fake.fire('change')
      // After typing 'F' only FEAT- (and the manual Concept by substring
      // search) should remain — but Concept doesn't contain 'f' so just
      // FEAT-.
      expect(ctl.state.prefixItems.map((p) => p.prefix)).toEqual(['FEAT-'])
      ctl.dispose()
    })
  })

  describe('keyboard navigation', () => {
    it('ArrowDown wraps the highlight', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      const len = ctl.state.prefixItems.length
      expect(len).toBeGreaterThan(1)
      const ev = new KeyboardEvent('keydown', { key: 'ArrowDown' })
      vi.spyOn(ev, 'preventDefault')
      fake.fire('keydown', null, ev)
      expect(ctl.state.highlightedIndex).toBe(1)
      expect(ev.preventDefault).toHaveBeenCalled()
      // Wrap-around from last to first.
      for (let i = 1; i < len; i++) {
        fake.fire('keydown', null, new KeyboardEvent('keydown', { key: 'ArrowDown' }))
      }
      expect(ctl.state.highlightedIndex).toBe(0)
      ctl.dispose()
    })

    it('Escape dismisses the session', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      const ev = new KeyboardEvent('keydown', { key: 'Escape' })
      fake.fire('keydown', null, ev)
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })

    it('passes through non-navigation keys to the editor', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      const ev = new KeyboardEvent('keydown', { key: 'a' })
      const pdSpy = vi.spyOn(ev, 'preventDefault')
      fake.fire('keydown', null, ev)
      expect(pdSpy).not.toHaveBeenCalled()
      ctl.dispose()
    })
  })

  describe('auto-dismiss', () => {
    it('closes on space typed', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      expect(ctl.state.phase).toBe('prefix')
      fake.setText('` ')
      fake.setCursor({ line: 0, ch: 2 })
      fake.fire('change')
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })

    it('closes when cursor moves off the trigger line', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      fake.setCursor({ line: 1, ch: 0 })
      fake.fire('cursorActivity')
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })

    it('closes when cursor jumps past the typed-after-trigger range (RR-E25Z)', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      expect(ctl.state.phase).toBe('prefix')
      // User mouse-clicks far to the right on the same line.
      fake.setCursor({ line: 0, ch: 50 })
      fake.fire('cursorActivity')
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })

    it('closes on editor blur', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      fake.fire('blur')
      expect(ctl.state.phase).toBe('idle')
      ctl.dispose()
    })
  })

  describe('dispose', () => {
    it('removes all CodeMirror listeners', () => {
      const fake = makeFakeEditor({
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      ctl.dispose()
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      // No state mutation should happen after dispose.
      expect(ctl.state.phase).toBe('idle')
    })
  })

  describe('setHighlight', () => {
    it('clamps out-of-range indices', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      const len = ctl.state.prefixItems.length
      ctl.setHighlight(999)
      expect(ctl.state.highlightedIndex).toBe(len - 1)
      ctl.setHighlight(-5)
      expect(ctl.state.highlightedIndex).toBe(0)
      ctl.setHighlight(1)
      expect(ctl.state.highlightedIndex).toBe(1)
      ctl.dispose()
    })
  })

  describe('pick', () => {
    it('inserts the prefix and transitions to phase id', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      // The list is sorted alphabetically; the first prefix is DEC-.
      expect(ctl.state.prefixItems[0].prefix).toBe('DEC-')
      ctl.pick()
      expect(ctl.state.phase).toBe('id')
      expect(ctl.state.resolvedPrefix?.prefix).toBe('DEC-')
      expect(fake.replaceRangeMock).toHaveBeenCalledWith(
        'DEC-',
        { line: 0, ch: 1 },
        expect.any(Object),
      )
      ctl.dispose()
    })

    it('manual-id prefix jumps to phase 2 without inserting text', () => {
      const fake = makeFakeEditor({
        text: '`',
        cursor: { line: 0, ch: 1 },
        tokenAt: () => ({ type: 'formatting formatting-code comment', string: '`' }),
      })
      const ctl = useBacktickAutocomplete(fake.editor, () => makeEntityTypes())
      fake.fire('inputRead', null, { text: ['`'], from: { line: 0, ch: 0 } })
      vi.advanceTimersByTime(OPEN_DELAY_MS + 10)
      // Manual entry is the last after sorting (Concept).
      const manualIdx = ctl.state.prefixItems.findIndex((p) => p.isManual)
      expect(manualIdx).toBeGreaterThanOrEqual(0)
      ctl.setHighlight(manualIdx)
      ctl.pick()
      expect(ctl.state.phase).toBe('id')
      // No replaceRange call — manual types insert nothing on prefix
      // selection (no prefix text exists).
      expect(fake.replaceRangeMock).not.toHaveBeenCalled()
      ctl.dispose()
    })
  })
})
