import { beforeEach, describe, expect, it, vi } from 'vitest'

const get = vi.fn()
const post = vi.fn()
vi.mock('./client', () => ({ api: { get: (u: string) => get(u), post: (u: string) => post(u) } }))

import { getMe } from './me'

// A member with no person yet is provisioned when the app loads, so the
// people list shows them before their first change (BUG-WS7QF2K).
describe('getMe', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('returns the GET answer when the principal is not provisionable', async () => {
    const me = { user: 'PERS-1', person: { type: 'person', id: 'PERS-1', title: 'A' } }
    get.mockResolvedValue(me)
    expect(await getMe()).toBe(me)
    expect(post).not.toHaveBeenCalled()
  })

  it('posts to provision a provisionable principal and returns its answer', async () => {
    const provisioned = { user: 'PERS-2', person: { type: 'person', id: 'PERS-2', title: 'B' } }
    get.mockResolvedValue({ user: 'usr_2', provisionable: true })
    post.mockResolvedValue(provisioned)
    expect(await getMe()).toBe(provisioned)
    expect(post).toHaveBeenCalledWith('/_me')
  })

  it('keeps the GET answer when the POST is refused', async () => {
    const me = { user: 'usr_3', provisionable: true }
    get.mockResolvedValue(me)
    post.mockRejectedValue(new Error('read-only'))
    expect(await getMe()).toBe(me)
  })
})
