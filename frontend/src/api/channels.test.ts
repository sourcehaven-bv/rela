import { describe, it, expect, vi, beforeEach } from 'vitest'
import { registerEntityPlurals } from './entities'
import {
  createChannel,
  editMessage,
  fetchMessageFile,
  getChannelHead,
  getChannelSchema,
  getInbox,
  getInboxSummary,
  getMessages,
  listChannels,
  markChannelRead,
  postMessage,
  releaseMessageAction,
  removeMessage,
  runLifecycle,
  runMessageAction,
} from './channels'
import { api } from './client'
import { ApiError } from './errors'

const axiosPost = vi.fn()
const axiosDelete = vi.fn()
const axiosGet = vi.fn()
vi.mock('./client', () => ({
  api: {
    get: vi.fn().mockResolvedValue({}),
    post: vi.fn().mockResolvedValue({}),
    patch: vi.fn().mockResolvedValue({}),
    put: vi.fn().mockResolvedValue({}),
    getAxios: () => ({ post: axiosPost, delete: axiosDelete, get: axiosGet }),
  },
}))

const target = { type: 'ticket', address: 'TKT-1@draft', channelType: 'room' }
const base = '/tickets/TKT-1@draft/_channels/room'

// The face rides in the address. No request carries `?world=`: the server
// refuses a non-default world on every channel route.
describe('channel paths', () => {
  it('refuses an address that would leave the entity path', async () => {
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
    await expect(getChannelHead({ ...target, address: '../_piles/x?y=' })).rejects.toThrow(
      /not an entity address/
    )
    expect(api.get).not.toHaveBeenCalledWith(expect.stringContaining('_piles'), expect.anything())
  })
})

describe('channel requests', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
  })

  it('reads by the faced address, without a world', async () => {
    await listChannels(target, 'open')
    await getChannelHead(target)
    await getMessages({ ...target, name: 'x1' }, { afterRev: 0, limit: 50 })
    await getChannelSchema()
    expect(api.get).toHaveBeenNthCalledWith(1, '/tickets/TKT-1@draft/_channels', {
      state: 'open',
    })
    expect(api.get).toHaveBeenNthCalledWith(2, base)
    expect(api.get).toHaveBeenNthCalledWith(
      3,
      `${base}/x1/messages`,
      { after_rev: 0, limit: 50 },
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

  it('sends expect_generation when set, 0 included', async () => {
    axiosPost.mockResolvedValue({ status: 201, data: { seq: 3, rev: 3 } })
    await postMessage(target, 'approve', {}, 12, 4)
    expect(axiosPost).toHaveBeenLastCalledWith(`${base}/messages`, {
      type: 'approve',
      props: {},
      reply_to: 12,
      expect_generation: 4,
    })
    await postMessage(target, 'go', {}, undefined, 0)
    expect(axiosPost).toHaveBeenLastCalledWith(`${base}/messages`, {
      type: 'go',
      props: {},
      expect_generation: 0,
    })
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

  it('marks read up to a seq, or up to the head with an empty body', async () => {
    await markChannelRead({ ...target, name: 'x1' }, 7)
    expect(api.put).toHaveBeenLastCalledWith(`${base}/x1/_read`, { seq: 7 })
    await markChannelRead(target)
    expect(api.put).toHaveBeenLastCalledWith(`${base}/_read`, {})
  })

  it('reads the inbox and its summary', async () => {
    vi.mocked(api.get).mockClear()
    await getInbox()
    await getInbox({ limit: 10, kinds: ['waiting', 'mention'] })
    await getInboxSummary()
    expect(api.get).toHaveBeenNthCalledWith(1, '/_inbox', undefined)
    expect(api.get).toHaveBeenNthCalledWith(2, '/_inbox', { limit: 10, kinds: 'waiting,mention' })
    expect(api.get).toHaveBeenNthCalledWith(3, '/_inbox/summary')
  })
})

/** The text of a FormData part, read the way the server reads it. */
async function partText(v: FormDataEntryValue | Blob): Promise<string> {
  return typeof v === 'string' ? v : await new Response(v).text()
}

describe('posts with files', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
    axiosPost.mockResolvedValue({ status: 201, data: { seq: 5, rev: 5 } })
  })

  it('sends the message part first, then one file:<property> part per file', async () => {
    const shot = new File(['png'], 'shot.png', { type: 'image/png' })
    const log = new File(['log'], 'run.log', { type: 'text/plain' })
    const res = await postMessage(target, 'bug', { title: 'Crash' }, 2, 3, [
      { property: 'screenshots', file: shot },
      { property: 'logs', file: log },
    ])
    expect(res).toEqual({ kind: 'message', message: { seq: 5, rev: 5 } })

    const [url, form, config] = axiosPost.mock.lastCall!
    expect(url).toBe(`${base}/messages`)
    // The instance default is JSON, under which axios would serialize the
    // FormData; multipart keeps it, and the browser adds the boundary.
    expect(config).toEqual({ headers: { 'Content-Type': 'multipart/form-data' } })
    expect(form).toBeInstanceOf(FormData)
    const parts = [...(form as FormData).entries()]
    expect(parts.map(([name]) => name)).toEqual(['message', 'file:screenshots', 'file:logs'])

    // A plain field: a Blob part would carry a file name, which the
    // server refuses on the message part.
    const message = parts[0]![1]
    expect(typeof message).toBe('string')
    expect(JSON.parse(await partText(message))).toEqual({
      type: 'bug',
      props: { title: 'Crash' },
      reply_to: 2,
      expect_generation: 3,
    })
    expect((parts[1]![1] as File).name).toBe('shot.png')
    expect(await partText(parts[1]![1])).toBe('png')
    expect((parts[2]![1] as File).name).toBe('run.log')
  })

  it('keeps a post without files a plain JSON post', async () => {
    await postMessage(target, 'text', { body: 'hi' }, undefined, undefined, [])
    expect(axiosPost).toHaveBeenLastCalledWith(`${base}/messages`, {
      type: 'text',
      props: { body: 'hi' },
    })
  })

  it('reads an ephemeral answer of a multipart post', async () => {
    axiosPost.mockResolvedValueOnce({ status: 202, data: { id: 'e1', expires_at: 'x' } })
    const file = new File(['x'], 'x.txt')
    expect(
      await postMessage(target, 'ping', {}, undefined, undefined, [{ property: 'f', file }])
    ).toEqual({
      kind: 'ephemeral',
      accepted: { id: 'e1', expires_at: 'x' },
    })
  })
})

describe('fetchMessageFile', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
  })

  it('reads the file as a blob from the message file route', async () => {
    const blob = new Blob(['bytes'])
    axiosGet.mockResolvedValueOnce({ status: 200, data: blob })
    expect(await fetchMessageFile({ ...target, name: 'x1' }, 7, 'f/1')).toBe(blob)
    expect(axiosGet).toHaveBeenLastCalledWith(`${base}/x1/messages/7/files/f%2F1`, {
      responseType: 'blob',
      signal: undefined,
    })
  })

  it('reads a problem body that arrived as a blob, so the type survives', async () => {
    const problem = {
      type: 'https://rela.dev/errors/file_not_found',
      title: 'File not found',
      status: 404,
    }
    const original = Object.assign(new Error('404'), {
      isAxiosError: true,
      response: {
        status: 404,
        headers: {},
        data: new Blob([JSON.stringify(problem)], { type: 'application/problem+json' }),
      },
    })
    axiosGet.mockRejectedValueOnce(
      new ApiError('Request failed (404)', { kind: 'http', status: 404, original })
    )
    const err = await fetchMessageFile(target, 7, 'f1').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('file_not_found')
    expect((err as ApiError).message).toBe('File not found')
  })
})
