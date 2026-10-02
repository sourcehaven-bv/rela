import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import KeyboardShortcutsModal from './KeyboardShortcutsModal.vue'

vi.mock('@/composables/modalStack', () => ({ useModalStack: () => {} }))

/**
 * The shortcut list's key rendering.
 *
 * RlKbd splits a combination on its separator and builds the accessible name
 * from the parts, so the SEPARATOR CHOICE is what makes `G then D` announce as
 * three tokens rather than as the unpronounceable "GthenD". That choice is
 * derived from the string here, which makes it this component's logic and the
 * one thing worth pinning: pass the wrong separator and the keys still render
 * correctly while the announcement silently degrades.
 *
 * Asserted through the rendered output rather than by exporting the helper —
 * the guarantee is what a screen reader receives, not how it was computed.
 */
describe('KeyboardShortcutsModal', () => {
  function mountOpen() {
    return mount(KeyboardShortcutsModal, {
      props: { open: true },
      global: {
        // RlModal teleports to body, which detaches it from the wrapper.
        stubs: { teleport: true },
      },
    })
  }

  function rowFor(wrapper: ReturnType<typeof mountOpen>, description: string) {
    const row = wrapper
      .findAll('.shortcut-row')
      .find((r) => r.find('.shortcut-description').text() === description)
    expect(row, `no row for "${description}"`).toBeTruthy()
    return row!
  }

  /** The spoken name of `description`'s key combination. */
  function labelFor(wrapper: ReturnType<typeof mountOpen>, description: string) {
    return rowFor(wrapper, description).find('.shortcut-keys').attributes('aria-label')
  }

  /**
   * The individual keys `description` renders.
   *
   * Asserted ALONGSIDE the spoken label, because the label alone cannot see a
   * wrong separator: splitting `G then D` on `+` yields one part, and joining
   * that one part back with `+` reproduces the original string exactly. The
   * announcement is unchanged and only the key COUNT reveals that nothing was
   * split — three keys became one wide box reading "G then D".
   */
  function keysFor(wrapper: ReturnType<typeof mountOpen>, description: string) {
    return rowFor(wrapper, description)
      .findAll('kbd')
      .map((k) => k.text())
  }

  it('splits a sequence into its keys and speaks the separator between them', () => {
    const wrapper = mountOpen()
    expect(keysFor(wrapper, 'Go to Dashboard')).toEqual(['G', 'D'])
    expect(labelFor(wrapper, 'Go to Dashboard')).toBe('G then D')
  })

  it('splits an alternative rather than running the keys together', () => {
    const wrapper = mountOpen()
    expect(keysFor(wrapper, 'Move selection down')).toEqual(['J', '↓'])
    expect(labelFor(wrapper, 'Move selection down')).toBe('J or ↓')
  })

  it('splits a held combination on "+"', () => {
    const wrapper = mountOpen()
    const keys = keysFor(wrapper, 'Quick jump')
    expect(keys).toHaveLength(2)
    // The modifier is platform-dependent, so match the shape, not the glyph.
    expect(keys[0]).toMatch(/^(⌘|Ctrl)$/)
    expect(keys[1]).toBe('K')
  })

  it('leaves a single key as itself', () => {
    const wrapper = mountOpen()
    expect(keysFor(wrapper, 'Focus search')).toEqual(['/'])
    expect(labelFor(wrapper, 'Focus search')).toBe('/')
  })

  /*
   * The separators must not survive as keys. rela used to split on spaces and
   * test each fragment against a list of separator words; a fragment that
   * failed that test became a <kbd>. This asserts the outcome that mattered:
   * "then" is punctuation, never a key the user presses.
   */
  it('never renders a separator as a key', () => {
    const wrapper = mountOpen()
    const keys = wrapper.findAll('kbd').map((k) => k.text())
    expect(keys).not.toContain('then')
    expect(keys).not.toContain('or')
    expect(keys).not.toContain('+')
  })
})
