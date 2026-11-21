import { afterEach, describe, expect, it } from 'vitest'
import { isReadOnlyResponse, trackWorkspaceReadOnly, workspaceReadOnly } from './workspaceReadOnly'

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
