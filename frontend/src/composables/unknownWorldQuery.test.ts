import { describe, expect, it } from 'vitest'
import { unknownWorldQuery } from './useWorld'

describe('unknownWorldQuery', () => {
  const worlds = new Map([['draft', {}], ['published', {}]])

  it('keeps a served world and a bare URL', () => {
    expect(unknownWorldQuery({ world: 'draft' }, worlds)).toBeNull()
    expect(unknownWorldQuery({ q: 'x' }, worlds)).toBeNull()
  })

  it('drops an unknown world and resets the page', () => {
    expect(unknownWorldQuery({ world: 'default', page: '3', q: 'x' }, worlds)).toEqual({ q: 'x' })
  })

  it('drops a repeated world', () => {
    expect(unknownWorldQuery({ world: ['draft', 'published'] }, worlds)).toEqual({})
  })

  it('waits for the schema', () => {
    expect(unknownWorldQuery({ world: 'nope' }, new Map())).toBeNull()
  })
})
