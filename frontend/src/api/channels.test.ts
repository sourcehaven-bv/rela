import { describe, it, expect, vi, beforeEach } from 'vitest'
import { registerEntityPlurals } from './entities'
import {
  createChannel,
  editMessage,
  getChannelHead,
  getChannelSchema,
  getMessages,
  listChannels,
  postMessage,
  releaseMessageAction,
  removeMessage,
  runLifecycle,
  runMessageAction,
} from './channels'
import { api } from './client'

const axiosPost = vi.fn()
const axiosDelete = vi.fn()
vi.mock('./client', () => ({
  api: {
    get: vi.fn().mockResolvedValue({}),
    post: vi.fn().mockResolvedValue({}),
    patch: vi.fn().mockResolvedValue({}),
    getAxios: () => ({ post: axiosPost, delete: axiosDelete }),
  },
}))

const target = { type: 'ticket', address: 'TKT-1@draft', world: 'main', channelType: 'room' }
const base = '/tickets/TKT-1@draft/_channels/room'

// The server refuses `?world=` on a write, so the world rides on reads only,
// and the face rides in the address.
describe('channel requests', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
  })

  it('reads in the page world', async () => {
    await listChannels(target, 'open')
    await getChannelHead(target)
    await getMessages({ ...target, name: 'x1' }, { afterRev: 0, limit: 50 })
    await getChannelSchema()
    expect(api.get).toHaveBeenNthCalledWith(1, '/tickets/TKT-1@draft/_channels', {
      world: 'main',
      state: 'open',
    })
    expect(api.get).toHaveBeenNthCalledWith(2, base, { world: 'main' })
    expect(api.get).toHaveBeenNthCalledWith(
      3,
      `${base}/x1/messages`,
      { world: 'main', after_rev: 0, limit: 50 },
      undefined
    )
    expect(api.get).toHaveBeenNthCalledWith(4, '/_schema/channels')
  })

  it('tells a stored post from an ephemeral one by the status', async () => {
    axiosPost.mockResolvedValueOnce({ status: 201, data: { seq: 3, rev: 3 } })
    expect(await postMessage(target, 'text', { body: 'hi' }, 2)).toEqual({
      kind: 'message',
      message: { seq: 3, rev: 3 },
    })
    expect(axiosPost).toHaveBeenLastCalledWith(`${base}/messages`, {
      type: 'text',
      props: { body: 'hi' },
      reply_to: 2,
    })

    axiosPost.mockResolvedValueOnce({ status: 202, data: { id: 'e1', expires_at: 'x' } })
    expect(await postMessage(target, 'typing', {})).toEqual({
      kind: 'ephemeral',
      accepted: { id: 'e1', expires_at: 'x' },
    })
    expect(axiosPost).toHaveBeenLastCalledWith(`${base}/messages`, { type: 'typing', props: {} })
  })

  it('sends delete and remove bodies, never the reason in the URL', async () => {
    axiosDelete.mockResolvedValue({ status: 200, data: { seq: 4 } })
    await removeMessage(target, 4)
    expect(axiosDelete).toHaveBeenLastCalledWith(`${base}/messages/4`, { data: {} })
    await removeMessage(target, 4, 'spam')
    expect(axiosDelete).toHaveBeenLastCalledWith(`${base}/messages/4`, { data: { reason: 'spam' } })
    await releaseMessageAction(target, 4)
    expect(axiosDelete).toHaveBeenLastCalledWith(`${base}/messages/4/_action`, { data: {} })
  })

  it('edits, runs actions and creates without a world', async () => {
    await editMessage(target, 4, { body: 'x' })
    expect(api.patch).toHaveBeenCalledWith(`${base}/messages/4`, { props: { body: 'x' } })
    await runMessageAction(target, 4, 'ack')
    expect(api.post).toHaveBeenLastCalledWith(`${base}/messages/4/_action/ack`, {})
    await createChannel(target, 'threads', 'Plan')
    expect(api.post).toHaveBeenLastCalledWith('/tickets/TKT-1@draft/_channels/threads', {
      title: 'Plan',
    })
  })

  it('answers null after a lifecycle delete', async () => {
    axiosPost.mockResolvedValueOnce({ status: 204, data: '' })
    expect(await runLifecycle(target, 'delete')).toBeNull()
    axiosPost.mockResolvedValueOnce({ status: 200, data: { state: 'closed' } })
    expect(await runLifecycle(target, 'close')).toEqual({ state: 'closed' })
    expect(axiosPost).toHaveBeenLastCalledWith(`${base}/_lifecycle/close`, {})
  })
})
