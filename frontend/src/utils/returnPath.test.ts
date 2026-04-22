import { describe, it, expect } from 'vitest'
import { isSafeReturnPath, readReturnTo } from './returnPath'

describe('isSafeReturnPath', () => {
  it.each([
    ['simple path', '/entity/x/Y', '/entity/x/Y'],
    ['with query', '/list/all?status=open', '/list/all?status=open'],
    ['with fragment', '/doc/x#section', '/doc/x#section'],
    ['path + query + fragment', '/form/x?y=1#sec', '/form/x?y=1#sec'],
    ['just slash', '/', '/'],
  ])('accepts %s', (_name, input, expected) => {
    expect(isSafeReturnPath(input)).toBe(expected)
  })

  it.each([
    ['protocol-relative', '//evil.com/pwn'],
    ['backslash literal', '/\\evil.com'],
    ['percent-encoded backslash (upper)', '/%5Cevil.com'],
    ['percent-encoded backslash (lower)', '/%5cevil.com'],
    ['percent-encoded slash (upper)', '/%2Fevil.com'],
    ['percent-encoded slash (lower)', '/%2fevil.com'],
    ['http scheme', 'http://evil.com'],
    ['https scheme', 'https://evil.com'],
    ['mailto', 'mailto:evil@evil.com'],
    ['javascript scheme', 'javascript:alert(1)'],
    ['data scheme', 'data:text/html,<x>'],
    ['no leading slash', 'evil.com'],
    ['empty', ''],
    ['null', null],
    ['undefined', undefined],
    ['array (vue-router duplicate keys)', ['/a', '/b']],
    ['number', 42],
  ])('rejects %s', (_name, input) => {
    expect(isSafeReturnPath(input)).toBe('')
  })
})

describe('readReturnTo', () => {
  it('returns the normalised path when the query has a single valid return_to', () => {
    expect(readReturnTo({ return_to: '/entity/x/Y' })).toBe('/entity/x/Y')
  })

  it('returns null when return_to is absent', () => {
    expect(readReturnTo({})).toBe(null)
    expect(readReturnTo({ other: '/foo' })).toBe(null)
  })

  it('returns null when return_to is an array (vue-router duplicate key)', () => {
    expect(readReturnTo({ return_to: ['/a', '/b'] })).toBe(null)
  })

  it('returns null when return_to is null or undefined', () => {
    expect(readReturnTo({ return_to: null })).toBe(null)
    expect(readReturnTo({ return_to: undefined })).toBe(null)
  })

  it('returns null on open-redirect payloads', () => {
    expect(readReturnTo({ return_to: '//evil.com' })).toBe(null)
    expect(readReturnTo({ return_to: 'http://evil.com' })).toBe(null)
    expect(readReturnTo({ return_to: '/\\evil.com' })).toBe(null)
  })

  it('returns null when the value does not start with /', () => {
    expect(readReturnTo({ return_to: 'entity/x/Y' })).toBe(null)
  })
})
