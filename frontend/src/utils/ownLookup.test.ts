import { describe, expect, it } from 'vitest'
import { ownLookup } from './ownLookup'

describe('ownLookup', () => {
  const table = { red: 'r' }

  it.each([
    ['red', 'r'],
    ['blue', undefined],
    ['constructor', undefined],
    ['toString', undefined],
    ['__proto__', undefined],
  ])('%s', (key, want) => {
    expect(ownLookup(table, key)).toBe(want)
  })
})
