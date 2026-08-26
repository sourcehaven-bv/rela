import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import PendingButton from './PendingButton.vue'

async function tick(ms: number) {
  vi.advanceTimersByTime(ms)
  await nextTick()
}

type ButtonProps = InstanceType<typeof PendingButton>['$props']

function mountButton(props: Partial<ButtonProps> = {}) {
  return mount(PendingButton, {
    props: { pending: false, label: 'Save', pendingLabel: 'Saving…', ...props },
  })
}

describe('PendingButton', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  // AC1 — the governing rule at the button. A fast save must leave the
  // button completely untouched; the click is already acknowledged by the
  // browser's own :active state.
  it('does not change the label when the action resolves before the delay', async () => {
    const wrapper = mountButton()
    await wrapper.setProps({ pending: true })
    await tick(100)
    await wrapper.setProps({ pending: false })
    await tick(2000)

    // data-pending is the rendered marker for "an indicator is showing";
    // absent means the button never changed at all.
    expect(wrapper.attributes('data-pending')).toBeUndefined()
    expect(wrapper.find('[data-pending]').exists()).toBe(false)
  })

  it('swaps the label after the delay and holds it for the minimum', async () => {
    const wrapper = mountButton()
    await wrapper.setProps({ pending: true })

    await tick(499)
    expect(wrapper.attributes('data-pending')).toBeUndefined()

    await tick(1)
    expect(wrapper.attributes('data-pending')).toBe('true')

    // Settles at 600ms, only 100ms into the 400ms minimum.
    await tick(100)
    await wrapper.setProps({ pending: false })
    await nextTick()
    expect(wrapper.attributes('data-pending')).toBe('true')

    await tick(300)
    expect(wrapper.attributes('data-pending')).toBeUndefined()
  })

  // AC3 — the width-reservation contract. jsdom has no layout, so assert
  // the structural invariants that make the CSS work: both labels present,
  // the inactive one hidden by visibility (which keeps its box) rather
  // than removed from the DOM.
  it('always renders both labels so the button cannot resize', async () => {
    const wrapper = mountButton()
    const labels = () => wrapper.findAll('.pending-button__label')

    expect(labels()).toHaveLength(2)
    expect(labels()[0].text()).toBe('Save')
    expect(labels()[1].text()).toBe('Saving…')
    // At rest the pending label is the hidden one.
    expect(labels()[0].classes()).not.toContain('pending-button__label--hidden')
    expect(labels()[1].classes()).toContain('pending-button__label--hidden')

    await wrapper.setProps({ pending: true })
    await tick(500)

    // While pending both are STILL in the DOM — only which one is hidden
    // has flipped. If a future change swaps this for v-if, the reserved
    // width collapses and this fails.
    expect(labels()).toHaveLength(2)
    expect(labels()[0].classes()).toContain('pending-button__label--hidden')
    expect(labels()[1].classes()).not.toContain('pending-button__label--hidden')
  })

  // AC9 / RR-R5VL59 — aria-disabled keeps the control focusable, so
  // suppression is the component's job, for pointer AND keyboard.
  it('uses aria-disabled rather than native disabled while pending', async () => {
    const wrapper = mountButton({ pending: true })
    await tick(500)

    expect(wrapper.attributes('aria-disabled')).toBe('true')
    // Native disabled would drop focus to <body> mid-interaction.
    expect(wrapper.attributes('disabled')).toBeUndefined()
  })

  it('suppresses click while pending', async () => {
    const wrapper = mountButton({ pending: true })
    await tick(500)

    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toBeUndefined()
  })

  // The pre-delay window is the dangerous one: nothing is on screen yet,
  // so a user may well click again. On a destructive action that would be
  // a second DELETE.
  it('suppresses click during the pre-delay window, before anything is shown', async () => {
    const wrapper = mountButton({ pending: true })
    await tick(100)

    expect(wrapper.attributes('data-pending')).toBeUndefined()
    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toBeUndefined()
  })

  // Asserts defaultPrevented, NOT the absence of a click event: jsdom
  // never synthesises a click from keydown, so `expect(emitted('click'))
  // .toBeUndefined()` passes even with the whole handler deleted. The
  // default action is the thing being suppressed, so that is what to check.
  it('suppresses keyboard activation while pending', async () => {
    const wrapper = mountButton({ pending: true })
    await tick(500)

    for (const key of ['Enter', ' ']) {
      const event = new KeyboardEvent('keydown', { key, cancelable: true, bubbles: true })
      wrapper.element.dispatchEvent(event)
      expect(event.defaultPrevented, `${key} should be prevented`).toBe(true)
    }
    expect(wrapper.emitted('click')).toBeUndefined()
  })

  it('does not suppress keyboard activation when idle', async () => {
    const wrapper = mountButton()

    const event = new KeyboardEvent('keydown', { key: 'Enter', cancelable: true, bubbles: true })
    wrapper.element.dispatchEvent(event)
    // An idle button must keep its native Enter-activates behaviour.
    expect(event.defaultPrevented).toBe(false)
  })

  // Native disabled already drops focus and makes the control inert, so
  // asserting aria-disabled alongside it would promise a focus-preserving
  // contract the element no longer honours.
  it('does not emit aria-disabled when natively disabled', async () => {
    const wrapper = mountButton({ pending: true, disabled: true })
    await tick(500)

    expect(wrapper.attributes('disabled')).toBeDefined()
    expect(wrapper.attributes('aria-disabled')).toBeUndefined()
  })

  it('emits click when idle', async () => {
    const wrapper = mountButton()
    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toHaveLength(1)
  })

  it('uses native disabled when disabled for non-pending reasons', async () => {
    // An invalid form is not an in-flight operation: there is no focus to
    // preserve, so native disabled is correct there.
    const wrapper = mountButton({ disabled: true })
    expect(wrapper.attributes('disabled')).toBeDefined()

    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toBeUndefined()
  })

  it('announces the pending label through a polite live region', async () => {
    const wrapper = mountButton()
    const sr = () => wrapper.get('.pending-button__sr')

    expect(sr().attributes('role')).toBe('status')
    expect(sr().attributes('aria-live')).toBe('polite')
    // Empty at idle so nothing is announced on mount.
    expect(sr().text()).toBe('')

    await wrapper.setProps({ pending: true })
    await tick(500)
    expect(sr().text()).toBe('Saving…')
  })

  it('does not announce for a sub-delay action', async () => {
    const wrapper = mountButton()
    await wrapper.setProps({ pending: true })
    await tick(100)
    await wrapper.setProps({ pending: false })
    await tick(2000)

    expect(wrapper.get('.pending-button__sr').text()).toBe('')
  })

  it('defaults to type=button so it cannot submit a form by accident', () => {
    expect(mountButton().attributes('type')).toBe('button')
    expect(mountButton({ type: 'submit' }).attributes('type')).toBe('submit')
  })
})
