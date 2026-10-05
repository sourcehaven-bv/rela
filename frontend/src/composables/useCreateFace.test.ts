import { describe, expect, it } from 'vitest'
import { pickableFaces } from './useCreateFace'

describe('pickableFaces', () => {
  it('offers every face when the server sent no per-face keys', () => {
    expect(pickableFaces(['draft', 'published'], undefined)).toEqual(['draft', 'published'])
    expect(pickableFaces(['draft', 'published'], { create: true })).toEqual(['draft', 'published'])
  })

  it('drops a face marked uncreatable', () => {
    expect(
      pickableFaces(['draft', 'published'], { 'create@draft': true, 'create@published': false }),
    ).toEqual(['draft'])
  })
})
