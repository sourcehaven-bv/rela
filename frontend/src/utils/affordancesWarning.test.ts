import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  _resetAffordancesWarningForTests,
  warnIfMissingActions,
} from './affordancesWarning'

describe('warnIfMissingActions', () => {
  beforeEach(() => {
    _resetAffordancesWarningForTests()
    vi.restoreAllMocks()
  })

  it('warns once when _actions is missing from an entity response', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    warnIfMissingActions({ id: 'TKT-1', type: 'ticket' } as never, '/api/v1/tickets/TKT-1')

    expect(spy).toHaveBeenCalledTimes(1)
    expect(spy.mock.calls[0][0]).toMatch(/missing the _actions field/)
  })

  it('does not warn when _actions is present (even empty)', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    warnIfMissingActions({ _actions: {} }, '/api/v1/tickets/TKT-1')
    warnIfMissingActions({ _actions: { delete: true } }, '/api/v1/tickets/TKT-2')

    expect(spy).not.toHaveBeenCalled()
  })

  it('deduplicates on repeat with the same requestPath', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    warnIfMissingActions({ id: 'X' } as never, '/api/v1/tickets/TKT-1')
    warnIfMissingActions({ id: 'X' } as never, '/api/v1/tickets/TKT-1')
    warnIfMissingActions({ id: 'X' } as never, '/api/v1/tickets/TKT-1')

    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('warns separately for different requestPaths', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    warnIfMissingActions({ id: 'X' } as never, '/api/v1/tickets/TKT-1')
    warnIfMissingActions({ id: 'X' } as never, '/api/v1/tickets/TKT-2')

    expect(spy).toHaveBeenCalledTimes(2)
  })

  it('treats a list response with top-level _actions as present', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    // List shape: { data: [...], meta: {...}, _actions: {create: true} }
    warnIfMissingActions(
      { data: [], _actions: { create: false } } as never,
      '/api/v1/tickets',
    )

    expect(spy).not.toHaveBeenCalled()
  })

  it('warns on list responses missing _actions', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    warnIfMissingActions({ data: [], meta: {} } as never, '/api/v1/tickets')

    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('does nothing for undefined response (no crash, no warn)', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    warnIfMissingActions(undefined, '/api/v1/tickets/TKT-1')

    expect(spy).not.toHaveBeenCalled()
  })

  // AC11 (additive vocabulary): the SPA tolerates unknown verbs in
  // the _actions map. When a future ticket adds a new verb, fixtures
  // not referencing it should pass unchanged. We assert here that
  // the warning helper specifically doesn't trigger on extra keys.
  it('does not warn when _actions has unknown verbs', () => {
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    warnIfMissingActions(
      { _actions: { noop: true, delete: false, update: true } },
      '/api/v1/tickets/TKT-1',
    )

    expect(spy).not.toHaveBeenCalled()
  })
})
