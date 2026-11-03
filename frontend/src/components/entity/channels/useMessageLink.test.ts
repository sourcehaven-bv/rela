import { describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref, shallowRef, watch } from 'vue'
import { useMessageLink, type LinkTarget } from './useMessageLink'

function setup(initial: number | undefined) {
  const seq = ref<number | undefined>(initial)
  const target: LinkTarget = { jumpTo: vi.fn() }
  const view = shallowRef<LinkTarget | null>(target)
  const done = vi.fn()
  effectScope().run(() => useMessageLink(() => seq.value, view, done))
  return { seq, target, view, done }
}

describe('useMessageLink', () => {
  it('jumps once after a tick', async () => {
    const { target, done } = setup(7)
    expect(target.jumpTo).not.toHaveBeenCalled()
    await nextTick()
    await nextTick()
    expect(target.jumpTo).toHaveBeenCalledTimes(1)
    expect(target.jumpTo).toHaveBeenCalledWith(7)
    expect(done).toHaveBeenCalledTimes(1)
  })

  it('reveals a seq that changes during the tick', async () => {
    const { seq, target, done } = setup(7)
    seq.value = 9
    for (let i = 0; i < 4; i++) await nextTick()
    expect(target.jumpTo).toHaveBeenCalledTimes(1)
    expect(target.jumpTo).toHaveBeenCalledWith(9)
    expect(done).toHaveBeenCalledTimes(1)
  })

  it('reveals a seq that changes in the same flush', async () => {
    // A second watcher moves seq while the first call waits for the flush
    // to end, so the watcher runs again inside that flush.
    const { seq, target, done } = setup(undefined)
    effectScope().run(() =>
      watch(seq, (s) => {
        if (s === 7) seq.value = 9
      })
    )
    seq.value = 7
    for (let i = 0; i < 4; i++) await nextTick()
    expect(target.jumpTo).toHaveBeenCalledTimes(1)
    expect(target.jumpTo).toHaveBeenCalledWith(9)
    expect(done).toHaveBeenCalledTimes(1)
  })

  it('does nothing when seq is cleared during the tick', async () => {
    const { seq, target, done } = setup(7)
    seq.value = undefined
    for (let i = 0; i < 4; i++) await nextTick()
    expect(target.jumpTo).not.toHaveBeenCalled()
    expect(done).not.toHaveBeenCalled()
  })

  it('waits for the log to mount', async () => {
    const { target, view, done } = setup(7)
    view.value = null
    for (let i = 0; i < 4; i++) await nextTick()
    expect(target.jumpTo).not.toHaveBeenCalled()
    view.value = target
    for (let i = 0; i < 4; i++) await nextTick()
    expect(target.jumpTo).toHaveBeenCalledWith(7)
    expect(done).toHaveBeenCalledTimes(1)
  })
})
