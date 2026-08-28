import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { effectScope, nextTick, ref, type MaybeRefOrGetter } from 'vue'
import { useDelayedPending, type DelayedPendingOptions } from './useDelayedPending'

// The gate is pure timer logic, so it is driven directly with fake timers
// rather than through a mounted component. `nextTick` after each advance
// because the source is watched, not polled.
async function tick(ms: number) {
  vi.advanceTimersByTime(ms)
  await nextTick()
}

// useDelayedPending registers a watcher and an onScopeDispose hook, so it
// needs an active effect scope. Calling it bare from a test leaves the
// watcher unbound (and warns), which silently breaks every timing
// assertion — so every case runs inside a scope that is stopped afterwards.
const scopes: ReturnType<typeof effectScope>[] = []
function gate(source: MaybeRefOrGetter<boolean>, options?: DelayedPendingOptions) {
  const scope = effectScope()
  scopes.push(scope)
  let visible!: ReturnType<typeof useDelayedPending>
  scope.run(() => {
    visible = useDelayedPending(source, options)
  })
  return visible
}

describe('useDelayedPending', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => {
    scopes.forEach((s) => s.stop())
    scopes.length = 0
    vi.useRealTimers()
  })

  // AC1 — the governing rule. This is the whole reason the composable
  // exists: on a fast connection the indicator must never appear at all.
  it('shows nothing when the operation resolves before the delay', async () => {
    const pending = ref(false)
    const visible = gate(pending, { delay: 500, minDuration: 400 })

    pending.value = true
    await nextTick()
    expect(visible.value).toBe(false)

    await tick(100)
    pending.value = false
    await nextTick()
    expect(visible.value).toBe(false)

    // Nothing may appear later either — the delay timer must have been
    // cancelled, not merely ignored.
    await tick(2000)
    expect(visible.value).toBe(false)
  })

  // AC2 — once shown, hold for the minimum so a just-over-threshold
  // operation doesn't produce a blink.
  it('shows after the delay and holds for the minimum duration', async () => {
    const pending = ref(false)
    const visible = gate(pending, { delay: 500, minDuration: 400 })

    pending.value = true
    await tick(499)
    expect(visible.value).toBe(false)

    await tick(1)
    expect(visible.value).toBe(true)

    // Resolves at 700ms — 200ms into the minimum.
    await tick(200)
    pending.value = false
    await nextTick()
    expect(visible.value).toBe(true)

    // Still held at 800ms.
    await tick(100)
    expect(visible.value).toBe(true)

    // Minimum elapses at 900ms (500 delay + 400 min).
    await tick(100)
    expect(visible.value).toBe(false)
  })

  it('hides on the next tick when the minimum is already satisfied', async () => {
    const pending = ref(false)
    const visible = gate(pending, { delay: 100, minDuration: 100 })

    pending.value = true
    await tick(100)
    expect(visible.value).toBe(true)

    // Well past the minimum — nothing left to hold, so the hide is
    // scheduled with 0 delay rather than fired inline. The one-tick
    // deferral is what lets an immediately-following operation adopt the
    // display instead of flickering; see the follow-up test below.
    await tick(500)
    pending.value = false
    await tick(0)
    expect(visible.value).toBe(false)
  })

  // Edge case: a flapping source must not push the indicator out forever.
  it('does not restart the delay when the source flaps within one window', async () => {
    const pending = ref(false)
    const visible = gate(pending, { delay: 500, minDuration: 0 })

    pending.value = true
    await tick(300)
    pending.value = false
    await nextTick()
    // Cancelled — back to IDLE.
    expect(visible.value).toBe(false)

    pending.value = true
    await tick(300)
    // A fresh 500ms window started at re-entry, so still hidden.
    expect(visible.value).toBe(false)
    await tick(200)
    expect(visible.value).toBe(true)
  })

  it('re-entering while visible does not extend the minimum indefinitely', async () => {
    const pending = ref(false)
    const visible = gate(pending, { delay: 0, minDuration: 400 })

    pending.value = true
    await nextTick()
    expect(visible.value).toBe(true)

    // A second operation starts while the first indicator is still up.
    await tick(200)
    pending.value = false
    await nextTick()
    pending.value = true
    await nextTick()
    expect(visible.value).toBe(true)

    // The original minimum still governs — it is not restarted, so the
    // indicator clears once the source settles and the window passes.
    pending.value = false
    await tick(200)
    expect(visible.value).toBe(false)
  })

  it('treats delay 0 as immediate', async () => {
    const pending = ref(false)
    const visible = gate(pending, { delay: 0, minDuration: 0 })

    pending.value = true
    await nextTick()
    expect(visible.value).toBe(true)

    // minDuration 0 still hides on the next tick, not inline — see above.
    pending.value = false
    await tick(0)
    expect(visible.value).toBe(false)
  })

  it('coerces nonsensical options to the defaults rather than arming a broken timer', async () => {
    const pending = ref(false)
    const visible = gate(pending, {
      delay: Number.NaN,
      minDuration: -1,
    })

    pending.value = true
    await tick(499)
    expect(visible.value).toBe(false)
    // Fell back to the 500ms default rather than firing immediately.
    await tick(1)
    expect(visible.value).toBe(true)
  })

  it('accepts a getter source', async () => {
    const pending = ref(false)
    const visible = gate(() => pending.value, { delay: 100, minDuration: 0 })

    pending.value = true
    await tick(100)
    expect(visible.value).toBe(true)
  })

  // Timers must not outlive the scope: a late fire writes to a ref nobody
  // is watching. Same leak class as RR-YWWAL in useConfirm.
  it('clears pending timers when the scope is disposed', async () => {
    const pending = ref(false)
    const scope = effectScope()
    let visible!: ReturnType<typeof useDelayedPending>
    scope.run(() => {
      visible = useDelayedPending(pending, { delay: 500, minDuration: 400 })
    })

    pending.value = true
    await tick(100)
    scope.stop()

    await tick(2000)
    expect(visible.value).toBe(false)
    // No timers left behind to fire into a dead scope.
    expect(vi.getTimerCount()).toBe(0)
  })

  it('clears the hold timer on dispose too', async () => {
    const pending = ref(false)
    const scope = effectScope()
    scope.run(() => {
      useDelayedPending(pending, { delay: 0, minDuration: 5000 })
    })

    // Shown immediately; the hold timer arms when the source SETTLES,
    // since that is the point at which a hide has to be scheduled.
    pending.value = true
    await nextTick()
    pending.value = false
    await nextTick()
    expect(vi.getTimerCount()).toBe(1)

    scope.stop()
    expect(vi.getTimerCount()).toBe(0)
  })

  // REGRESSION (code review, critical). A second operation that begins
  // after the first one's minimum has already elapsed used to inherit no
  // minimum of its own: it vanished the instant it settled, producing the
  // blink this composable exists to prevent. Worse, the intermediate
  // settle hid the indicator outright and the restart paid the full delay
  // again — a visible flicker mid-sequence.
  it('gives a follow-up operation its own minimum once the first has elapsed', async () => {
    const pending = ref(false)
    const visible = gate(pending, { delay: 500, minDuration: 400 })

    pending.value = true
    await tick(500)
    expect(visible.value).toBe(true)

    // Run well past the minimum while still pending.
    await tick(500)
    expect(visible.value).toBe(true)

    // op1 settles and op2 starts in the same breath.
    pending.value = false
    await nextTick()
    pending.value = true
    await nextTick()
    // The indicator must NOT have blinked off in between.
    expect(visible.value).toBe(true)

    // op2 settles almost immediately. It still owns the display, so it
    // must be held rather than vanishing on the spot.
    pending.value = false
    await nextTick()
    expect(visible.value).toBe(true)

    // ...and cleared once its own hold expires.
    await tick(400)
    expect(visible.value).toBe(false)
  })

  it('holds only the REMAINING minimum when an operation settles mid-window', async () => {
    const pending = ref(false)
    const visible = gate(pending, { delay: 0, minDuration: 400 })

    pending.value = true
    await nextTick()
    expect(visible.value).toBe(true)

    // Settles 100ms in — 300ms of the minimum is still owed.
    await tick(100)
    pending.value = false
    await nextTick()
    expect(visible.value).toBe(true)

    await tick(299)
    expect(visible.value).toBe(true)
    await tick(1)
    expect(visible.value).toBe(false)
  })
})
