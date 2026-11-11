import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { HEARTBEAT_MS, STALE_MS, startTabFocus } from './tabFocus'

// A BroadcastChannel that delivers to the other instances of the same name.
class FakeChannel {
  static all: FakeChannel[] = []
  onmessage: ((e: MessageEvent) => void) | null = null
  constructor(public name: string) {
    FakeChannel.all.push(this)
  }
  postMessage(data: unknown) {
    for (const c of FakeChannel.all) {
      if (c !== this && c.name === this.name) c.onmessage?.({ data } as MessageEvent)
    }
  }
  close() {
    FakeChannel.all = FakeChannel.all.filter((c) => c !== this)
  }
}

let focused = false

beforeEach(() => {
  vi.useFakeTimers()
  FakeChannel.all = []
  vi.stubGlobal('BroadcastChannel', FakeChannel)
  vi.spyOn(document, 'hasFocus').mockImplementation(() => focused)
  focused = false
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('tabFocus', () => {
  it('sees its own focus', () => {
    const f = startTabFocus('c')
    expect(f.anyFocused()).toBe(false)
    focused = true
    expect(f.anyFocused()).toBe(true)
    f.stop()
  })

  it('sees another focused tab, until it blurs', () => {
    const holder = startTabFocus('c')
    const other = startTabFocus('c')
    // The other tab gains focus; in jsdom both share hasFocus, so post by hand.
    ;(FakeChannel.all[1] as FakeChannel).postMessage({ id: 'x', focused: true })
    expect(holder.anyFocused()).toBe(true)
    ;(FakeChannel.all[1] as FakeChannel).postMessage({ id: 'x', focused: false })
    expect(holder.anyFocused()).toBe(false)
    holder.stop()
    other.stop()
  })

  it('forgets a focus report that is not repeated', () => {
    const holder = startTabFocus('c')
    const peer = new FakeChannel('c')
    peer.postMessage({ id: 'x', focused: true })
    expect(holder.anyFocused()).toBe(true)
    vi.advanceTimersByTime(STALE_MS)
    expect(holder.anyFocused()).toBe(false)
    holder.stop()
  })

  it('ignores other channels and malformed messages', () => {
    const holder = startTabFocus('c')
    new FakeChannel('other').postMessage({ id: 'x', focused: true })
    new FakeChannel('c').postMessage({ id: 1, focused: 'yes' })
    expect(holder.anyFocused()).toBe(false)
    holder.stop()
  })

  it('repeats its focus while focused', () => {
    const peer = new FakeChannel('c')
    const got: unknown[] = []
    peer.onmessage = (e) => got.push(e.data)
    const f = startTabFocus('c')
    got.length = 0
    focused = true
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    vi.advanceTimersByTime(HEARTBEAT_MS)
    expect(got).toEqual([expect.objectContaining({ focused: true })])
    f.stop()
    expect(got[got.length - 1]).toEqual(expect.objectContaining({ focused: false }))
  })
  it('says nothing after pagehide, and speaks again when restored', () => {
    const f = startTabFocus('c')
    const peer = new FakeChannel('c')
    const got: Array<{ focused: boolean }> = []
    peer.onmessage = (e) => got.push(e.data as { focused: boolean })
    focused = true
    window.dispatchEvent(new Event('pagehide'))
    document.dispatchEvent(new Event('visibilitychange'))
    window.dispatchEvent(new Event('focus'))
    vi.advanceTimersByTime(HEARTBEAT_MS)
    expect(got).toEqual([expect.objectContaining({ focused: false })])

    const restored = new Event('pageshow') as PageTransitionEvent
    Object.defineProperty(restored, 'persisted', { value: true })
    window.dispatchEvent(restored)
    expect(got[got.length - 1]).toEqual(expect.objectContaining({ focused: true }))
    f.stop()
  })
  it('answers a starting tab at once, and asks when it starts itself', () => {
    focused = true
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    const newcomer = new FakeChannel('c')
    const got: Array<{ id: string; focused: boolean; ask?: boolean }> = []
    newcomer.onmessage = (e) => got.push(e.data)
    const f = startTabFocus('c')
    expect(got).toEqual([expect.objectContaining({ focused: true, ask: true })])

    got.length = 0
    newcomer.postMessage({ id: 'n', focused: false, ask: true })
    expect(got).toEqual([expect.objectContaining({ focused: true })])
    expect(got[0]!.ask).toBeUndefined()
    f.stop()
  })
})
