// Unit tests for AutoSaveIndicator — covers the hidden-until-needed
// behaviour (TKT-U62DVR): idle is invisible, saving/saved/error are
// visible, the wrapper stays in the DOM either way so the saved → idle
// opacity transition can run, and the visually-hidden live region carries
// the screen-reader announcement (empty at idle).

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AutoSaveIndicator from './AutoSaveIndicator.vue'
import type { SaveStatus } from '@/composables/useAutoSave'

function mountIndicator(props: { status: SaveStatus; error?: string | null }) {
  return mount(AutoSaveIndicator, { props })
}

describe('AutoSaveIndicator visibility', () => {
  it('is hidden (faded out) when idle', () => {
    const wrapper = mountIndicator({ status: 'idle' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    // Stays in the DOM (for the transition) but marked hidden.
    expect(el.classes()).toContain('autosave-hidden')
  })

  it('is visible while saving', () => {
    const wrapper = mountIndicator({ status: 'saving' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.classes()).not.toContain('autosave-hidden')
    expect(el.attributes('data-status')).toBe('saving')
    // Spinner glyph.
    expect(wrapper.find('.autosave-spin').exists()).toBe(true)
  })

  it('is visible in the saved state (shown briefly before upstream idle)', () => {
    const wrapper = mountIndicator({ status: 'saved' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.classes()).not.toContain('autosave-hidden')
    expect(el.classes()).toContain('autosave-saved')
  })

  it('stays visible on error and does not fade out', () => {
    const wrapper = mountIndicator({ status: 'error', error: 'Save failed' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.classes()).not.toContain('autosave-hidden')
    expect(el.classes()).toContain('autosave-error')
  })

  it('shows the error state (and stays visible) even when status is idle but an error is present', () => {
    // An error present with a non-error status must still surface as error
    // and remain visible — error wins over idle-hidden.
    const wrapper = mountIndicator({ status: 'idle', error: 'boom' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.classes()).toContain('autosave-error')
    expect(el.classes()).not.toContain('autosave-hidden')
    // Tooltip surfaces the raw error detail on hover.
    expect(el.attributes('title')).toBe('boom')
  })
})

describe('AutoSaveIndicator screen-reader announcement', () => {
  // The glyphs are aria-hidden SVGs; state is announced via the live
  // region's TEXT CONTENT (RR-4SN00Y).
  it('exposes a polite live region', () => {
    const sr = mountIndicator({ status: 'idle' }).get('.autosave-sr-only')
    expect(sr.attributes('role')).toBe('status')
    expect(sr.attributes('aria-live')).toBe('polite')
  })

  it('announces nothing at idle (empty live region — no mount/settle noise)', () => {
    const sr = mountIndicator({ status: 'idle' }).get('.autosave-sr-only')
    expect(sr.text()).toBe('')
  })

  it('announces the state while saving / saved / error', () => {
    expect(mountIndicator({ status: 'saving' }).get('.autosave-sr-only').text()).toBe('Saving…')
    expect(mountIndicator({ status: 'saved' }).get('.autosave-sr-only').text()).toBe('Saved')
    expect(
      mountIndicator({ status: 'error', error: 'nope' }).get('.autosave-sr-only').text(),
    ).toBe('Save failed')
  })

  it('the visual glyph wrapper is aria-hidden (not double-announced)', () => {
    const el = mountIndicator({ status: 'saving' }).get('[data-testid="autosave-indicator"]')
    expect(el.attributes('aria-hidden')).toBe('true')
  })
})
