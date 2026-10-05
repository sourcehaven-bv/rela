import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  acceptComment,
  addComment,
  checkAnchorable,
  deleteComment,
  listComments,
  updateComment,
} from './comments'
import { api } from './client'

vi.mock('./client', () => ({
  api: {
    get: vi.fn().mockResolvedValue({ comments: [] }),
    post: vi.fn().mockResolvedValue({}),
    patch: vi.fn().mockResolvedValue(undefined),
    delete: vi.fn().mockResolvedValue(undefined),
  },
}))

// A comment thread is stored per face, and the server refuses to choose a
// face for a bare-id write on a faced type. Each call must therefore carry
// the face the caller names, encoded into the one path segment.
describe('comment requests address a face', () => {
  beforeEach(() => vi.clearAllMocks())

  const base = '/_comments/policy/POL-1%40draft'

  it.each([
    ['listComments', () => listComments('policy', 'POL-1@draft'), () => api.get, base],
    [
      'addComment',
      () =>
        addComment('policy', 'POL-1@draft', {
          anchor: { kind: 'property', ref: 'title' },
          body: 'x',
        }),
      () => api.post,
      base,
    ],
    [
      'updateComment',
      () => updateComment('policy', 'POL-1@draft', 'c1', { resolved: true }),
      () => api.patch,
      `${base}/c1`,
    ],
    [
      'deleteComment',
      () => deleteComment('policy', 'POL-1@draft', 'c1'),
      () => api.delete,
      `${base}/c1`,
    ],
    [
      'checkAnchorable',
      () => checkAnchorable('policy', 'POL-1@draft', 'q', '', ''),
      () => api.post,
      `${base}/resolve`,
    ],
    [
      'acceptComment',
      () => acceptComment('policy', 'POL-1@draft', 'c1'),
      () => api.post,
      `${base}/c1/accept`,
    ],
  ])('%s sends the face in the path', async (_name, call, method, want) => {
    await call()
    expect(vi.mocked(method()).mock.calls[0][0]).toBe(want)
  })

  it('keeps a bare id bare for a faceless type', async () => {
    await listComments('ticket', 'TKT-1')
    expect(vi.mocked(api.get).mock.calls[0][0]).toBe('/_comments/ticket/TKT-1')
  })
})
