import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  isReadOnlyResponse,
  loadWorkspaceStatus,
  trackWorkspaceReadOnly,
  workspaceReadOnly,
} from './workspaceReadOnly'
import { accessRefusal, resetAccessForTest } from './workspaceAccess'
import { actionAllowed } from '@/utils/affordancesWarning'

const refused = { type: 'https://rela.dev/errors/tenant_read_only' }

describe('workspace read-only flag', () => {
  afterEach(() => {
    workspaceReadOnly.value = false
  })

  it('matches only a 403 with the tenant_read_only problem type', () => {
    expect(isReadOnlyResponse(403, refused)).toBe(true)
    expect(isReadOnlyResponse(409, refused)).toBe(false)
    expect(isReadOnlyResponse(403, { type: 'https://rela.dev/errors/forbidden' })).toBe(false)
    expect(isReadOnlyResponse(403, undefined)).toBe(false)
  })

  it('is set by any refused write and cleared only by a write that succeeds', () => {
    trackWorkspaceReadOnly({ method: 'patch', url: '/tickets/TKT-1' }, 403, refused)
    expect(workspaceReadOnly.value).toBe(true)

    // Reads, and the subscriptions PUT a read-only workspace still accepts,
    // say nothing about writes.
    trackWorkspaceReadOnly({ method: 'get', url: '/tickets' }, 200, [])
    trackWorkspaceReadOnly({ method: 'put', url: '/_events/c1/subscriptions' }, 204, '')
    trackWorkspaceReadOnly({ method: 'post', url: '/tickets' }, 500, {})
    expect(workspaceReadOnly.value).toBe(true)

    trackWorkspaceReadOnly({ method: 'post', url: '/tickets?dry_run=true' }, 200, {})
    expect(workspaceReadOnly.value).toBe(false)
  })
})

describe('loadWorkspaceStatus', () => {
  afterEach(() => {
    workspaceReadOnly.value = false
    resetAccessForTest()
  })

  const answer = (status: number, body: unknown) =>
    vi.fn().mockResolvedValue({ ok: status < 300, status, json: async () => body })

  it('reads the flag from the tenant status at start', async () => {
    await loadWorkspaceStatus(answer(200, { status: 'active', read_only: true }))
    expect(workspaceReadOnly.value).toBe(true)
    await loadWorkspaceStatus(answer(200, { status: 'active' }))
    expect(workspaceReadOnly.value).toBe(false)
  })

  it('leaves the flag alone on a server with no tenant status', async () => {
    workspaceReadOnly.value = true
    await loadWorkspaceStatus(answer(404, {}))
    await loadWorkspaceStatus(vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    expect(workspaceReadOnly.value).toBe(true)
    expect(accessRefusal.value).toBeNull()
  })

  it.each(['suspended', 'archived'] as const)(
    'records a %s workspace from its status, without a refused request',
    async (status) => {
      await loadWorkspaceStatus(answer(200, { status }))
      expect(accessRefusal.value).toBe(status)
      expect(workspaceReadOnly.value).toBe(false)
    }
  )

  it('records a refused status request like any refused request', async () => {
    await loadWorkspaceStatus(
      answer(403, { type: 'https://rela.dev/errors/unknown_tenant', status: 403 })
    )
    expect(accessRefusal.value).toBe('unknown_workspace')
  })

  it('keeps the first refusal', async () => {
    await loadWorkspaceStatus(answer(200, { status: 'suspended' }))
    await loadWorkspaceStatus(answer(200, { status: 'archived' }))
    expect(accessRefusal.value).toBe('suspended')
  })

  it('records nothing for a workspace that is active or being set up', async () => {
    await loadWorkspaceStatus(answer(200, { status: 'provisioning' }))
    await loadWorkspaceStatus(answer(200, { status: 'active' }))
    expect(accessRefusal.value).toBeNull()
  })

  it('gives up when the status does not answer in time', async () => {
    const hang = vi.fn(
      (_input: RequestInfo | URL, init?: RequestInit) =>
        new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener('abort', () => reject(init.signal?.reason))
        })
    )
    await loadWorkspaceStatus(hang, 10)
    expect(hang).toHaveBeenCalled()
    expect(accessRefusal.value).toBeNull()
  })
})

describe('create affordances on a read-only workspace', () => {
  afterEach(() => {
    workspaceReadOnly.value = false
  })

  it('hides every write verb whatever the verdict', () => {
    const verbs = ['create', 'create@draft', 'update', 'delete', 'rename']
    const carrier = { _actions: Object.fromEntries(verbs.map((v) => [v, true])) }
    workspaceReadOnly.value = true
    for (const verb of verbs) expect(actionAllowed(carrier, verb)).toBe(false)
    expect(actionAllowed(undefined, 'update')).toBe(false)
    workspaceReadOnly.value = false
    for (const verb of verbs) expect(actionAllowed(carrier, verb)).toBe(true)
  })
})
