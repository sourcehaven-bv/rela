// An exact-order stand-in for fc.scheduler(), for the orders random runs
// rarely reach. fakeAdapter and fakeEventSource take it as their scheduler.

import { flushPromises } from '@vue/test-utils'
import { expect, vi } from 'vitest'

/**
 * A scheduler that runs a held step only when the test names it. It stands
 * in for fc.scheduler() where a test needs one exact order.
 */
export class Gate {
  private held: { label: string; release: () => void }[] = []

  schedule<T>(p: Promise<T>, label: string): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      this.held.push({ label, release: () => p.then(resolve, reject) })
    })
  }

  count() {
    return this.held.length
  }

  /** Runs the first (or last) held step whose label starts with prefix. */
  async release(prefix: string, which: 'first' | 'last' = 'first') {
    const match = (h: { label: string }) => h.label.startsWith(prefix)
    const all = this.held.flatMap((h, at) => (match(h) ? [at] : []))
    const i = (which === 'first' ? all[0] : all[all.length - 1]) ?? -1
    expect(
      i,
      `a held step "${prefix}"; held: ${this.held.map((h) => h.label).join(', ')}`
    ).toBeGreaterThanOrEqual(0)
    this.held.splice(i, 1)[0]!.release()
    await flushPromises()
  }

  /** Runs held steps, and moves the clock, until nothing is left. */
  async drain() {
    for (let round = 0; round < 50; round++) {
      vi.advanceTimersByTime(2000)
      await flushPromises()
      if (this.held.length === 0) return
      while (this.held.length) {
        this.held.shift()!.release()
        await flushPromises()
      }
    }
    throw new Error('Gate.drain: work kept arriving')
  }
}
