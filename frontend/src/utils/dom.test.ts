import { describe, it, expect, afterEach } from 'vitest'
import { isInputFocused } from './dom'

/**
 * Unit coverage for the guard every global keyboard handler must use.
 *
 * The case that matters most is `contenteditable`: BUG-DNP5E7 shipped because
 * two handlers hand-rolled an `['INPUT','TEXTAREA']` tag-name check instead of
 * calling this, and Milkdown/ProseMirror edits in a contenteditable div. A
 * tag-name test cannot see that surface, so `/` navigated away mid-sentence.
 *
 * The behavioural half — that the handlers actually consult this — lives in
 * Sidebar.shortcut.test.ts and SearchView.shortcut.test.ts. Both halves are
 * needed: this file would keep passing if a handler stopped calling it.
 */
describe('isInputFocused', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  function focused(make: () => HTMLElement): boolean {
    const el = make()
    document.body.appendChild(el)
    el.focus()
    return isInputFocused()
  }

  it('is false when nothing is focused', () => {
    expect(isInputFocused()).toBe(false)
  })

  it.each([
    ['input', () => document.createElement('input')],
    ['textarea', () => document.createElement('textarea')],
    ['select', () => document.createElement('select')],
  ])('is true for a focused <%s>', (_name, make) => {
    expect(focused(make)).toBe(true)
  })

  it('is true for a focused contenteditable host', () => {
    // The Milkdown/ProseMirror shape. This is the assertion BUG-DNP5E7 needed.
    expect(
      focused(() => {
        const el = document.createElement('div')
        el.setAttribute('contenteditable', 'true')
        el.className = 'ProseMirror milkdown-prose'
        return el
      })
    ).toBe(true)
  })

  it.each(['checkbox', 'radio'])('is true for a focused <input type="%s">', (type) => {
    // Deliberate and worth pinning: these are INPUT, so the guard blocks. A
    // checkbox takes no text, which makes it tempting for someone later to
    // narrow the tag test to "text-like inputs only" — that narrowing is how
    // BUG-DNP5E7 would come back, since the same reasoning excludes
    // contenteditable. Blocking a shortcut on a focused checkbox costs nothing;
    // getting the rule wrong costs the bug.
    expect(
      focused(() => {
        const el = document.createElement('input')
        el.type = type
        return el
      })
    ).toBe(true)
  })

  it('is true for a focused element inside a CodeMirror wrapper', () => {
    const wrapper = document.createElement('div')
    wrapper.className = 'CodeMirror'
    const inner = document.createElement('div')
    inner.tabIndex = 0
    wrapper.appendChild(inner)
    document.body.appendChild(wrapper)
    inner.focus()

    expect(isInputFocused()).toBe(true)
  })

  it('is false for a focused non-editing element', () => {
    // A button takes focus but is not a text surface — single-key shortcuts
    // should still fire here, so this must not be over-broad.
    expect(focused(() => document.createElement('button'))).toBe(false)
  })

  it('is false for a contenteditable="false" host', () => {
    expect(
      focused(() => {
        const el = document.createElement('div')
        el.setAttribute('contenteditable', 'false')
        el.tabIndex = 0
        return el
      })
    ).toBe(false)
  })
})
