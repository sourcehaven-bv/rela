import { describe, it, expect, vi, beforeEach } from 'vitest'
import { attachBacktickAutocomplete } from './relaBacktick'

// A minimal fake CodeMirror 5 + EasyMDE wrapper that models a single-line
// buffer and the events the backtick controller subscribes to. The real flow
// is verified end-to-end in-browser (puppeteer); these tests pin the state
// machine (trigger → prefix → id → insert) and the bridge wiring in CI.

interface Pos {
  line: number
  ch: number
}

function makeFakeEditor() {
  let text = ''
  let cursor: Pos = { line: 0, ch: 0 }
  const handlers: Record<string, Array<(...a: unknown[]) => void>> = {}
  // token type returned by getTokenAt — tests set this to steer the gate.
  let afterTokenType: string | null = 'formatting formatting-code comment'
  let beforeTokenType: string | null = null

  const cm = {
    on: (ev: string, fn: (...a: unknown[]) => void) => {
      ;(handlers[ev] ||= []).push(fn)
    },
    off: (ev: string, fn: (...a: unknown[]) => void) => {
      handlers[ev] = (handlers[ev] || []).filter((f) => f !== fn)
    },
    getCursor: (_side?: string) => ({ ...cursor }),
    getRange: (a: Pos, b: Pos) => text.slice(a.ch, b.ch),
    getTokenAt: (pos: Pos) => ({
      type: pos.ch > cursor.ch - 1 ? afterTokenType : beforeTokenType,
      string: '`',
    }),
    charCoords: () => ({ left: 10, top: 20, bottom: 36 }),
    replaceSelection: (t: string) => {
      text = text.slice(0, cursor.ch) + t + text.slice(cursor.ch)
      cursor = { line: 0, ch: cursor.ch + t.length }
    },
    replaceRange: (t: string, from: Pos, to?: Pos) => {
      const end = to ? to.ch : from.ch
      text = text.slice(0, from.ch) + t + text.slice(end)
      cursor = { line: 0, ch: from.ch + t.length }
    },
    focus: vi.fn(),
    // test helpers
    _setText: (t: string, ch: number) => {
      text = t
      cursor = { line: 0, ch }
    },
    _getText: () => text,
    _setTokenTypes: (after: string | null, before: string | null) => {
      afterTokenType = after
      beforeTokenType = before
    },
    _emit: (ev: string, ...args: unknown[]) => {
      ;(handlers[ev] || []).forEach((f) => f(...args))
    },
  }
  return { codemirror: cm } as unknown as Parameters<typeof attachBacktickAutocomplete>[0] & {
    codemirror: typeof cm
  }
}

function makeBridge() {
  return {
    schema: vi.fn(async () => ({
      entities: {
        ticket: { label: 'Ticket', id_prefix: 'TKT-', id_type: 'short' },
        feature: { label: 'Feature', id_prefix: 'FEAT-', id_type: 'short' },
      },
    })),
    list: vi.fn(async () => ({ data: [{ id: 'TKT-AAAA', _title: 'Alpha' }, { id: 'TKT-BBBB', _title: 'Beta' }] })),
    search: vi.fn(async ({ query }: { query: string }) => ({ data: [{ id: 'TKT-AAAA', _title: 'Alpha ' + query }] })),
  }
}

// Fire the inputRead the way CM5 does when a backtick is typed at `ch`.
function typeBacktick(ed: ReturnType<typeof makeFakeEditor>, before: string) {
  const cm = ed.codemirror
  cm._setText(before + '`', before.length + 1)
  cm._emit('inputRead', cm, { from: { line: 0, ch: before.length }, to: { line: 0, ch: before.length }, text: ['`'], origin: '+input' })
}

const flushTimers = () => new Promise((r) => setTimeout(r, 350))

describe('attachBacktickAutocomplete', () => {
  let ed: ReturnType<typeof makeFakeEditor>
  let bridge: ReturnType<typeof makeBridge>
  let popup: () => HTMLElement | null

  beforeEach(() => {
    document.body.replaceChildren()
    ed = makeFakeEditor()
    bridge = makeBridge()
    popup = () => document.querySelector('.rela-bt-popup')
  })

  it('warms the prefix cache from the bridge schema', async () => {
    attachBacktickAutocomplete(ed, bridge)
    await Promise.resolve()
    expect(bridge.schema).toHaveBeenCalled()
  })

  it('shows the prefix popup after typing a backtick in inline text', async () => {
    attachBacktickAutocomplete(ed, bridge)
    await flushTimers() // let the schema cache warm
    typeBacktick(ed, 'see ')
    await flushTimers() // past the open delay
    const p = popup()!
    expect(p.style.display).toBe('block')
    const opts = [...p.querySelectorAll('.rela-bt-option')].map((o) => o.textContent)
    expect(opts.length).toBe(2)
    expect(opts.join(' ')).toContain('Feature')
    expect(opts.join(' ')).toContain('Ticket')
  })

  it('does NOT open inside a non-inline context (e.g. a code/comment token before)', async () => {
    attachBacktickAutocomplete(ed, bridge)
    await flushTimers()
    ed.codemirror._setTokenTypes('formatting formatting-code comment', 'comment')
    typeBacktick(ed, 'x')
    await flushTimers()
    expect(popup()!.style.display).toBe('none')
  })

  it('picks a prefix → id phase → inserts `<id>` and closes', async () => {
    attachBacktickAutocomplete(ed, bridge)
    await flushTimers()
    typeBacktick(ed, 'see ')
    await flushTimers()
    // click "Ticket"
    const ticket = [...popup()!.querySelectorAll('.rela-bt-option')].find((o) =>
      o.textContent?.includes('Ticket'),
    ) as HTMLElement
    ticket.click()
    await flushTimers() // debounced list fetch
    expect(bridge.list).toHaveBeenCalled()
    const entOpts = [...popup()!.querySelectorAll('.rela-bt-option')]
    expect(entOpts.length).toBeGreaterThan(0)
    ;(entOpts[0] as HTMLElement).click()
    await Promise.resolve()
    expect(ed.codemirror._getText()).toBe('see `TKT-AAAA`')
    expect(popup()!.style.display).toBe('none')
  })

  it('destroy() removes the popup and unsubscribes', async () => {
    const ctrl = attachBacktickAutocomplete(ed, bridge)
    await flushTimers()
    ctrl.destroy()
    expect(popup()).toBeNull()
    // a post-destroy backtick must not resurrect the popup
    typeBacktick(ed, 'a ')
    await flushTimers()
    expect(popup()).toBeNull()
  })
})
