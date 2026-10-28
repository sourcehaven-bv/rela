import { afterEach, describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { ApiError } from '@/api/errors'
import { CHANNEL_ERROR_RULES, classifyChannelError, problemType } from './channelErrors'
import { resetLanguageState } from '@/i18n/locale'

// The refusal types the channel service declares. Every one needs a rule,
// so a type the server gains cannot fall through to "toast and do nothing"
// unnoticed (TKT-91BK6F AC 14).
const ERRORS_GO = resolve(__dirname, '../../../../../internal/realtime/errors.go')

function goTypes(): string[] {
  const src = readFileSync(ERRORS_GO, 'utf-8')
  return [...src.matchAll(/^\s*Type[A-Za-z]+\s*=\s*"([a-z_]+)"/gm)].map((m) => m[1])
}

function problem(status: number, type: string, title = 'refused', detail?: string, headers = {}) {
  return new ApiError(title, {
    kind: 'http',
    status,
    problem: { type: `https://rela.dev/errors/${type}`, title, status, detail },
    original: { response: { headers } },
  })
}

describe('CHANNEL_ERROR_RULES', () => {
  it('has a rule for every refusal type in internal/realtime/errors.go', () => {
    const types = goTypes()
    expect(types.length).toBeGreaterThan(30)
    expect(types.filter((t) => !(t in CHANNEL_ERROR_RULES))).toEqual([])
  })

  it('has no rule for a type the server does not declare', () => {
    const types = new Set(goTypes())
    expect(Object.keys(CHANNEL_ERROR_RULES).filter((t) => !types.has(t))).toEqual([])
  })
})

describe('classifyChannelError', () => {
  const cases: {
    name: string
    err: unknown
    place: 'form' | 'toast'
    then: 'none' | 'refetch' | 'list' | 'schema'
    message?: string
  }[] = [
    {
      name: 'invalid message shows in the form',
      err: problem(400, 'invalid_message'),
      place: 'form',
      then: 'none',
    },
    {
      name: 'reason required shows in the form',
      err: problem(400, 'reason_required'),
      place: 'form',
      then: 'none',
    },
    {
      name: 'invalid ref shows in the form',
      err: problem(422, 'invalid_ref'),
      place: 'form',
      then: 'none',
    },
    {
      name: 'too many mentions shows in the form',
      err: problem(422, 'too_many_mentions', 'a message may mention at most 10 people or agents'),
      place: 'form',
      then: 'none',
      message: 'a message may mention at most 10 people or agents',
    },
    {
      name: 'too large shows in the form',
      err: problem(413, 'too_large'),
      place: 'form',
      then: 'none',
    },
    {
      name: 'closed channel refetches',
      err: problem(409, 'channel_not_open'),
      place: 'toast',
      then: 'refetch',
    },
    {
      name: 'post denied refetches',
      err: problem(403, 'channel_post_denied'),
      place: 'toast',
      then: 'refetch',
    },
    {
      name: 'edit window closed refetches',
      err: problem(403, 'edit_window_closed'),
      place: 'toast',
      then: 'refetch',
    },
    {
      name: 'action taken refetches',
      err: problem(409, 'action_taken'),
      place: 'toast',
      then: 'refetch',
    },
    {
      name: 'missing message refetches',
      err: problem(404, 'message_not_found'),
      place: 'toast',
      then: 'refetch',
    },
    {
      name: 'missing channel returns to the list',
      err: problem(404, 'channel_not_found'),
      place: 'toast',
      then: 'list',
    },
    {
      name: 'gone message type reloads the descriptor',
      err: problem(409, 'message_type_gone'),
      place: 'toast',
      then: 'schema',
    },
    {
      name: 'config change reloads the descriptor',
      err: problem(409, 'config_changed'),
      place: 'toast',
      then: 'schema',
    },
    {
      name: 'too many channels only says so',
      err: problem(409, 'too_many_channels'),
      place: 'toast',
      then: 'none',
    },
    {
      name: 'entity 404 refetches',
      err: problem(404, 'not_found'),
      place: 'toast',
      then: 'refetch',
    },
    {
      name: 'server error only says so',
      err: new ApiError('boom', { kind: 'http', status: 500, original: null }),
      place: 'toast',
      then: 'none',
    },
    {
      name: 'network failure only says so',
      err: new ApiError('offline', { kind: 'network', original: null }),
      place: 'toast',
      then: 'none',
    },
  ]

  for (const tc of cases) {
    it(tc.name, () => {
      const out = classifyChannelError(tc.err)
      expect({ place: out.place, then: out.then }).toEqual({ place: tc.place, then: tc.then })
      expect(out.message).not.toBe('')
      if (tc.message) expect(out.message).toBe(tc.message)
    })
  }

  it('shows the title as written and names the rule a 403 gives', () => {
    const out = classifyChannelError(
      problem(403, 'channel_post_denied', 'posting is not allowed', 'post: [member]')
    )
    expect(out.message).toBe('posting is not allowed (post: [member])')
  })

  it('says how long to wait from Retry-After on a 429', () => {
    const out = classifyChannelError(
      problem(429, 'rate_limited', 'slow down', undefined, { 'retry-after': '7' })
    )
    expect(out).toEqual({
      place: 'toast',
      then: 'none',
      message: 'Too many messages. Try again in 7 s.',
    })
  })

  it('says to wait without a Retry-After', () => {
    expect(classifyChannelError(problem(429, 'rate_limited')).message).toBe(
      'Too many messages. Try again shortly.'
    )
  })
})

describe('session_wrong_state', () => {
  const err = () =>
    problem(
      409,
      'session_wrong_state',
      'approve may be posted only while the session is in plan-review; it is in working',
      'channel_types.run.message_types.approve.in'
    )

  afterEach(() => resetLanguageState({ user: 'en' }))

  it('refetches the channel with a translated toast', () => {
    resetLanguageState({ user: 'en' })
    expect(classifyChannelError(err())).toEqual({
      place: 'toast',
      then: 'refetch',
      message: "This message type cannot be posted in the session's current state. Choose another.",
    })
  })

  it('says so in Dutch', () => {
    resetLanguageState({ user: 'nl' })
    expect(classifyChannelError(err()).message).toBe(
      'Dit berichttype kan in de huidige stand van de sessie niet worden geplaatst. Kies een ander.'
    )
  })
})

describe('problemType', () => {
  it('strips the problem type prefix', () => {
    expect(problemType(problem(409, 'removed'))).toBe('removed')
  })

  it('is empty for anything that is not an ApiError', () => {
    expect(problemType(new Error('x'))).toBe('')
  })
})
