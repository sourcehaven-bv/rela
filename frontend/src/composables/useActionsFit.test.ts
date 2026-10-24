import { describe, it, expect, beforeEach, vi } from 'vitest'
import { defineComponent, h, nextTick, ref, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import { useActionsFit } from './useActionsFit'

let callbacks: Array<() => void> = []
class FakeResizeObserver {
  constructor(cb: () => void) {
    callbacks.push(cb)
  }
  observe() {}
  disconnect() {}
}

function sized(el: HTMLElement, prop: 'clientWidth' | 'scrollWidth', value: () => number) {
  Object.defineProperty(el, prop, { configurable: true, get: value })
}

describe('useActionsFit', () => {
  beforeEach(() => {
    callbacks = []
    vi.stubGlobal('ResizeObserver', FakeResizeObserver)
  })

  function setup() {
    const width = ref(1000)
    const key = ref(0)
    let api!: ReturnType<typeof useActionsFit>
    let els!: { container: Ref<HTMLElement | null>; row: Ref<HTMLElement | null> }
    const Host = defineComponent({
      setup() {
        const container = ref<HTMLElement | null>(null)
        const info = ref<HTMLElement | null>(null)
        const row = ref<HTMLElement | null>(null)
        api = useActionsFit(container, info, row, key)
        els = { container, row }
        return () =>
          h('header', { ref: container }, [
            h('div', { ref: info, style: 'min-width: 200px' }),
            h('div', { ref: row }),
          ])
      },
    })
    const wrapper = mount(Host, { attachTo: document.body })
    const header = els.container.value!
    const row = els.row.value!
    sized(header, 'clientWidth', () => width.value)
    sized(row, 'scrollWidth', () => 600)
    const resize = (w: number) => {
      width.value = w
      callbacks.forEach((cb) => cb())
    }
    return { api, resize, key, wrapper }
  }

  it('stays full while the row fits beside the title', () => {
    const { api, resize } = setup()
    resize(800) // 600 + 200
    expect(api.compact.value).toBe(false)
  })

  it('turns compact when the row does not fit and back when it does', () => {
    const { api, resize } = setup()
    resize(799)
    expect(api.compact.value).toBe(true)
    resize(700)
    expect(api.compact.value).toBe(true)
    resize(900)
    expect(api.compact.value).toBe(false)
  })

  it('remeasures the full row when the actions change', async () => {
    const { api, resize, key } = setup()
    resize(700)
    expect(api.compact.value).toBe(true)
    key.value++
    await nextTick()
    await nextTick()
    expect(api.compact.value).toBe(true)
  })
})
