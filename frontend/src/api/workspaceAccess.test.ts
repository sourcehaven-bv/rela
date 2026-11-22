// The whole-workspace refusals: an ended session, a server that cannot
// identify anyone, and a suspended, archived or unknown workspace. The client
// records them; the App shows one page.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios'
import { api } from './client'
import {
  accessRefusal,
  loginUrl,
  loginUrlOf,
  probeSessionEnded,
  refusalDetail,
  refusalOf,
  resetAccessForTest,
  trackAccess,
} from './workspaceAccess'

const problem = (code: string) => ({
  type: `https://rela.dev/errors/${code}`,
  title: code,
  status: 403,
})

describe('refusalOf', () => {
  it('names each whole-workspace refusal and nothing else', () => {
    expect(refusalOf(401, null)).toBe('session_ended')
    expect(refusalOf(401, { error: 'session_expired' })).toBe('session_ended')
    // A 401 that explains itself and names no sign-in page: signing in again
    // cannot help.
    expect(refusalOf(401, { status: 401, detail: 'no user identity' })).toBe('no_identity')
    expect(refusalOf(401, { detail: 'expired', login_url: '/auth/login?rd=%2Ftickets' })).toBe(
      'session_ended'
    )
    expect(refusalOf(403, problem('tenant_suspended'))).toBe('suspended')
    expect(refusalOf(403, problem('tenant_archived'))).toBe('archived')
    expect(refusalOf(403, problem('unknown_tenant'))).toBe('unknown_workspace')
    // A refused write on a read-only workspace, a missing permission and an
    // unrelated status are about one request, not the workspace.
    expect(refusalOf(403, problem('tenant_read_only'))).toBeNull()
    expect(refusalOf(403, problem('forbidden'))).toBeNull()
    expect(refusalOf(409, problem('tenant_suspended'))).toBeNull()
    expect(refusalOf(403, problem('constructor'))).toBeNull()
  })
})

describe('loginUrlOf', () => {
  it('accepts a local path and nothing else', () => {
    expect(loginUrlOf({ login_url: '/auth/login?rd=%2Ftickets' })).toBe('/auth/login?rd=%2Ftickets')
    // Another site, however it is written.
    expect(loginUrlOf({ login_url: 'https://account.example/login' })).toBeNull()
    expect(loginUrlOf({ login_url: `${window.location.origin}/auth/login` })).toBeNull()
    expect(loginUrlOf({ login_url: '//evil.example/login' })).toBeNull()
    expect(loginUrlOf({ login_url: '/\\evil.example/login' })).toBeNull()
    expect(loginUrlOf({ login_url: '/\t/evil.example/login' })).toBeNull()
    expect(loginUrlOf({ login_url: 'javascript:alert(1)' })).toBeNull()
    expect(loginUrlOf({ login_url: 'auth/login' })).toBeNull()
    expect(loginUrlOf({ login_url: 42 })).toBeNull()
    expect(loginUrlOf(null)).toBeNull()
  })
})

describe('trackAccess', () => {
  afterEach(resetAccessForTest)

  it('keeps the first refusal and the sign-in page of a 401', () => {
    trackAccess(401, { error: 'session_expired', login_url: '/auth/login?rd=%2Ftickets' })
    trackAccess(403, problem('tenant_suspended'))
    expect(accessRefusal.value).toBe('session_ended')
    expect(loginUrl.value).toBe('/auth/login?rd=%2Ftickets')
  })

  it('keeps the explanation of a 401 that names no sign-in page', () => {
    trackAccess(401, {
      ...problem('unauthenticated'),
      status: 401,
      detail: 'no user identity: see docs',
    })
    expect(accessRefusal.value).toBe('no_identity')
    expect(refusalDetail.value).toBe('no user identity: see docs')
    expect(loginUrl.value).toBeNull()
  })

  it('ignores an ordinary answer', () => {
    trackAccess(200, {})
    trackAccess(404, problem('not_found'))
    expect(accessRefusal.value).toBeNull()
  })
})

describe('probeSessionEnded', () => {
  afterEach(resetAccessForTest)

  const respond = (init: Partial<Response>) =>
    vi.fn().mockResolvedValue({ json: async () => ({}), ...init })

  it('reads a redirect the browser would not follow as an ended session', async () => {
    const fetchFn = respond({ type: 'opaqueredirect', status: 0 })
    await probeSessionEnded(fetchFn)
    expect(fetchFn.mock.calls[0][1]).toMatchObject({ redirect: 'manual' })
    expect(accessRefusal.value).toBe('session_ended')
  })

  it('reads a 401 and its sign-in page', async () => {
    const fetchFn = respond({
      type: 'basic',
      status: 401,
      json: async () => ({ error: 'session_expired', login_url: '/auth/login?rd=%2Ftickets' }),
    })
    await probeSessionEnded(fetchFn)
    expect(accessRefusal.value).toBe('session_ended')
    expect(loginUrl.value).toBe('/auth/login?rd=%2Ftickets')
  })

  it('leaves a server that answers, or does not answer at all, alone', async () => {
    let clock = 0
    const now = () => clock
    await probeSessionEnded(respond({ type: 'basic', status: 404 }), now)
    clock += 60_000
    await probeSessionEnded(vi.fn().mockRejectedValue(new TypeError('Failed to fetch')), now)
    expect(accessRefusal.value).toBeNull()
  })

  it('runs one probe at a time and at most one per cooldown', async () => {
    let clock = 0
    const now = () => clock
    let release: (r: unknown) => void = () => {}
    const fetchFn = vi.fn(
      () =>
        new Promise((resolve) => {
          release = resolve
        })
    ) as unknown as typeof fetch
    const first = probeSessionEnded(fetchFn, now)
    const second = probeSessionEnded(fetchFn, now)
    expect(second).toBe(first)
    release({ type: 'basic', status: 200 })
    await first
    clock += 1_000
    await probeSessionEnded(fetchFn, now)
    expect(fetchFn).toHaveBeenCalledTimes(1)
    clock += 30_000
    void probeSessionEnded(fetchFn, now)
    expect(fetchFn).toHaveBeenCalledTimes(2)
  })
})

// Through the real API client, so the interceptor wiring is what is tested.
describe('API client', () => {
  const realAdapter = api.getAxios().defaults.adapter
  let fetchSpy: ReturnType<typeof vi.spyOn>

  function refuse(status: number, data: unknown): AxiosAdapter {
    return (config: InternalAxiosRequestConfig) =>
      Promise.reject(
        new AxiosError('refused', 'ERR_BAD_REQUEST', config, null, {
          data,
          status,
          statusText: '',
          headers: {},
          config,
        })
      )
  }

  beforeEach(() => {
    fetchSpy = vi.spyOn(globalThis, 'fetch')
  })
  afterEach(() => {
    api.getAxios().defaults.adapter = realAdapter
    fetchSpy.mockRestore()
    resetAccessForTest()
  })

  it('records a suspended workspace from any request', async () => {
    api.getAxios().defaults.adapter = refuse(403, problem('tenant_suspended'))
    await expect(api.get('/_config')).rejects.toThrow()
    expect(accessRefusal.value).toBe('suspended')
  })

  it("records the edge's 401 for an ended session", async () => {
    api.getAxios().defaults.adapter = refuse(401, {
      error: 'session_expired',
      login_url: '/auth/login?rd=%2Ftickets',
    })
    await expect(api.get('/tickets')).rejects.toThrow()
    expect(accessRefusal.value).toBe('session_ended')
    expect(loginUrl.value).toBe('/auth/login?rd=%2Ftickets')
  })

  it('asks whether a request that got no response met a sign-in redirect', async () => {
    fetchSpy.mockResolvedValue({ type: 'opaqueredirect', status: 0 } as Response)
    api.getAxios().defaults.adapter = (config) =>
      Promise.reject(new AxiosError('Network Error', 'ERR_NETWORK', config, {}, undefined))
    await expect(api.get('/tickets')).rejects.toMatchObject({ kind: 'network' })
    await vi.waitFor(() => expect(accessRefusal.value).toBe('session_ended'))
  })
})
