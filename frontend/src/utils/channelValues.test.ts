import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import type { ChannelPropertySchema, ChannelPropertyType, MessagePropValue } from '@/types/channels'
import {
  codePoints,
  fromPropValue,
  initialValues,
  toEditProps,
  toPropValue,
  toProps,
} from './channelValues'

// Composer values convert per type without day shifts, and an empty
// optional field is absent (TKT-91BK6F R20, AC 17). The zone is fixed so the
// datetime cases, including the spring-forward gap, are deterministic.
let savedTZ: string | undefined
beforeAll(() => {
  savedTZ = process.env.TZ
  process.env.TZ = 'Europe/Amsterdam'
})
afterAll(() => {
  process.env.TZ = savedTZ
})

function prop(
  type: ChannelPropertyType,
  over: Partial<ChannelPropertySchema> = {}
): ChannelPropertySchema {
  return { name: 'p', label: 'P', type, ...over }
}

describe('toPropValue', () => {
  it.each([
    ['string', prop('string'), 'hi', 'hi'],
    ['string keeps spaces', prop('string'), ' hi ', ' hi '],
    ['markdown', prop('markdown'), '**x**', '**x**'],
    ['integer', prop('integer'), '42', 42],
    ['negative integer', prop('integer'), '-7', -7],
    ['largest safe integer', prop('integer'), '9007199254740991', 9007199254740991],
    ['boolean true', prop('boolean'), true, true],
    ['boolean false', prop('boolean'), false, false],
    ['date stays a string', prop('date'), '2026-03-29', '2026-03-29'],
    ['leap day', prop('date'), '2028-02-29', '2028-02-29'],
    ['datetime in winter', prop('datetime'), '2026-01-15T10:30', '2026-01-15T09:30:00Z'],
    ['datetime in summer', prop('datetime'), '2026-07-01T00:15', '2026-06-30T22:15:00Z'],
    ['datetime with seconds', prop('datetime'), '2026-07-01T00:15:09', '2026-06-30T22:15:09Z'],
    ['enum', prop('enum', { values: ['low', 'high'] }), 'high', 'high'],
    ['ref', prop('ref', { to: ['ticket'] }), 'TKT-1@draft', 'TKT-1@draft'],
    ['max counts code points', prop('string', { max: 2 }), '😀😀', '😀😀'],
  ])('%s', (_name, p, raw, want) => {
    expect(toPropValue(p, raw)).toEqual({ ok: true, value: want })
  })

  it.each([
    ['string', prop('string'), ''],
    ['blank string', prop('string'), '   '],
    ['integer', prop('integer'), ''],
    ['date', prop('date'), ''],
    ['datetime', prop('datetime'), ''],
    ['enum', prop('enum', { values: ['a'] }), ''],
    ['ref', prop('ref'), ''],
    ['unset boolean', prop('boolean'), undefined],
  ])('an empty optional %s is absent', (_name, p, raw) => {
    expect(toPropValue(p, raw)).toEqual({ ok: true, value: undefined })
  })

  it.each([
    ['string', prop('string', { required: true }), ''],
    ['integer', prop('integer', { required: true }), undefined],
    ['boolean', prop('boolean', { required: true }), undefined],
  ])('an empty required %s is refused', (_name, p, raw) => {
    expect(toPropValue(p, raw)).toEqual({ ok: false, error: 'is required' })
  })

  it.each([
    ['a fraction', prop('integer'), '1.5'],
    ['an exponent', prop('integer'), '1e3'],
    ['an unsafe integer', prop('integer'), '9007199254740993'],
    ['text for an integer', prop('integer'), 'NaN'],
    ['a day that does not exist', prop('date'), '2026-02-30'],
    ['a month that does not exist', prop('date'), '2026-13-01'],
    ['a date with a time', prop('date'), '2026-01-01T00:00'],
    ['a malformed datetime', prop('datetime'), '2026-01-01 10:00'],
    ['an hour that does not exist', prop('datetime'), '2026-01-01T24:00'],
    ['a time in the spring-forward gap', prop('datetime'), '2026-03-29T02:30'],
    ['an unknown enum value', prop('enum', { values: ['a'] }), 'b'],
    ['a string over max', prop('string', { max: 2 }), 'abc'],
    ['markdown over max', prop('markdown', { max: 1 }), '😀😀'],
    ['text for a boolean', prop('boolean'), 'true'],
  ])('refuses %s', (_name, p, raw) => {
    expect(toPropValue(p, raw).ok).toBe(false)
  })
})

describe('toProps', () => {
  const props = [
    prop('string', { name: 'body', required: true }),
    prop('integer', { name: 'level' }),
    prop('date', { name: 'due' }),
  ]

  it('leaves absent values out', () => {
    expect(toProps(props, { body: 'x', level: '', due: '2026-01-02' })).toEqual({
      ok: true,
      props: { body: 'x', due: '2026-01-02' },
    })
  })

  it('keys errors by property', () => {
    expect(toProps(props, { body: '', level: 'x' })).toEqual({
      ok: false,
      errors: { body: 'is required', level: 'must be a whole number' },
    })
  })
})

describe('fromPropValue', () => {
  it.each([
    ['string', prop('string'), 'hi', 'hi'],
    ['integer', prop('integer'), 3, '3'],
    ['boolean', prop('boolean'), true, true],
    ['absent boolean stays absent', prop('boolean'), undefined, undefined],
    ['date', prop('date'), '2026-03-29', '2026-03-29'],
    ['datetime in the local zone', prop('datetime'), '2026-06-30T22:15:00Z', '2026-07-01T00:15'],
    ['datetime with seconds', prop('datetime'), '2026-06-30T22:15:09Z', '2026-07-01T00:15:09'],
    ['readable ref', prop('ref'), { id: 'TKT-1', type: 'ticket' }, 'TKT-1'],
    ['restricted ref', prop('ref'), { restricted: true as const }, undefined],
    ['absent string', prop('string'), undefined, undefined],
  ])('%s', (_name, p, value, want) => {
    expect(fromPropValue(p, value)).toEqual(want)
  })

  it('round-trips a datetime', () => {
    const p = prop('datetime')
    const shown = fromPropValue(p, '2026-10-26T01:30:00Z')
    expect(toPropValue(p, shown)).toEqual({ ok: true, value: '2026-10-26T01:30:00Z' })
  })

  it('takes the first of a local time that repeats when the clocks go back', () => {
    expect(toPropValue(prop('datetime'), '2026-10-25T02:30')).toEqual({
      ok: true,
      value: '2026-10-25T00:30:00Z',
    })
  })
})

describe('datetime years', () => {
  it('keeps a year below 100 rather than reading it as 19xx', () => {
    const c = toPropValue(prop('datetime'), '0050-01-01T10:00')
    expect(c.ok).toBe(true)
    if (c.ok) expect(String(c.value).slice(0, 4)).toBe('0050')
  })
})

describe('initialValues', () => {
  it('seeds a required boolean with false and nothing else', () => {
    expect(
      initialValues([
        { name: 'ok', label: 'ok', type: 'boolean', required: true },
        { name: 'maybe', label: 'maybe', type: 'boolean' },
        { name: 'body', label: 'body', type: 'string', required: true },
      ])
    ).toEqual({ ok: false })
  })
})

describe('toEditProps', () => {
  const schema: ChannelPropertySchema[] = [
    { name: 'body', label: 'body', type: 'string', required: true },
    { name: 'note', label: 'note', type: 'string' },
    { name: 'flag', label: 'flag', type: 'boolean' },
    { name: 'about', label: 'about', type: 'ref', required: true },
    { name: 'also', label: 'also', type: 'ref' },
    { name: 'secret', label: 'secret', type: 'ref' },
  ]
  const current: Record<string, MessagePropValue> = {
    body: 'old',
    note: 'set by a trigger',
    flag: true,
    about: { restricted: true },
    also: { id: 'TKT-2', type: 'ticket' },
    secret: { restricted: true },
  }

  it('carries every prop the composer does not render through unchanged', () => {
    const rendered = new Set(['body', 'about'])
    expect(toEditProps(schema, rendered, { body: 'new' }, current)).toEqual({
      ok: true,
      props: { body: 'new', note: 'set by a trigger', flag: true, also: 'TKT-2' },
    })
  })

  it('lets a rendered field clear an optional prop', () => {
    const rendered = new Set(['body', 'note', 'flag'])
    expect(toEditProps(schema, rendered, { body: 'new' }, current)).toEqual({
      ok: true,
      props: { body: 'new', also: 'TKT-2' },
    })
  })

  it('still requires a rendered required field that was readable', () => {
    expect(toEditProps(schema, new Set(['body']), { body: '' }, current)).toEqual({
      ok: false,
      errors: { body: 'is required' },
    })
  })
})

describe('codePoints', () => {
  it('counts code points, not UTF-16 units', () => {
    expect(codePoints('a😀é')).toBe(3)
    expect('a😀é'.length).toBe(4)
  })
})
