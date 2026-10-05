import { describe, it, expect, afterEach } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { useStickyHeight } from './useStickyHeight'

const scopes: ReturnType<typeof effectScope>[] = []
function measure(el: HTMLElement | null) {
  const scope = effectScope()
  scopes.push(scope)
  const target = ref(el)
  const height = scope.run(() => useStickyHeight(target))!
  return { target, height }
}

function bar(position: string, height: number): HTMLElement {
  const el = document.createElement('div')
  el.style.position = position
  el.getBoundingClientRect = () => new DOMRect(0, 0, 0, height)
  document.body.appendChild(el)
  return el
}

afterEach(() => {
  scopes.splice(0).forEach((s) => s.stop())
  document.body.innerHTML = ''
})

describe('useStickyHeight', () => {
  it('is the height of a sticky bar', async () => {
    const { height } = measure(bar('sticky', 48))
    await nextTick()
    expect(height.value).toBe(48)
  })

  it('is 0 for a bar in normal flow', async () => {
    const { height } = measure(bar('static', 48))
    await nextTick()
    expect(height.value).toBe(0)
  })

  it('is 0 without a bar, and follows one that appears', async () => {
    const { target, height } = measure(null)
    await nextTick()
    expect(height.value).toBe(0)

    target.value = bar('sticky', 60)
    await nextTick()
    expect(height.value).toBe(60)
  })

  it('re-measures on resize, when the bar may have switched mode', async () => {
    const el = bar('sticky', 48)
    const { height } = measure(el)
    await nextTick()
    el.style.position = 'static'
    window.dispatchEvent(new Event('resize'))
    expect(height.value).toBe(0)
  })
})
