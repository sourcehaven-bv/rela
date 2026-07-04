// Unit tests for AutoSaveIndicator — covers the hidden-until-needed
// behaviour (TKT-U62DVR): idle is invisible, saving/saved/error are
// visible, and the wrapper stays in the DOM either way so the
// saved → idle opacity transition can run and aria-live still announces.

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
    // Stays in the DOM (for the transition + aria-live) but marked hidden.
    expect(el.classes()).toContain('autosave-hidden')
    expect(el.attributes('data-visible')).toBe('false')
  })

  it('is visible while saving', () => {
    const wrapper = mountIndicator({ status: 'saving' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.classes()).not.toContain('autosave-hidden')
    expect(el.attributes('data-visible')).toBe('true')
    expect(el.attributes('data-status')).toBe('saving')
    // Spinner glyph.
    expect(wrapper.find('.autosave-spin').exists()).toBe(true)
  })

  it('is visible in the saved state (shown briefly before upstream idle)', () => {
    const wrapper = mountIndicator({ status: 'saved' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.classes()).not.toContain('autosave-hidden')
    expect(el.attributes('data-visible')).toBe('true')
    expect(el.classes()).toContain('autosave-saved')
  })

  it('stays visible on error and does not fade out', () => {
    const wrapper = mountIndicator({ status: 'error', error: 'Save failed' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.classes()).not.toContain('autosave-hidden')
    expect(el.classes()).toContain('autosave-error')
    expect(el.attributes('data-visible')).toBe('true')
  })

  it('shows the error state (and stays visible) even when status is idle but an error is present', () => {
    // An error present with a non-error status must still surface as error
    // and remain visible — error wins over idle-hidden.
    const wrapper = mountIndicator({ status: 'idle', error: 'boom' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.classes()).toContain('autosave-error')
    expect(el.classes()).not.toContain('autosave-hidden')
    expect(el.attributes('title')).toBe('boom')
  })

  it('keeps role=status / aria-live for screen readers', () => {
    const wrapper = mountIndicator({ status: 'idle' })
    const el = wrapper.get('[data-testid="autosave-indicator"]')
    expect(el.attributes('role')).toBe('status')
    expect(el.attributes('aria-live')).toBe('polite')
  })
})
