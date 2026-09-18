import { describe, it, expect, vi } from 'vitest'
import { createMentionKeydownHandler, type MentionKeymapHost } from './mentionKeymap'
import type { MentionChoice, MentionMenuController, MentionMenuState } from './useMentionMenu'

/** A controller stub exposing only what the keymap touches. */
function menuStub(state: Partial<MentionMenuState>, choice: MentionChoice | null = null) {
  const calls = {
    moveHighlight: vi.fn(),
    clearType: vi.fn(),
    close: vi.fn(),
  }
  const menuState: MentionMenuState = {
    open: true,
    query: '',
    items: [],
    // The shared snapshot's numeric index; the SPA menu resolves its own
    // highlight from `highlight` instead, so this is only here to satisfy the
    // inherited type.
    highlightedIndex: 0,
    typeItems: [],
    selectedType: null,
    highlight: null,
    loading: false,
    errorMsg: '',
    ...state,
  }
  const menu = {
    state: menuState,
    current: () => choice,
    ...calls,
  } as unknown as MentionMenuController
  return { menu, calls }
}

function hostStub(queryText: string | undefined): {
  host: MentionKeymapHost
  commit: ReturnType<typeof vi.fn>
  dismiss: ReturnType<typeof vi.fn>
} {
  const commit = vi.fn()
  const dismiss = vi.fn()
  return { host: { queryText: () => queryText, commit, dismiss }, commit, dismiss }
}

function keyEvent(key: string): KeyboardEvent & { preventDefault: ReturnType<typeof vi.fn> } {
  return { key, preventDefault: vi.fn() } as unknown as KeyboardEvent & {
    preventDefault: ReturnType<typeof vi.fn>
  }
}

describe('createMentionKeydownHandler', () => {
  it('ignores every key while the menu is closed', () => {
    const { menu, calls } = menuStub({ open: false })
    const { host, commit } = hostStub('@')
    const handler = createMentionKeydownHandler(menu, host)
    for (const key of ['ArrowDown', 'Enter', 'Backspace', 'Escape']) {
      const ev = keyEvent(key)
      handler(ev)
      expect(ev.preventDefault).not.toHaveBeenCalled()
    }
    expect(calls.moveHighlight).not.toHaveBeenCalled()
    expect(commit).not.toHaveBeenCalled()
  })

  it('moves the highlight on the arrow keys', () => {
    const { menu, calls } = menuStub({})
    const { host } = hostStub('@')
    const handler = createMentionKeydownHandler(menu, host)

    const down = keyEvent('ArrowDown')
    handler(down)
    expect(down.preventDefault).toHaveBeenCalled()
    expect(calls.moveHighlight).toHaveBeenCalledWith(1)

    handler(keyEvent('ArrowUp'))
    expect(calls.moveHighlight).toHaveBeenLastCalledWith(-1)
  })

  it.each(['Enter', 'Tab'])('commits the highlighted choice on %s', (key) => {
    const choice: MentionChoice = { kind: 'type', name: 'ticket' }
    const { menu } = menuStub({}, choice)
    const { host, commit } = hostStub('@')
    const ev = keyEvent(key)
    createMentionKeydownHandler(menu, host)(ev)
    expect(ev.preventDefault).toHaveBeenCalled()
    expect(commit).toHaveBeenCalledWith(choice)
  })

  it('lets Enter through when there is nothing highlighted', () => {
    const { menu } = menuStub({}, null)
    const { host, commit } = hostStub('@')
    const ev = keyEvent('Enter')
    createMentionKeydownHandler(menu, host)(ev)
    // Not prevented: with no row to pick, Enter must stay a paragraph break.
    expect(ev.preventDefault).not.toHaveBeenCalled()
    expect(commit).not.toHaveBeenCalled()
  })

  it('dismisses on Escape, carrying the query so the menu stays shut', () => {
    const { menu } = menuStub({ query: 'tkt' })
    const { host, dismiss } = hostStub('@tkt')
    const ev = keyEvent('Escape')
    createMentionKeydownHandler(menu, host)(ev)
    expect(ev.preventDefault).toHaveBeenCalled()
    expect(dismiss).toHaveBeenCalledWith('tkt')
  })

  describe('Backspace', () => {
    it('clears the scope when the live query is already empty', () => {
      const { menu, calls } = menuStub({ selectedType: 'ticket' })
      const { host } = hostStub('see @')
      const ev = keyEvent('Backspace')
      createMentionKeydownHandler(menu, host)(ev)
      // Prevented, so the `@` trigger survives and the menu stays open.
      expect(ev.preventDefault).toHaveBeenCalled()
      expect(calls.clearType).toHaveBeenCalled()
    })

    it('deletes text instead while the query still has characters', () => {
      const { menu, calls } = menuStub({ selectedType: 'ticket' })
      const { host } = hostStub('see @f')
      const ev = keyEvent('Backspace')
      createMentionKeydownHandler(menu, host)(ev)
      expect(ev.preventDefault).not.toHaveBeenCalled()
      expect(calls.clearType).not.toHaveBeenCalled()
    })

    it('does nothing special with no scope set', () => {
      const { menu, calls } = menuStub({ selectedType: null })
      const { host } = hostStub('see @')
      const ev = keyEvent('Backspace')
      createMentionKeydownHandler(menu, host)(ev)
      expect(ev.preventDefault).not.toHaveBeenCalled()
      expect(calls.clearType).not.toHaveBeenCalled()
    })

    // The regression this guards: reading `menu.state.query` instead of the
    // live document. That mirror lags one ProseMirror tick, so on the keystroke
    // that empties the query it still holds the previous character. A handler
    // trusting it fires one keystroke late and the `@` gets deleted.
    it('uses the live document, not the menu state mirror', () => {
      // Mirror says 'f' (stale); the document says the query is already empty.
      const { menu, calls } = menuStub({ selectedType: 'ticket', query: 'f' })
      const { host } = hostStub('see @')
      const ev = keyEvent('Backspace')
      createMentionKeydownHandler(menu, host)(ev)
      expect(calls.clearType).toHaveBeenCalled()
      expect(ev.preventDefault).toHaveBeenCalled()
    })

    it('does not clear when the cursor is outside a mention', () => {
      const { menu, calls } = menuStub({ selectedType: 'ticket' })
      const { host } = hostStub(undefined)
      const ev = keyEvent('Backspace')
      createMentionKeydownHandler(menu, host)(ev)
      expect(ev.preventDefault).not.toHaveBeenCalled()
      expect(calls.clearType).not.toHaveBeenCalled()
    })
  })

  it('leaves an unrelated key alone', () => {
    const { menu, calls } = menuStub({})
    const { host, commit } = hostStub('@')
    const ev = keyEvent('a')
    createMentionKeydownHandler(menu, host)(ev)
    expect(ev.preventDefault).not.toHaveBeenCalled()
    expect(calls.moveHighlight).not.toHaveBeenCalled()
    expect(commit).not.toHaveBeenCalled()
  })
})
