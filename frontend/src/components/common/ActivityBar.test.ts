import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import ActivityBar from './ActivityBar.vue'

async function tick(ms: number) {
  vi.advanceTimersByTime(ms)
  await nextTick()
}

// The bar is "visible" via a modifier class rather than v-if: it must keep
// its (fixed-position, zero-flow) box so the opacity transition can run.
// So these assert the class, not presence in the DOM.
function isVisible(wrapper: ReturnType<typeof mount>) {
  return wrapper.get('[data-testid="activity-bar"]').classes().includes('activity-bar--visible')
}

describe('ActivityBar', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('stays hidden for a navigation that resolves before the delay', async () => {
    const wrapper = mount(ActivityBar, { props: { active: false } })
    await wrapper.setProps({ active: true })

    await tick(100)
    await wrapper.setProps({ active: false })
    await tick(2000)

    expect(isVisible(wrapper)).toBe(false)
  })

  it('appears after the navigation delay and holds for the minimum', async () => {
    const wrapper = mount(ActivityBar, { props: { active: false } })
    await wrapper.setProps({ active: true })

    await tick(249)
    expect(isVisible(wrapper)).toBe(false)

    await tick(1)
    expect(isVisible(wrapper)).toBe(true)

    // Settles at 350ms — 100ms into the 300ms minimum.
    await tick(100)
    await wrapper.setProps({ active: false })
    await nextTick()
    expect(isVisible(wrapper)).toBe(true)

    await tick(200)
    expect(isVisible(wrapper)).toBe(false)
  })

  // The bar stays mounted so its opacity can animate; replacing this with
  // v-if would make the fade a no-op.
  it('remains in the DOM when idle so the fade can run', () => {
    const wrapper = mount(ActivityBar, { props: { active: false } })
    expect(wrapper.find('[data-testid="activity-bar"]').exists()).toBe(true)
    expect(isVisible(wrapper)).toBe(false)
  })

  // A route change is announced by the destination view's own content; a
  // live region here would double up on every navigation.
  it('is hidden from assistive technology', () => {
    const wrapper = mount(ActivityBar, { props: { active: false } })
    expect(wrapper.get('[data-testid="activity-bar"]').attributes('aria-hidden')).toBe('true')
  })
})
