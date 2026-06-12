import { describe, it, expect, vi, beforeEach } from 'vitest'
import { listEntities, registerEntityPlurals, _setEntityPluralForTest } from './entities'
import { api } from './client'

vi.mock('./client', () => ({
  api: { get: vi.fn().mockResolvedValue({ data: [], meta: {}, _actions: {} }) },
}))

// B1a: the API layer resolves type → URL plural from a registry the schema
// store populates, NOT by importing the store. getPlural throws on an
// unknown type instead of fabricating a wrong URL (`category` → /categorys).
describe('entity plural resolution', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses the registered plural for the URL path', async () => {
    registerEntityPlurals(new Map([['policy', 'policies']]))
    await listEntities('policy')
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/policies')
  })

  it('throws a descriptive error for an unregistered type', async () => {
    registerEntityPlurals(new Map())
    await expect(listEntities('nope')).rejects.toThrow(/unknown entity type "nope"/)
  })

  it('registerEntityPlurals replaces the prior set', async () => {
    _setEntityPluralForTest('ticket', 'tickets')
    registerEntityPlurals(new Map([['risk', 'risks']]))
    // ticket is gone after the replace.
    await expect(listEntities('ticket')).rejects.toThrow(/unknown entity type/)
    await listEntities('risk')
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/risks')
  })
})
