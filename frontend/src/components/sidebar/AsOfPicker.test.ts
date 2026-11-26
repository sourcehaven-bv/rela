import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { useSchemaStore } from '@/stores/schema'

let asOf: string | undefined
vi.mock('@/api/asOf', () => ({ currentAsOf: () => asOf }))

import AsOfPicker from './AsOfPicker.vue'

function mountPicker() {
  const pinia = createPinia()
  useSchemaStore(pinia).asOfEnabled = true
  return mount(AsOfPicker, { global: { plugins: [pinia] }, attachTo: document.body })
}

describe('AsOfPicker', () => {
  afterEach(() => {
    asOf = undefined
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  it('shows Now over the empty input instead of the browser placeholder', () => {
    const w = mountPicker()
    const now = w.find('[data-testid="as-of-now"]')
    expect(now.exists()).toBe(true)
    expect(now.text()).toBe('Now')
    // One tab stop: the button, not the input under it.
    const input = w.find('[data-testid="as-of-picker"]')
    expect(input.attributes('tabindex')).toBe('-1')
    // One control for assistive tech too: the hidden input is not announced.
    expect(input.attributes('aria-hidden')).toBe('true')
    w.unmount()
  })

  it('opens the date picker from Now', async () => {
    const showPicker = vi.fn()
    Object.defineProperty(HTMLInputElement.prototype, 'showPicker', {
      value: showPicker,
      configurable: true,
    })
    try {
      const w = mountPicker()
      await w.find('[data-testid="as-of-now"]').trigger('click')
      expect(showPicker).toHaveBeenCalledTimes(1)
      w.unmount()
    } finally {
      delete (HTMLInputElement.prototype as { showPicker?: unknown }).showPicker
    }
  })

  it('focuses the input where the browser has no showPicker', async () => {
    const w = mountPicker()
    await w.find('[data-testid="as-of-now"]').trigger('click')
    expect(document.activeElement).toBe(w.find('[data-testid="as-of-picker"]').element)
    w.unmount()
  })

  it('shows the picked date and no Now button', () => {
    asOf = '2026-01-31T23:59:59.999+01:00'
    const w = mountPicker()
    expect(w.find('[data-testid="as-of-now"]').exists()).toBe(false)
    const input = w.find('[data-testid="as-of-picker"]')
    expect((input.element as HTMLInputElement).value).toBe('2026-01-31')
    expect(input.attributes('tabindex')).toBeUndefined()
    expect(input.attributes('aria-hidden')).toBeUndefined()
    w.unmount()
  })
})
