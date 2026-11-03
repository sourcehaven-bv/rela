import { describe, it, expect, vi, beforeEach } from 'vitest'
import { addPileItems, createPile, getPile, listPiles, removePileItems, updatePile } from './piles'
import { api } from './client'

vi.mock('./client', () => ({
  api: {
    get: vi.fn().mockResolvedValue({}),
    post: vi.fn().mockResolvedValue({}),
    patch: vi.fn().mockResolvedValue({}),
  },
}))

// The server refuses `?world=` on a write with 422 world_read_only, so the
// world rides on reads only (BUG-5NP70I).
describe('pile requests and the world', () => {
  beforeEach(() => vi.clearAllMocks())

  it('reads in the page world', async () => {
    await listPiles('published')
    await getPile('PIL-1', 'published')
    expect(api.get).toHaveBeenNthCalledWith(1, '/_piles', { world: 'published' }, undefined)
    expect(api.get).toHaveBeenNthCalledWith(2, '/_piles/PIL-1', { world: 'published' }, undefined)
  })

  it.each([
    ['createPile', () => createPile({ name: 'x', icon: 'layers' }), () => api.post],
    ['updatePile', () => updatePile('PIL-1', { name: 'x' }), () => api.patch],
    ['addPileItems', () => addPileItems('PIL-1', ['POL-1@draft']), () => api.post],
    ['removePileItems', () => removePileItems('PIL-1', ['POL-1@draft']), () => api.post],
  ])('%s sends no world', async (_name, call, method) => {
    await call()
    const [path] = vi.mocked(method()).mock.calls[0]
    expect(path).not.toContain('world')
  })
})
