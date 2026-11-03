import { describe, it, expect, vi } from 'vitest'
import { ApiError } from '@/api/errors'
import { DECISIONS_BLOCKED_KEY } from 'rela-components/components/channel/channelFields'
import { blockedPending, handleChannelRefusal } from './decisionGlue'

function problem(type: string, status = 409) {
  return new ApiError(type, {
    kind: 'http',
    status,
    problem: { type: `https://rela.dev/errors/${type}`, title: type, status },
    original: {},
  })
}

function steps() {
  return { refetch: vi.fn(), gone: vi.fn(), reloadSchema: vi.fn(), toast: vi.fn() }
}

describe('blockedPending', () => {
  it('adds the blocked key to the pending keys', () => {
    const pending = new Set(['decision:approve'])
    expect([...blockedPending(pending)]).toEqual(['decision:approve', DECISIONS_BLOCKED_KEY])
  })
})

describe('handleChannelRefusal', () => {
  it.each([
    { type: 'channel_post_denied', step: 'refetch' as const },
    { type: 'channel_not_found', step: 'gone' as const },
    { type: 'config_changed', step: 'reloadSchema' as const },
  ])('$type runs $step and toasts', ({ type, step }) => {
    const s = steps()
    expect(handleChannelRefusal(problem(type), 'form', s)).toBeUndefined()
    expect(s[step]).toHaveBeenCalledOnce()
    expect(s.toast).toHaveBeenCalledOnce()
  })

  it('a form refusal returns its message with a form, and toasts without', () => {
    const s = steps()
    expect(handleChannelRefusal(problem('invalid_message', 400), 'form', s)).toBeTruthy()
    expect(s.toast).not.toHaveBeenCalled()
    expect(handleChannelRefusal(problem('invalid_message', 400), 'toast', s)).toBeUndefined()
    expect(s.toast).toHaveBeenCalledOnce()
  })
})
