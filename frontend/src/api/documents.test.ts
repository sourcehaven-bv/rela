import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderDocument } from './documents'
import { api } from './client'

vi.mock('./client', () => ({
  api: { get: vi.fn().mockResolvedValue({ html: '<p>ok</p>', cached: false }) },
}))

describe('renderDocument', () => {
  beforeEach(() => vi.clearAllMocks())

  it('requests the two-segment path for an entity-anchored document', async () => {
    await renderDocument('spec', 'TKT-9')
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/_documents/spec/TKT-9')
  })

  it('omits the id segment for a standalone document', async () => {
    await renderDocument('sales_review')
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/_documents/sales_review')
  })

  // An empty-string id must take the standalone path rather than producing
  // `/_documents/sales_review/`, which the server rejects as a malformed path.
  it('treats an empty entityId as absent', async () => {
    await renderDocument('sales_review', '')
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/_documents/sales_review')
  })

  it('forwards refresh and return_to for a standalone document', async () => {
    await renderDocument('sales_review', undefined, { refresh: true, returnTo: '/dashboard' })
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/_documents/sales_review')
    expect(vi.mocked(api.get).mock.calls[0][1]).toEqual({
      refresh: 'true',
      return_to: '/dashboard',
    })
  })

  it('drops an unsafe return_to (open-redirect guard)', async () => {
    await renderDocument('sales_review', undefined, { returnTo: 'https://evil.example' })
    expect(vi.mocked(api.get).mock.calls[0][1]).toBeUndefined()
  })
})
