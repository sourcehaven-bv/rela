import { describe, it, expect, vi } from 'vitest'
import { createToolbar } from './relaToolbar'
import { INLINE_COMMANDS, BLOCK_COMMANDS } from '@/components/forms/milkdown/editorCommands'
import { TABLE_COMMANDS } from '@/components/forms/milkdown/tableCommands'

function make(hasBridge = false) {
  const run = vi.fn()
  const insertRef = vi.fn()
  const toolbar = createToolbar({ run, insertRef }, hasBridge)
  return { toolbar, run, insertRef }
}

function btn(root: HTMLElement, id: string): HTMLButtonElement | null {
  return root.querySelector(`[data-command="${id}"]`)
}

describe('createToolbar', () => {
  it('renders a button for every shared command', () => {
    const { toolbar } = make()
    for (const cmd of [...INLINE_COMMANDS, ...BLOCK_COMMANDS, ...TABLE_COMMANDS]) {
      expect(btn(toolbar.root, cmd.id), cmd.id).not.toBeNull()
    }
  })

  it('labels every button for assistive technology', () => {
    const { toolbar } = make()
    for (const el of toolbar.root.querySelectorAll('.rela-toolbar-button')) {
      expect(el.getAttribute('aria-label')).toBeTruthy()
      expect(el.getAttribute('title')).toBeTruthy()
    }
  })

  it('draws each glyph as inline SVG, so the bundle needs no icon font', () => {
    const { toolbar } = make(true)
    for (const el of toolbar.root.querySelectorAll('.rela-toolbar-button')) {
      const svg = el.querySelector('svg')
      expect(svg, el.getAttribute('data-command') ?? '').not.toBeNull()
      expect(svg!.childElementCount).toBeGreaterThan(0)
    }
  })

  it('runs a command on click', () => {
    const { toolbar, run } = make()
    btn(toolbar.root, 'strong')!.click()
    expect(run).toHaveBeenCalledWith(INLINE_COMMANDS[0])
  })

  it('refuses a command whose button is unavailable', () => {
    // `aria-disabled` keeps the button focusable, so it does not prevent
    // activation the way a native `disabled` would. The handler has to.
    const { toolbar, run } = make()
    toolbar.update(new Set(), new Set(['strong']), false)
    btn(toolbar.root, 'strong')!.click()
    expect(run).not.toHaveBeenCalled()
  })

  it('marks state without ever using the native disabled attribute', () => {
    const { toolbar } = make()
    toolbar.update(new Set(['emphasis']), new Set(['h1']), false)
    const em = btn(toolbar.root, 'emphasis')!
    expect(em.classList.contains('is-active')).toBe(true)
    expect(em.getAttribute('aria-pressed')).toBe('true')
    const h1 = btn(toolbar.root, 'h1')!
    expect(h1.classList.contains('is-unavailable')).toBe(true)
    expect(h1.getAttribute('aria-disabled')).toBe('true')
    // A native `disabled` control cannot be focused, so disabling the button the
    // user is currently tabbed to would drop focus to <body> mid-interaction.
    expect(h1.hasAttribute('disabled')).toBe(false)
  })

  it('clears state that no longer applies', () => {
    const { toolbar } = make()
    toolbar.update(new Set(['strong']), new Set(['h1']), false)
    toolbar.update(new Set(), new Set(), false)
    expect(btn(toolbar.root, 'strong')!.classList.contains('is-active')).toBe(false)
    expect(btn(toolbar.root, 'h1')!.getAttribute('aria-disabled')).toBe('false')
  })

  it('shows the table group only inside a table', () => {
    // Hidden rather than disabled: seven permanently-greyed buttons is a lot of
    // dead chrome, and unlike the block commands these have no meaning outside
    // a table to hint at.
    const { toolbar } = make()
    const group = btn(toolbar.root, 'deleteRow')!.parentElement as HTMLElement
    expect(group.hidden).toBe(true)
    toolbar.update(new Set(), new Set(), true)
    expect(group.hidden).toBe(false)
    toolbar.update(new Set(), new Set(), false)
    expect(group.hidden).toBe(true)
  })

  it('omits the entity-reference button without a bridge to search through', () => {
    expect(btn(make(false).toolbar.root, 'entityRef')).toBeNull()
    expect(btn(make(true).toolbar.root, 'entityRef')).not.toBeNull()
  })

  it('opens the reference picker from its button', () => {
    const { toolbar, insertRef } = make(true)
    btn(toolbar.root, 'entityRef')!.click()
    expect(insertRef).toHaveBeenCalledOnce()
  })

  it('prevents the mousedown default so a press cannot steal the selection', () => {
    // Without this the selection collapses before the command runs, and
    // formatting a selection from the toolbar becomes impossible.
    const { toolbar } = make()
    const ev = new MouseEvent('mousedown', { bubbles: true, cancelable: true })
    btn(toolbar.root, 'strong')!.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(true)
  })

  it('detaches its DOM on destroy', () => {
    const { toolbar } = make()
    document.body.appendChild(toolbar.root)
    toolbar.destroy()
    expect(toolbar.root.parentNode).toBeNull()
  })
})
