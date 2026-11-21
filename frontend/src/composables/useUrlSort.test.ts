import { describe, it, expect } from 'vitest'
import { parseSortQuery } from './useUrlSort'

describe('parseSortQuery', () => {
  it.each([
    ['', []],
    [undefined, []],
    [null, []],
    [['x'], [{ property: 'x', direction: 'asc' }]],
    ['a', [{ property: 'a', direction: 'asc' }]],
    ['-a', [{ property: 'a', direction: 'desc' }]],
    [
      'a,-b',
      [
        { property: 'a', direction: 'asc' },
        { property: 'b', direction: 'desc' },
      ],
    ],
    ['a,,-', [{ property: 'a', direction: 'asc' }]],
    ['a,-a', [{ property: 'a', direction: 'asc' }]],
  ])('reads %j', (raw, want) => {
    expect(parseSortQuery(raw)).toEqual(want)
  })
})
