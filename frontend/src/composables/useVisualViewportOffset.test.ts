import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { useVisualViewportOffset } from './useVisualViewportOffset'

interface FakeVisualViewport {
  offsetTop: number
  addEventListener: ReturnType<typeof vi.fn>
  removeEventListener: ReturnType<typeof vi.fn>
}

function makeFakeVV(offsetTop = 0): FakeVisualViewport {
  return {
    offsetTop,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }
}

const Host = defineComponent({
  setup() {
    useVisualViewportOffset()
    return () => h('div')
  },
})

describe('useVisualViewportOffset', () => {
  let originalVV: VisualViewport | null

  beforeEach(() => {
    originalVV = window.visualViewport
    document.documentElement.style.removeProperty('--vv-offset-top')
  })

  afterEach(() => {
    Object.defineProperty(window, 'visualViewport', {
      configurable: true,
      value: originalVV,
    })
    document.documentElement.style.removeProperty('--vv-offset-top')
  })

  it('sets --vv-offset-top from visualViewport.offsetTop on mount', () => {
    const vv = makeFakeVV(42)
    Object.defineProperty(window, 'visualViewport', { configurable: true, value: vv })

    mount(Host)

    expect(document.documentElement.style.getPropertyValue('--vv-offset-top')).toBe('42px')
  })

  it('registers resize and scroll listeners', () => {
    const vv = makeFakeVV(0)
    Object.defineProperty(window, 'visualViewport', { configurable: true, value: vv })

    mount(Host)

    const events = vv.addEventListener.mock.calls.map((c) => c[0])
    expect(events).toContain('resize')
    expect(events).toContain('scroll')
  })

  it('removes listeners on unmount', async () => {
    const vv = makeFakeVV(0)
    Object.defineProperty(window, 'visualViewport', { configurable: true, value: vv })

    const wrapper = mount(Host)
    wrapper.unmount()
    await nextTick()

    expect(vv.removeEventListener).toHaveBeenCalledWith('resize', expect.any(Function))
    expect(vv.removeEventListener).toHaveBeenCalledWith('scroll', expect.any(Function))
  })

  it('no-ops when window.visualViewport is unavailable', () => {
    Object.defineProperty(window, 'visualViewport', { configurable: true, value: null })

    expect(() => mount(Host)).not.toThrow()
    expect(document.documentElement.style.getPropertyValue('--vv-offset-top')).toBe('')
  })
})
