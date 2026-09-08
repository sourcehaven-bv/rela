import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fetchView } from './views'
import { api } from './client'

vi.mock('./client', () => ({
  api: { get: vi.fn().mockResolvedValue({ entry: { id: 'POL-1' }, sections: [] }) },
}))

// The world on the entity VIEW (TKT-F2D5U5 gap 1).
//
// `/_views/{type}/{id}` became world-capable in TKT-WRLDAPI item 4b — the
// entry resolves to that world's face and every collection entity resolves
// through the same world, per neighbour. But the SPA never passed the param,
// so the detail page rendered draft content while the world selector said
// "published". The API was correct; the page simply never asked.
//
// Ruling 10 note: the interesting assertions here are about a param being
// ABSENT (the default world must not send `?world=`), which is the class that
// passes trivially when nothing was requested at all. So every absence
// assertion also asserts the request FIRED and carries the right path.
describe('fetchView world param', () => {
  beforeEach(() => vi.clearAllMocks())

  it('sends ?world= for a non-default world', async () => {
    await fetchView('policy', 'POL-1', 'published')

    const [path, params] = vi.mocked(api.get).mock.calls[0]
    expect(path).toBe('/_views/policy/POL-1')
    expect(params).toEqual({ world: 'published' })
  })

  // The default world must send NO param, not `?world=` empty or
  // `?world=default`. An empty value is a different request than an absent
  // one, and this is the request every existing detail page makes — a
  // regression here changes behaviour for every user, not only world users.
  it('omits the param entirely for the default world', async () => {
    await fetchView('policy', 'POL-1', undefined)

    const [path, params] = vi.mocked(api.get).mock.calls[0]
    expect(path).toBe('/_views/policy/POL-1') // the request fired
    expect(params).toBeUndefined()
  })

  // `useWorld().worldParam` yields '' for the default world in some call
  // shapes; an empty string must be treated as absent rather than sent as
  // `?world=`, which the API would read as a duplicate/empty selection.
  it('treats an empty world string as absent', async () => {
    await fetchView('policy', 'POL-1', '')

    const [, params] = vi.mocked(api.get).mock.calls[0]
    expect(params).toBeUndefined()
  })
})
