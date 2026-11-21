import { describe, it, expect } from 'vitest'
import { entityDisplayTitle, entityDisplayTitleWithId } from './entityDisplay'

describe('entityDisplayTitle', () => {
  it('returns _title when present', () => {
    expect(entityDisplayTitle({ id: 'PRS-DO-1', _title: 'Audit log' })).toBe('Audit log')
  })

  it('falls back to id when _title is absent', () => {
    expect(entityDisplayTitle({ id: 'PRS-DO-1' })).toBe('PRS-DO-1')
  })

  it('falls back to id when _title is empty', () => {
    expect(entityDisplayTitle({ id: 'PRS-DO-1', _title: '' })).toBe('PRS-DO-1')
  })

  it('returns _title even when it equals the id', () => {
    expect(entityDisplayTitle({ id: 'X', _title: 'X' })).toBe('X')
  })
})

describe('entityDisplayTitleWithId', () => {
  it('formats "Title (ID)" when title differs from id', () => {
    expect(entityDisplayTitleWithId({ id: 'PRS-DO-1', _title: 'Audit log' })).toBe(
      'Audit log (PRS-DO-1)',
    )
  })

  it('returns just the id when title equals id', () => {
    expect(entityDisplayTitleWithId({ id: 'PRS-DO-1', _title: 'PRS-DO-1' })).toBe('PRS-DO-1')
  })

  it('returns just the id when title is absent', () => {
    expect(entityDisplayTitleWithId({ id: 'PRS-DO-1' })).toBe('PRS-DO-1')
  })
})
