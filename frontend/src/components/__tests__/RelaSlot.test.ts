import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import RelaSlotHost from './fixtures/RelaSlotHost.vue'

/**
 * Pins AC4 of TKT-3DBK6I: an unregistered `<rela-slot>` renders inert and
 * emits no Vue warning.
 *
 * ⚠ This MUST be an SFC fixture, not a runtime string `template`.
 * Runtime-compiled templates never see build-time `compilerOptions`, so a
 * string-template version of this test reports the warning as still present
 * even when `isCustomElement` is configured correctly — a false negative that
 * already cost one debugging cycle during planning.
 */
describe('<rela-slot> (tier-1 operator hook)', () => {
  it('renders without an unresolved-component warning', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const wrapper = mount(RelaSlotHost)
    const warnings = warn.mock.calls.map((c) => String(c[0])).join('\n')
    warn.mockRestore()

    expect(warnings).not.toContain('Failed to resolve component')
    expect(wrapper.html()).toContain('rela-slot')
  })

  it('is inert: no content of its own when nothing defines it', () => {
    const wrapper = mount(RelaSlotHost)
    const slot = wrapper.find('rela-slot')
    expect(slot.exists()).toBe(true)
    expect(slot.text()).toBe('')
  })

  it('passes attributes through so a definition can read them', () => {
    // Vue → operator: rela sets attributes; the element's
    // attributeChangedCallback fires on change.
    const slot = mount(RelaSlotHost).find('rela-slot')
    expect(slot.attributes('name')).toBe('companion')
    expect(slot.attributes('data-band')).toBe('stalled')
  })
})
