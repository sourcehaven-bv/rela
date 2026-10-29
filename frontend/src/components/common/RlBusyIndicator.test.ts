import { afterEach, describe, it, expect, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import RlBusyIndicator from 'rela-components/components/feedback/RlBusyIndicator.vue'
import source from 'rela-components/components/feedback/RlBusyIndicator.vue?raw'

// The busy indicator lives in rela-components, whose own vitest runs only
// stories. These checks need the component source or fake timers, so they
// run here.

// The minute clock's timer is shared across indicators, so every mount must
// end before the next test or a real timer would outlive its test.
enableAutoUnmount(afterEach)

const HOUR = 3_600_000

describe('RlBusyIndicator', () => {
  const entries = [{ typeLabel: 'Agent run', label: 'Planning', since: new Date().toISOString() }]

  it('hides the dots and the static icon from screen readers', () => {
    const wrapper = mount(RlBusyIndicator, { props: { entries } })
    const dots = wrapper.find('.rl-busy__dots')
    const icon = wrapper.find('.rl-busy__icon')
    expect(dots.exists()).toBe(true)
    expect(icon.exists()).toBe(true)
    expect(dots.attributes('aria-hidden')).toBe('true')
    expect(icon.attributes('aria-hidden')).toBe('true')
  })

  it('hides the dots and stops their animation when motion is reduced', () => {
    const media = /@media \(prefers-reduced-motion: reduce\) \{([\s\S]*?)\n\}/.exec(source)
    expect(media).not.toBeNull()
    const body = media![1]
    expect(body).toMatch(/\.rl-busy__dots \{\s*display: none;/)
    expect(body).toMatch(/\.rl-busy__dot \{\s*animation: none;/)
    expect(body).toMatch(/\.rl-busy__icon \{\s*display: inline-flex;/)
  })

  it('renders nothing without entries', () => {
    const wrapper = mount(RlBusyIndicator, { props: { entries: [] } })
    expect(wrapper.find('.rl-busy').exists()).toBe(false)
  })
})

// The time in the state is the stall signal, so it must move on while the
// board stays open, with no new props.
describe('RlBusyIndicator time', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('moves on as time passes, from one shared timer that stops on unmount', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-29T12:00:00Z'))
    const since = new Date(Date.now() - 3 * HOUR).toISOString()
    const entries = [{ typeLabel: 'Agent run', label: 'Planning', since }]
    const a = mount(RlBusyIndicator, { props: { entries } })
    const b = mount(RlBusyIndicator, { props: { entries } })
    expect(a.find('.rl-busy__text').text()).toBe('Planning · 3h')
    expect(vi.getTimerCount()).toBe(1)

    vi.advanceTimersByTime(2 * HOUR)
    await nextTick()
    expect(a.find('.rl-busy__text').text()).toBe('Planning · 5h')
    expect(a.find('.rl-visually-hidden').text()).toBe('Agent run: Planning, for 5 hours')
    expect(b.find('.rl-busy__text').text()).toBe('Planning · 5h')

    a.unmount()
    expect(vi.getTimerCount()).toBe(1)
    b.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
