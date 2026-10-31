import { describe, it, expect, vi, afterEach } from 'vitest'
import { nextTick, ref } from 'vue'
import { useBoardFlip } from './useBoardFlip'

// Cards slide to where the data put them (TKT-0VBQW0H). Nothing plays under
// reduced motion, and a group only opens up when it is new, not on load.

function place(node: HTMLElement, x: number, y: number) {
  node.getBoundingClientRect = () => ({ left: x, top: y, width: 100, height: 50 }) as DOMRect
}

// The observer the board watches its width with; `resize` reports a width.
let resize: (width: number) => void = () => {}

function setup(reduce: boolean) {
  vi.stubGlobal('ResizeObserver', class {
    constructor(cb: (entries: { contentRect: { width: number } }[]) => void) {
      resize = (width) => cb([{ contentRect: { width } }])
    }
    observe() {}
    unobserve() {}
    disconnect() {}
  })
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: reduce && q.includes('reduce') }))
  const root = document.createElement('div')
  place(root, 0, 0)
  document.body.appendChild(root)
  const finish = vi.fn()
  const animate = vi.fn((_frames: Keyframe[], _options?: KeyframeAnimationOptions) => ({ finish, addEventListener: vi.fn() }))
  // jsdom has no Web Animations API.
  HTMLElement.prototype.animate = animate as never
  const add = (attr: 'item' | 'section', id: string, x: number, y: number) => {
    const node = document.createElement('div')
    node.dataset[attr === 'item' ? 'itemId' : 'sectionId'] = id
    place(node, x, y)
    root.appendChild(node)
    return node
  }
  const tick = ref(0)
  useBoardFlip(ref(root), () => tick.value, () => true)
  const render = async () => {
    tick.value++
    await nextTick()
    await nextTick()
  }
  return { root, animate, finish, add, render }
}

afterEach(() => {
  vi.unstubAllGlobals()
  delete (HTMLElement.prototype as { animate?: unknown }).animate
  document.body.replaceChildren()
})

describe('useBoardFlip', () => {
  it('slides a card that moved and opens only a new section', async () => {
    const { animate, add, render } = setup(false)
    const card = add('item', 'N-1', 0, 0)
    add('section', 'CL-1', 0, 0)
    await render()
    animate.mockClear()

    place(card, 200, 0)
    add('section', 'CL-2', 300, 0)
    await render()

    const slides = animate.mock.calls.filter(([frames]) => String(frames[0].transform).startsWith('translate'))
    expect(slides).toHaveLength(1)
    expect(slides[0][0][0].transform).toBe('translate(-200px, 0px)')
    const opens = animate.mock.calls.filter(([frames]) => frames[0].transform === 'scale(0.85)')
    expect(opens).toHaveLength(1)
  })

  it('finishes only its own animations before it measures', async () => {
    const { finish, add, render } = setup(false)
    const card = add('item', 'N-1', 0, 0)
    // A card's own CSS animation, such as the infinite wobble, cannot be
    // finished; it must be left alone.
    const wobble = { finish: vi.fn(() => { throw new Error('InvalidStateError') }) }
    card.getAnimations = () => [wobble] as never
    await render()
    place(card, 200, 0)
    await render()
    place(card, 0, 0)
    await render()
    expect(wobble.finish).not.toHaveBeenCalled()
    expect(finish).toHaveBeenCalled()
  })

  it('slides nothing on the first render after the board width changed', async () => {
    const { animate, add, render } = setup(false)
    const card = add('item', 'N-1', 0, 0)
    resize(800)
    await render()
    animate.mockClear()

    resize(600)
    place(card, 200, 0)
    await render()
    expect(animate).not.toHaveBeenCalled()

    place(card, 0, 0)
    await render()
    expect(animate).toHaveBeenCalled()
  })

  it('plays nothing under reduced motion', async () => {
    const { animate, add, render } = setup(true)
    const card = add('item', 'N-1', 0, 0)
    await render()
    place(card, 200, 0)
    add('section', 'CL-2', 300, 0)
    await render()
    expect(animate).not.toHaveBeenCalled()
  })
})
