// An EventSource that receives the FakeServer's events.
//
// Install it with `vi.stubGlobal('EventSource', fakeEventSource(server))`. With
// a scheduler, each event is delivered when the scheduler picks it, so a test
// can let an `entity:changed` arrive before, between or after the answers to
// requests in flight. That is the order a real stream gives no guarantee
// about.

import type { Scheduler } from 'fast-check'

import type { FakeServer } from './server'

export function fakeEventSource(server: FakeServer, scheduler?: Scheduler): typeof EventSource {
  let seq = 0

  class FakeEventSource {
    static readonly CONNECTING = 0
    static readonly OPEN = 1
    static readonly CLOSED = 2
    readonly CONNECTING = 0
    readonly OPEN = 1
    readonly CLOSED = 2

    readonly url: string
    readonly withCredentials = false
    readyState = FakeEventSource.CONNECTING
    onopen: ((event: Event) => void) | null = null
    onerror: ((event: Event) => void) | null = null
    onmessage: ((event: MessageEvent) => void) | null = null
    private readonly listeners = new Map<string, Set<(event: MessageEvent) => void>>()
    private readonly unsubscribe: () => void

    constructor(url: string | URL) {
      this.url = String(url)
      const stream = server.connect((e) => {
        // A write's live round runs when its event is delivered, so writes
        // made before a delivery share one round, as writes inside one
        // window do on the server.
        const deliver = () => {
          for (const f of stream.liveRound()) this.dispatch(f.event, JSON.stringify(f.data))
          this.dispatch(e.event, JSON.stringify(e.data))
        }
        if (scheduler) void scheduler.schedule(Promise.resolve(), `sse ${e.event}`).then(deliver)
        else queueMicrotask(deliver)
      })
      this.unsubscribe = stream.close
      queueMicrotask(() => {
        if (this.readyState === FakeEventSource.CLOSED) return
        this.readyState = FakeEventSource.OPEN
        this.onopen?.(new Event('open'))
        this.dispatch('ready', JSON.stringify({ conn: stream.conn }))
      })
    }

    addEventListener(type: string, listener: (event: MessageEvent) => void) {
      let set = this.listeners.get(type)
      if (!set) this.listeners.set(type, (set = new Set()))
      set.add(listener)
    }

    removeEventListener(type: string, listener: (event: MessageEvent) => void) {
      this.listeners.get(type)?.delete(listener)
    }

    dispatchEvent(): boolean {
      return true
    }

    close() {
      this.readyState = FakeEventSource.CLOSED
      this.unsubscribe()
    }

    private dispatch(type: string, data: string) {
      if (this.readyState === FakeEventSource.CLOSED) return
      const event = new MessageEvent(type, { data, lastEventId: String(++seq) })
      for (const l of this.listeners.get(type) ?? []) l(event)
      if (type === 'message') this.onmessage?.(event)
    }
  }

  return FakeEventSource as unknown as typeof EventSource
}
