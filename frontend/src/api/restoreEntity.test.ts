import { describe, it, expect, vi, beforeEach } from 'vitest'
import { restoreEntity, _setEntityPluralForTest } from './entities'
import { api } from './client'

vi.mock('./client', () => ({
  api: { post: vi.fn().mockResolvedValue(undefined) },
}))

describe('restoreEntity', () => {
  beforeEach(() => vi.clearAllMocks())

  it('POSTs to the entity restore endpoint under the type plural', async () => {
    _setEntityPluralForTest('policy', 'policies')
    await restoreEntity('policy', 'POL-1')
    expect(api.post).toHaveBeenCalledWith('/policies/POL-1/restore')
  })

  it('addresses a face the same way a delete does', async () => {
    _setEntityPluralForTest('policy', 'policies')
    await restoreEntity('policy', 'POL-1@published')
    expect(api.post).toHaveBeenCalledWith('/policies/POL-1@published/restore')
  })
})
