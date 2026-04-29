import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ScriptError } from '../types/scriptError'

import { useScriptErrorStore } from './scriptError'

function makeError(overrides: Partial<ScriptError> = {}): ScriptError {
  return {
    error: 'script_error',
    correlation_id: 'abc123',
    script: { surface: 'action', path: 'actions/x.lua' },
    lua: { message: 'boom', line: 1 },
    ...overrides,
  }
}

describe('scriptError store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts empty', () => {
    const store = useScriptErrorStore()
    expect(store.current).toBeNull()
  })

  it('show sets the current error', () => {
    const store = useScriptErrorStore()
    const err = makeError()
    store.show(err)
    expect(store.current).toEqual(err)
  })

  it('latest replaces previous (single-modal model)', () => {
    const store = useScriptErrorStore()
    const first = makeError({ correlation_id: 'first' })
    const second = makeError({ correlation_id: 'second' })
    store.show(first)
    store.show(second)
    expect(store.current?.correlation_id).toBe('second')
  })

  it('dismiss clears and restores focus to the trigger', () => {
    const store = useScriptErrorStore()
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    const focusSpy = vi.spyOn(trigger, 'focus')

    store.show(makeError(), trigger)
    store.dismiss()

    expect(store.current).toBeNull()
    expect(focusSpy).toHaveBeenCalledTimes(1)
    document.body.removeChild(trigger)
  })

  it('dismiss is a no-op when nothing is showing', () => {
    const store = useScriptErrorStore()
    expect(() => store.dismiss()).not.toThrow()
    expect(store.current).toBeNull()
  })

  it('dismiss skips focus restore when the trigger has been detached', () => {
    // List actions can detach the trigger before dismiss runs (the
    // optimistic row removal in useListActions unmounts the action
    // header alongside the focused row). The store must not call
    // .focus() on a detached node — that silently lands focus on body.
    const store = useScriptErrorStore()
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    const focusSpy = vi.spyOn(trigger, 'focus')

    store.show(makeError(), trigger)
    document.body.removeChild(trigger)
    store.dismiss()

    expect(store.current).toBeNull()
    expect(focusSpy).not.toHaveBeenCalled()
  })
})
