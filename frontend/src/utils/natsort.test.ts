import { describe, expect, it } from 'vitest'
import { naturalCompare } from './natsort'

// The cases mirror internal/natsort/natsort_test.go, so the two orders
// cannot drift apart unnoticed.
describe('naturalCompare', () => {
  it.each([
    ['', '', 0],
    ['abc', 'abc', 0],
    ['abc', 'def', -1],
    ['abc', 'ABC', 1],
    ['ABC', 'abc', -1],
    ['9', '10', -1],
    ['100', '99', 1],
    ['REQ-2', 'REQ-10', -1],
    ['COMP-1', 'REQ-1', -1],
    ['a1b2', 'a1b10', -1],
    ['v1.2.3', 'v1.10.1', -1],
    ['REQ-01', 'REQ-1', 1],
    ['REQ-001', 'REQ-01', 1],
    ['00', '0', 1],
    ['', 'a', -1],
    ['a1', 'ab', -1],
    ['abc', 'abcd', -1],
  ])('compares %j with %j as %i', (a, b, want) => {
    expect(naturalCompare(a, b)).toBe(want)
  })
})
