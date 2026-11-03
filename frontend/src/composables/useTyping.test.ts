import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import { ApiError } from '@/api/errors'
import type { ChannelTarget } from '@/api/channels'
import type { TypingAccepted } from '@/types/channels'
import { TYPING_BACKOFF_MS, useTyping } from './useTyping'

// The composer's typing events become at most one POST …/_typing per half
// TTL. A refusal or failure backs off quietly; a post resets the throttle
// (TKT-9ODC1E). The composer itself emits only for a log type and never
// while editing; see the EmitsTyping story.

const sendMock = vi.fn<(t: ChannelTarget) => Promise<TypingAccepted>>()
vi.mock('@/api/channels', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/channels')>()),
  sendTyping: (t: ChannelTarget) => sendMock(t),
}))

const room: ChannelTarget = { type: 'ticket', address: 'TKT-1', channelType: 'room' }

function httpError(status: number) {
  return new ApiError(`HTTP ${status}`, { kind: 'http', status, original: null })
}

describe('useTyping', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(1_000_000)
    sendMock.mockReset()
    sendMock.mockResolvedValue({ ttl_seconds: 6 })
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('sends at most once per half TTL', async () => {
    const typing = useTyping(room, () => true)
    typing.onInput()
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(1)
    expect(sendMock).toHaveBeenCalledWith(room)

    vi.advanceTimersByTime(2999)
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(1)
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(2)
  })

  it('sends nothing while typing is not allowed', async () => {
    const allowed = ref(false)
    const typing = useTyping(room, () => allowed.value)
    typing.onInput()
    await flushPromises()
    expect(sendMock).not.toHaveBeenCalled()
    allowed.value = true
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['a 429', httpError(429)],
    ['a 403', httpError(403)],
    ['a network failure', new Error('offline')],
  ])('backs off after %s without throwing', async (_name, err) => {
    sendMock.mockRejectedValueOnce(err)
    const typing = useTyping(room, () => true)
    typing.onInput()
    await flushPromises()
    vi.advanceTimersByTime(TYPING_BACKOFF_MS - 1)
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(1)
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(2)
  })

  it('sends again on the next keystroke after a post', async () => {
    const typing = useTyping(room, () => true)
    typing.onInput()
    await flushPromises()
    typing.reset()
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(2)
  })

  it('starts clean on another channel', async () => {
    const target = ref<ChannelTarget>(room)
    const typing = useTyping(target, () => true)
    typing.onInput()
    await flushPromises()
    target.value = { ...room, channelType: 'chat' }
    await nextTick()
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(2)
    expect(sendMock).toHaveBeenLastCalledWith({ ...room, channelType: 'chat' })
  })

  it('settles a pending send before the post and sends nothing until reset', async () => {
    let resolve!: (a: TypingAccepted) => void
    sendMock.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const typing = useTyping(room, () => true)
    typing.onInput()
    let settled = false
    const done = typing.settle().then(() => (settled = true))
    await flushPromises()
    expect(settled).toBe(false)
    resolve({ ttl_seconds: 6 })
    await done
    expect(settled).toBe(true)

    vi.advanceTimersByTime(10_000)
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(1)
    typing.reset()
    typing.onInput()
    await flushPromises()
    expect(sendMock).toHaveBeenCalledTimes(2)
  })

  it('settles at once with nothing pending, and after a failed send', async () => {
    const typing = useTyping(room, () => true)
    await typing.settle()
    typing.reset()
    sendMock.mockRejectedValueOnce(new Error('offline'))
    typing.onInput()
    await expect(typing.settle()).resolves.toBeUndefined()
  })

  it('does not send twice while a call is in flight', async () => {
    let resolve!: (a: TypingAccepted) => void
    sendMock.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const typing = useTyping(room, () => true)
    typing.onInput()
    typing.reset()
    typing.onInput()
    expect(sendMock).toHaveBeenCalledTimes(1)
    resolve({ ttl_seconds: 6 })
    await flushPromises()
  })
})
