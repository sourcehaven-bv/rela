import { describe, it, expect, vi, afterEach } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { PHONE_MEDIA_QUERY, usePhoneScreen } from './usePhoneScreen'

function host() {
  let phone: ReturnType<typeof usePhoneScreen> | null = null
  const C = defineComponent({
    setup() {
      phone = usePhoneScreen()
      return () => h('div')
    },
  })
  const wrapper = mount(C)
  return { wrapper, phone: () => phone!.value }
}

describe('usePhoneScreen', () => {
  afterEach(() => vi.restoreAllMocks())

  it('follows the phone media query as it changes', async () => {
    let listener: ((e: MediaQueryListEvent) => void) | null = null
    const remove = vi.fn()
    const spy = vi.spyOn(window, 'matchMedia').mockImplementation(
      (query: string) =>
        ({
          matches: true,
          media: query,
          addEventListener: (_: string, fn: (e: MediaQueryListEvent) => void) => (listener = fn),
          removeEventListener: remove,
        }) as unknown as MediaQueryList
    )
    const { wrapper, phone } = host()
    expect(spy).toHaveBeenCalledWith(PHONE_MEDIA_QUERY)
    expect(phone()).toBe(true)

    listener!({ matches: false } as MediaQueryListEvent)
    await nextTick()
    expect(phone()).toBe(false)

    wrapper.unmount()
    expect(remove).toHaveBeenCalled()
  })

  it('is false without matchMedia', () => {
    const original = window.matchMedia
    // @ts-expect-error: simulate a browser without matchMedia
    delete window.matchMedia
    try {
      const { phone } = host()
      expect(phone()).toBe(false)
    } finally {
      window.matchMedia = original
    }
  })
})
