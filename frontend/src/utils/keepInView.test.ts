import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { keepInView } from './keepInView'

// BUG-UZGMHIK: content that renders above the target after the first scroll
// pushed the step form to the bottom edge of the pane. keepInView scrolls
// again whenever the target moves, until the page settles or the user scrolls.

let frames: FrameRequestCallback[] = []
function runFrame() {
  const due = frames
  frames = []
  for (const f of due) f(0)
}

/** An element whose position the test sets, and whose scroll puts it at 0. */
function element() {
  const el = document.createElement('div')
  let top = 500
  el.getBoundingClientRect = () => new DOMRect(0, top, 0, 0)
  el.scrollIntoView = vi.fn(() => {
    top = 0
  })
  document.body.appendChild(el)
  return {
    el,
    shift(by: number) {
      top += by
    },
  }
}

describe('keepInView', () => {
  beforeEach(() => {
    frames = []
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((f) => {
      frames.push(f)
      return frames.length
    })
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {
      frames = []
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('scrolls at once and again each time content above pushes the target down', () => {
    const t = element()
    const stop = keepInView(() => t.el)
    expect(t.el.scrollIntoView).toHaveBeenCalledTimes(1)
    expect(t.el.scrollIntoView).toHaveBeenCalledWith({ block: 'start' })

    runFrame()
    expect(t.el.scrollIntoView).toHaveBeenCalledTimes(1)

    t.shift(400)
    runFrame()
    expect(t.el.scrollIntoView).toHaveBeenCalledTimes(2)
    stop()
  })

  it('takes over a target that renders later', () => {
    const card = element()
    let form: HTMLElement | null = null
    const stop = keepInView(() => form ?? card.el)
    form = element().el
    runFrame()
    expect(form.scrollIntoView).toHaveBeenCalledTimes(1)
    stop()
  })

  it('stops when the user scrolls', () => {
    const t = element()
    keepInView(() => t.el)
    window.dispatchEvent(new Event('wheel'))
    t.shift(400)
    runFrame()
    expect(t.el.scrollIntoView).toHaveBeenCalledTimes(1)
  })

  it('stops after the timeout', () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    const t = element()
    keepInView(() => t.el, { timeoutMs: 1000, idleMs: 500 })
    vi.setSystemTime(Date.now() + 1001)
    t.shift(400)
    runFrame()
    expect(t.el.scrollIntoView).toHaveBeenCalledTimes(1)
    expect(frames).toHaveLength(0)
  })

  it('keeps going past the timeout while the target still moves, up to the cap', () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    const t = element()
    keepInView(() => t.el, { timeoutMs: 1000, idleMs: 500, maxMs: 3000 })
    vi.setSystemTime(Date.now() + 900)
    t.shift(400)
    runFrame()
    vi.setSystemTime(Date.now() + 300)
    t.shift(400)
    runFrame()
    expect(t.el.scrollIntoView).toHaveBeenCalledTimes(3)
    vi.setSystemTime(Date.now() + 2000)
    t.shift(400)
    runFrame()
    expect(t.el.scrollIntoView).toHaveBeenCalledTimes(3)
    expect(frames).toHaveLength(0)
  })

  it('removes its listeners when stopped', () => {
    const removed = vi.spyOn(window, 'removeEventListener')
    const t = element()
    keepInView(() => t.el)()
    const events = removed.mock.calls.map(([ev]) => ev)
    expect(events).toEqual(expect.arrayContaining(['wheel', 'touchmove', 'keydown', 'pointerdown']))
  })

  it('stops when abort says so', () => {
    const t = element()
    let gone = false
    keepInView(() => t.el, { abort: () => gone })
    gone = true
    t.shift(400)
    runFrame()
    expect(t.el.scrollIntoView).toHaveBeenCalledTimes(1)
    expect(frames).toHaveLength(0)
  })
})
