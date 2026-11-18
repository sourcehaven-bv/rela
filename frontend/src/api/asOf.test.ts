import { describe, expect, it } from 'vitest'
import { carriesAsOf, parseAsOf } from './asOf'
import { endOfDay } from '@/composables/useAsOf'

describe('carriesAsOf', () => {
  it.each([
    ['/tickets', true],
    ['/tickets/TKT-1', true],
    ['/tickets/TKT-1/relations', true],
    ['/_views/ticket/TKT-1', true],
    ['/_search', true],
    ['/_dashboard/card', true],
    ['/_gantts/plan', true],
    ['/_documents/report/TKT-1', true],
    ['/_analyze', true],
    ['/_schema', false],
    ['/_config', false],
    ['/_sidebar', false],
    ['/_dashboard', false],
    ['/_me', false],
    ['/tickets/TKT-1/_presence', false],
    ['/tickets/TKT-1/_history', false],
    ['/tickets/TKT-1/_comments', false],
  ])('%s → %s', (path, want) => {
    expect(carriesAsOf(path)).toBe(want)
  })
})

describe('parseAsOf', () => {
  it('takes an instant with a zone', () => {
    expect(parseAsOf('2025-12-31T23:59:59Z')).toBe('2025-12-31T23:59:59Z')
    expect(parseAsOf('2025-12-31T23:59:59.999+01:00')).toBe('2025-12-31T23:59:59.999+01:00')
  })
  it.each([['2025-12-31'], ['2025-12-31T23:59:59'], ['yesterday'], [['a', 'b']], [null]])(
    'drops %j',
    (raw) => {
      expect(parseAsOf(raw)).toBeUndefined()
    }
  )
})

describe('endOfDay', () => {
  it('is the last millisecond of the local day, with the local offset', () => {
    const got = endOfDay('2025-12-31')!
    expect(got).toMatch(/^2025-12-31T23:59:59\.999[+-]\d{2}:\d{2}$/)
    const d = new Date(got)
    expect(d.getFullYear()).toBe(2025)
    expect(d.getMonth()).toBe(11)
    expect(d.getDate()).toBe(31)
    expect(d.getHours()).toBe(23)
  })
  it('refuses a malformed date', () => {
    expect(endOfDay('31-12-2025')).toBeUndefined()
  })
})
