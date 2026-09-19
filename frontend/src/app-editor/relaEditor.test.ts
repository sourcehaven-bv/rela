import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// These tests exercise the ELEMENT's public contract — the swap seam
// (value/placeholder/readonly/events/focus/teardown) — against the REAL editor.
//
// The EasyMDE version of this file mocked the editor away, because CodeMirror 5
// needs real layout and does not mount under happy-dom. Milkdown/ProseMirror
// does mount, so the mock is gone: every assertion below runs against the editor
// an app actually gets, which is what makes them worth having.
//
// Edits are made through the TOOLBAR rather than by reaching for the
// ProseMirror view. That is not a workaround: the element deliberately exposes
// no editor handle, since one would widen the very contract these tests pin, and
// a toolbar button is a real user action end to end. What happy-dom cannot do is
// lay anything out, so nothing here asserts geometry.

import './relaEditor'

interface RelaEditorEl extends HTMLElement {
  value: string
}

function makeEditor(): RelaEditorEl {
  return document.createElement('rela-editor') as RelaEditorEl
}

/**
 * Waits for the deferred mount and the editor's async creation.
 *
 * `connectedCallback` defers to a microtask so the mount does not block the
 * parse path, and `Editor.create()` is itself a promise chain. One turn of the
 * macrotask queue drains both.
 */
const settle = (): Promise<void> => new Promise((r) => setTimeout(r, 0))

function prose(el: HTMLElement): HTMLElement | null {
  return el.querySelector('.ProseMirror')
}

function button(el: HTMLElement, command: string): HTMLButtonElement | null {
  return el.querySelector(`[data-command="${command}"]`)
}

/** Makes a real edit: the same click an app's user would make. */
function edit(el: HTMLElement, command = 'bulletList'): void {
  const b = button(el, command)
  if (!b) throw new Error(`no toolbar button for ${command}`)
  b.click()
}

/**
 * Selects a word in the editor.
 *
 * Goes through the DOM selection the view reads, which is what a user's drag
 * produces. happy-dom does not deliver a real selection to ProseMirror, so this
 * also nudges the view to resync from the DOM.
 */
function selectWord(el: HTMLElement, word: string): void {
  const surface = prose(el)
  if (!surface) throw new Error('editor surface not mounted')
  const para = surface.querySelector('p')
  const textNode = para?.firstChild
  if (!textNode || textNode.textContent === null) throw new Error('no text to select')
  const offset = textNode.textContent.indexOf(word)
  if (offset < 0) throw new Error(`"${word}" not in the document`)
  const range = document.createRange()
  range.setStart(textNode, offset)
  range.setEnd(textNode, offset + word.length)
  const sel = window.getSelection()
  sel?.removeAllRanges()
  sel?.addRange(range)
  surface.dispatchEvent(new Event('focus'))
}

/** Sends Escape the way the editor's capture-phase handler receives it. */
function pressEscape(el: HTMLElement): void {
  const surface = prose(el)
  if (!surface) throw new Error('editor surface not mounted')
  surface.dispatchEvent(
    new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
  )
}

describe('<rela-editor> contract', () => {
  beforeEach(() => {
    document.body.replaceChildren()
    delete (window as { rela?: unknown }).rela
  })
  afterEach(() => {
    document.body.replaceChildren()
  })

  it('registers the custom element', () => {
    expect(customElements.get('rela-editor')).toBeTruthy()
  })

  it('flushes a value set BEFORE connection into the editor', async () => {
    const ed = makeEditor()
    ed.value = '# Before connect\n'
    document.body.appendChild(ed)
    await settle()
    expect(ed.value).toBe('# Before connect\n')
    expect(prose(ed)?.textContent).toContain('Before connect')
  })

  it('round-trips value set AFTER connection', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = '## After\n'
    expect(ed.value).toBe('## After\n')
    expect(prose(ed)?.textContent).toContain('After')
  })

  it('preserves an UNEDITED body byte for byte, churn and all', async () => {
    // The write-back guard sits on the getter. A WYSIWYG round-trip reformats
    // (a table re-padded, a setext heading rewritten to ATX), and an app that
    // only displayed a body must get its own bytes back rather than the
    // editor's rendering of them.
    //
    // Every element here is one the round-trip WOULD rewrite, so this fails if
    // the guard is bypassed rather than passing on a body that happens to be
    // already-normalized.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const md =
      'Intro.\n\n* one\n* two\n\n| a  | b   |\n| -- | --- |\n| 1  | 2   |\n\nSetext\n======\n'
    ed.value = md
    expect(ed.value).toBe(md)
  })

  it('reserializes the WHOLE body once the user edits any of it', async () => {
    // The other half of the contract, and a real behaviour change from the
    // EasyMDE editor this replaced: that one was a plain text buffer and handed
    // back exactly what was typed. Editing a parsed document cannot do that —
    // one command anywhere reformats everything the serializer normalizes.
    //
    // Asserted rather than merely accepted, so the day it changes is a failing
    // test and not an app author's surprise. It is documented in the
    // custom-apps guide for the same reason.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'Intro.\n\n| a  | b   |\n| -- | --- |\n| 1  | 2   |\n\nSetext\n======\n'
    // Touches the LAST block only.
    edit(ed, 'h2')
    const after = ed.value
    expect(after).toContain('## Setext')
    // The table was not touched by the user, and is repadded anyway.
    expect(after).toContain('| a | b |')
    // Meaning is preserved throughout — this is reformatting, never a rewrite.
    expect(after).toContain('Intro.')
    expect(after).toContain('| 1 | 2 |')
  })

  it('returns the edited markdown once the user has actually changed something', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'plain\n'
    edit(ed)
    expect(ed.value).toBe('- plain\n\n')
  })

  it('does NOT emit input when value is set programmatically', async () => {
    // A native <textarea>.value setter is silent; <rela-editor> must match, else
    // loading content into the editor triggers autosave loops in consumers (the
    // bug that froze the Today app's goals load).
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const onInput = vi.fn()
    ed.addEventListener('input', onInput)
    ed.value = 'programmatic content\n'
    expect(onInput).not.toHaveBeenCalled()
    // A genuine user edit still fires input.
    edit(ed)
    expect(onInput).toHaveBeenCalled()
  })

  it('dispatches input immediately, carrying the value AFTER the edit', async () => {
    // Milkdown's markdown listener is debounced by 200ms. Wiring `input` to it
    // would drop a change made and read inside that window, which is exactly
    // what an app saving on submit does. It also must not fire mid-transaction:
    // an earlier version did, and every listener read the document as it was
    // BEFORE the edit.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'plain\n'
    const seen: string[] = []
    ed.addEventListener('input', () => seen.push(ed.value))
    edit(ed)
    expect(seen.length).toBeGreaterThan(0)
    expect(new Set(seen)).toEqual(new Set(['- plain\n\n']))
  })

  it('dispatches change on blur after an edit', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const onChange = vi.fn()
    ed.addEventListener('change', onChange)
    const dom = prose(ed)!
    dom.dispatchEvent(new Event('focus'))
    edit(ed)
    dom.dispatchEvent(new Event('blur'))
    expect(onChange).toHaveBeenCalledOnce()
  })

  it('still dispatches change when the only edit came from the toolbar', async () => {
    // Every command returns focus to the document, which re-fires `focus`. An
    // earlier version re-snapshotted the comparison baseline there, so a user
    // who formatted something from the toolbar and clicked away got no `change`
    // at all — their edit never reached an app wiring autosave to it.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const onChange = vi.fn()
    ed.addEventListener('change', onChange)
    const dom = prose(ed)!
    dom.dispatchEvent(new Event('focus'))
    edit(ed)
    // The command refocuses; the real browser does this too, because the button
    // prevents its mousedown default precisely so focus never leaves.
    dom.dispatchEvent(new Event('focus'))
    dom.dispatchEvent(new Event('blur'))
    expect(onChange).toHaveBeenCalledOnce()
  })

  it('does NOT dispatch change on blur when nothing was edited', async () => {
    // Native <textarea> semantics: a click-away must not trigger autosave.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const onChange = vi.fn()
    ed.addEventListener('change', onChange)
    const dom = prose(ed)!
    dom.dispatchEvent(new Event('focus'))
    dom.dispatchEvent(new Event('blur'))
    expect(onChange).not.toHaveBeenCalled()
  })

  it('applies readonly via attribute at mount and on change', async () => {
    const ed = makeEditor()
    ed.setAttribute('readonly', '')
    document.body.appendChild(ed)
    await settle()
    expect(prose(ed)?.getAttribute('contenteditable')).toBe('false')
    ed.removeAttribute('readonly')
    expect(prose(ed)?.getAttribute('contenteditable')).toBe('true')
  })

  it('refuses a toolbar command while readonly', async () => {
    const ed = makeEditor()
    ed.setAttribute('readonly', '')
    document.body.appendChild(ed)
    await settle()
    ed.value = 'plain\n'
    edit(ed)
    expect(ed.value).toBe('plain\n')
  })

  it('shows the placeholder only while the document is empty', async () => {
    const ed = makeEditor()
    ed.setAttribute('placeholder', 'Write something')
    document.body.appendChild(ed)
    await settle()
    const ph = ed.querySelector('.rela-editor-placeholder') as HTMLElement
    expect(ph.textContent).toBe('Write something')
    expect(ph.hidden).toBe(false)
    ed.value = 'now it has content\n'
    expect(ph.hidden).toBe(true)
  })

  it('honours an explicitly empty placeholder', async () => {
    // `placeholder=""` asks for NO placeholder, as on a native <textarea>. An
    // earlier version used `||` and handed back the default instead.
    const ed = makeEditor()
    ed.setAttribute('placeholder', '')
    document.body.appendChild(ed)
    await settle()
    expect((ed.querySelector('.rela-editor-placeholder') as HTMLElement).textContent).toBe('')
  })

  it('takes back a reference trigger the toolbar typed, when the user dismisses it', async () => {
    // The button types `@` to open the menu, which is a document change. Without
    // withdrawing it, pressing the button and changing your mind leaves a stray
    // `@` behind — and the cursor is wherever the user left it, so it can land
    // mid-sentence or inside a table cell.
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'Intro.\n'
    button(ed, 'entityRef')!.click()
    expect(ed.value).toContain('@')

    pressEscape(ed)
    expect(ed.value).not.toContain('@')
    expect(ed.value).toContain('Intro.')
  })

  it('withdraws the trigger even when the menu never opened', async () => {
    // The dismissal path must not be gated on the menu being on screen: that is
    // precisely the case where a stray `@` would be stranded with nothing
    // visible to explain it.
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'Intro.\n'
    button(ed, 'entityRef')!.click()
    // Nothing has driven the provider, so no menu is showing.
    expect(ed.querySelector('.rela-mention-menu')?.childElementCount ?? 0).toBe(0)
    pressEscape(ed)
    expect(ed.value).not.toContain('@')
  })

  it('leaves an unedited body unedited when a prompt is opened and cancelled', async () => {
    // The whole point of withdrawing the trigger. An earlier version removed the
    // `@` but left the document marked as edited, so `.value` still returned a
    // fully reserialized body — the exact churn the write-back guard exists to
    // prevent, defeated by one press of a button whose effect was undone.
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    // Constructs the round-trip WOULD rewrite, so this fails if the guard is off.
    const src = 'Intro.\n\n| a  | b   |\n| -- | --- |\n| 1  | 2   |\n'
    ed.value = src
    button(ed, 'entityRef')!.click()
    pressEscape(ed)
    expect(ed.value).toBe(src)
  })

  it('restores a table-ending body too, when the document really did revert', async () => {
    // The case the comparison exists to decide. Withdrawal restores the
    // edited-state only when the editor's output matches what it produced just
    // before the trigger — never on an argument about which plugins ran, because
    // a leftover empty paragraph is invisible in rendered text and only the
    // serialization can see it.
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const src = 'Intro.\n\n| a  | b   |\n| -- | --- |\n| 1  | 2   |\n'
    ed.value = src
    button(ed, 'entityRef')!.click()
    pressEscape(ed)
    expect(ed.value).toBe(src)
  })

  it('does not swallow the selection when the reference button is pressed', async () => {
    // `insertText(text, from, to)` REPLACES the range, so passing the live
    // selection deleted whatever the user had highlighted. Pressing an *insert*
    // button must never destroy text.
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'hello world\n'
    selectWord(ed, 'world')
    button(ed, 'entityRef')!.click()
    expect(ed.value).toContain('world')
  })

  it('keeps the trigger withdrawable when it could not be removed yet', async () => {
    // Readonly is an ordinary thing for an app to set (on save, on lock). An
    // earlier version consumed the pending span on every dismissal attempt, so a
    // dismissal that merely could not run threw away the only record of what to
    // remove — stranding the `@` for the session with no way back.
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'Intro.\n'
    button(ed, 'entityRef')!.click()
    expect(ed.value).toContain('@')

    ed.setAttribute('readonly', '')
    pressEscape(ed)
    expect(ed.value).toContain('@') // could not act, correctly

    ed.removeAttribute('readonly')
    pressEscape(ed)
    expect(ed.value).not.toContain('@') // and the span survived to be used
  })

  it('forgets a pending trigger when the app loads a different body', async () => {
    // A span records positions in the document that produced it. Carried across
    // a new body it points into a different one, and a body that happens to have
    // ` @` at the same offset would lose two characters.
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'Intro.\n'
    button(ed, 'entityRef')!.click()
    const replacement = 'a b c @ d\n'
    ed.value = replacement
    pressEscape(ed)
    expect(ed.value).toBe(replacement)
  })

  it('withdraws a trigger only once, so a second Escape is inert', async () => {
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'Intro.\n'
    button(ed, 'entityRef')!.click()
    pressEscape(ed)
    const after = ed.value
    pressEscape(ed)
    expect(ed.value).toBe(after)
  })

  it('focus() moves focus into the editing surface', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.focus()
    expect(document.activeElement).toBe(prose(ed))
  })

  it('tears the editor down on disconnect and preserves value across re-connect', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'keep me\n'
    ed.remove()
    // Everything the element built is gone, so a detached editor cannot keep
    // holding listeners or a ProseMirror view.
    expect(ed.querySelector('.ProseMirror')).toBeNull()
    expect(ed.querySelector('.rela-editor-shell')).toBeNull()

    document.body.appendChild(ed)
    await settle()
    expect(ed.value).toBe('keep me\n')
    expect(prose(ed)?.textContent).toContain('keep me')
  })

  it('carries an edit across a disconnect/reconnect round-trip', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'plain\n'
    edit(ed)
    ed.remove()
    document.body.appendChild(ed)
    await settle()
    expect(ed.value).toBe('- plain\n\n')
  })

  it('renders a toolbar of formatting buttons, with the table group hidden', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const toolbar = ed.querySelector('.rela-toolbar')!
    const buttons = toolbar.querySelectorAll('.rela-toolbar-button')
    expect(buttons.length).toBeGreaterThan(8)
    // Every button draws an inline SVG: the bundle ships no icon font, so a
    // missing glyph would mean an empty button rather than a tofu box.
    expect(toolbar.querySelectorAll('.rela-toolbar-button svg').length).toBe(buttons.length)
    const groups = toolbar.querySelectorAll('.rela-toolbar-group')
    const tableGroup = groups[groups.length - 1] as HTMLElement
    expect(tableGroup.hidden).toBe(true)
  })

  it('reflects the formatting at the cursor on the toolbar', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = '# Heading\n'
    expect(button(ed, 'h1')!.classList.contains('is-active')).toBe(true)
    expect(button(ed, 'h2')!.classList.contains('is-active')).toBe(false)
  })

  it('disables a command that would do nothing where the cursor is', async () => {
    // A heading cannot be applied inside a list item. A button that looks
    // pressable and no-ops gives the user nothing to reason about.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = '- item\n'
    expect(button(ed, 'h1')!.getAttribute('aria-disabled')).toBe('true')
    // Kept in the tab order: a native `disabled` would drop focus to <body> if
    // the cursor moved into a context that disabled the focused button.
    expect(button(ed, 'h1')!.hasAttribute('disabled')).toBe(false)
  })

  it('toggles a block command back off when it is already active', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'plain\n'
    edit(ed, 'h1')
    expect(ed.value).toBe('# plain\n')
    edit(ed, 'h1')
    expect(ed.value).toBe('plain\n')
  })

  it('omits the entity-reference button when the app bridge is absent', async () => {
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    expect(button(ed, 'entityRef')).toBeNull()
  })

  it('offers the entity-reference button when the bridge is present', async () => {
    ;(window as { rela?: unknown }).rela = { search: vi.fn().mockResolvedValue({ data: [] }) }
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    expect(button(ed, 'entityRef')).not.toBeNull()
  })

  it('serializes an entity reference back to the code span it came from', async () => {
    // The storage format does not change: a reference is `` `ID` `` in markdown,
    // whatever the editor draws. An app reading .value must see that.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const src = 'See `TKT-ABC` for detail.\n'
    ed.value = src
    expect(ed.querySelector('a[data-entity-ref]')).not.toBeNull()
    expect(ed.value).toBe(src)
  })

  it('renders an entity reference as its bare ID, with no title and no link target', async () => {
    // The app bridge has no per-principal mentions endpoint, so there is no
    // title the editor may show without routing around the read gate.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    ed.value = 'See `TKT-ABC`.\n'
    const ref = ed.querySelector('a[data-entity-ref]') as HTMLElement
    expect(ref.textContent).toBe('TKT-ABC')
    expect(ref.getAttribute('data-entity-title')).toBeNull()
    expect(ref.getAttribute('href')).toBeNull()
  })

  it('says so when the stylesheet fails to load', async () => {
    // A missing stylesheet leaves a fully functional but completely unstyled
    // editor, which reads as a rendering bug rather than a missing file — and
    // the likeliest cause (a checkout where the frontend build has not run)
    // gives no other signal at all. The editor keeps working on purpose: an app
    // that can still capture text beats one that cannot.
    const ed = makeEditor()
    document.body.appendChild(ed)
    await settle()
    const link = document.head.querySelector('#rela-editor-styles')
    expect(link).not.toBeNull()
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    link!.dispatchEvent(new Event('error'))
    expect(spy).toHaveBeenCalledOnce()
    expect(String(spy.mock.calls[0][0])).toContain('_rela-editor.css')
    spy.mockRestore()
  })

  it('links its stylesheet once, however many editors mount', async () => {
    // A <style> element would be blocked outright by the app CSP, which carries
    // no 'unsafe-inline'; the editor would render completely unstyled with only
    // a console violation to show for it.
    const a = makeEditor()
    const b = makeEditor()
    document.body.append(a, b)
    await settle()
    const links = document.head.querySelectorAll('link[rel="stylesheet"]#rela-editor-styles')
    expect(links.length).toBe(1)
    expect(links[0].getAttribute('href')).toBe('_rela-editor.css')
    expect(document.head.querySelector('style')).toBeNull()
  })
})

// Comment chips in the sandboxed element (TKT-T0QP4Y).
//
// The node and its remark plugin now load from `RELA_OUTPUT_NODES` in the
// shared preset, so both editors get them from one place. That is the fix for a
// review finding, and it needs a test on THIS side: the whole risk the shared
// preset exists to remove is the two editors writing different bytes, and a
// test that only ever runs the SPA cannot see that happen.
describe('rela-editor comment chips', () => {
  it('renders an HTML comment as a chip, not as raw text', async () => {
    const ed = makeEditor()
    ed.value = '<!-- Document what IS and IS NOT in scope -->\n'
    document.body.appendChild(ed)
    await settle()

    const surface = prose(ed)
    const chip = surface?.querySelector('.rela-comment')
    expect(chip).not.toBeNull()
    expect(chip?.textContent).toBe('Document what IS and IS NOT in scope')
    expect(surface?.textContent).not.toContain('<!--')
  })

  // The property the shared preset exists to guarantee. `.value` reads through
  // `guardWriteBack`, which is the app's save path.
  it('round-trips a comment unchanged through the value getter', async () => {
    const src = '## Options\n\n<!-- For each option:\n- **Pros**: good\n-->\n'
    const ed = makeEditor()
    ed.value = src
    document.body.appendChild(ed)
    await settle()

    expect(ed.value).toContain('<!-- For each option:\n- **Pros**: good\n-->')
  })

  // Raw markup must stay visible here too: the app editor renders untrusted
  // content in a page an app controls, so a chip hiding live markup would be
  // worse here than in the SPA, not better.
  it('leaves non-comment raw HTML visible as text', async () => {
    const ed = makeEditor()
    ed.value = 'Body <img src=x onerror="alert(1)"> tail\n'
    document.body.appendChild(ed)
    await settle()

    const surface = prose(ed)
    expect(surface?.querySelector('.rela-comment')).toBeNull()
    expect(surface?.querySelector('img')).toBeNull()
    expect(surface?.textContent).toContain('onerror')
  })
})
