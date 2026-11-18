import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  isProvisioningResponse,
  resetProvisioningForTest,
  startProvisioningPoll,
  tenantProvisioning,
} from './tenantProvisioning'

function json(status: number, body: unknown): Response {
  const res: Pick<Response, 'ok' | 'status' | 'json'> = {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  }
  return res as Response
}

describe('tenant provisioning', () => {
  afterEach(() => resetProvisioningForTest())

  it('recognizes only the provisioning answers', () => {
    const problem = (code: string) => ({ type: `https://rela.dev/errors/${code}` })
    expect(isProvisioningResponse(202, { status: 'provisioning' })).toBe(true)
    for (const code of [
      'tenant_provisioning',
      'provisioning_busy',
      'provisioning_failed',
      'provisioning_needs_operator',
    ]) {
      expect(isProvisioningResponse(503, problem(code))).toBe(true)
    }
    expect(isProvisioningResponse(503, problem('tenant_unavailable'))).toBe(false)
    expect(isProvisioningResponse(200, { status: 'provisioning' })).toBe(false)
    expect(isProvisioningResponse(202, problem('tenant_provisioning'))).toBe(false)
    expect(isProvisioningResponse(202, [{ id: 'TSK-1' }])).toBe(false)
    expect(isProvisioningResponse(202, null)).toBe(false)
  })

  it('polls until active, then reloads once', async () => {
    const responses = [
      json(200, { status: 'provisioning' }),
      json(503, { type: 'https://rela.dev/errors/provisioning_failed' }),
      json(200, { status: 'active' }),
    ]
    const fetchFn = vi.fn(() => Promise.resolve(responses.shift()!))
    const states: string[] = []
    const wait = vi.fn(() => {
      states.push(tenantProvisioning.value)
      return Promise.resolve()
    })
    const reload = vi.fn()

    const run = startProvisioningPoll({ fetchFn: fetchFn as unknown as typeof fetch, wait, reload })
    // A second 202 does not start a second poll.
    expect(
      startProvisioningPoll({ fetchFn: fetchFn as unknown as typeof fetch, wait, reload })
    ).toBeUndefined()
    await run

    expect(fetchFn).toHaveBeenCalledTimes(3)
    expect(states).toEqual(['provisioning', 'failed'])
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('reloads when the organization turns out suspended', async () => {
    const fetchFn = vi.fn(() => Promise.resolve(json(200, { status: 'suspended' })))
    const wait = vi.fn(() => Promise.resolve())
    const reload = vi.fn()
    await startProvisioningPoll({ fetchFn: fetchFn as unknown as typeof fetch, wait, reload })
    expect(wait).not.toHaveBeenCalled()
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it.each([401, 403])('stops polling and reloads on %i', async (status) => {
    const fetchFn = vi.fn(() => Promise.resolve(json(status, {})))
    const wait = vi.fn(() => Promise.resolve())
    const reload = vi.fn()
    await startProvisioningPoll({ fetchFn: fetchFn as unknown as typeof fetch, wait, reload })
    expect(fetchFn).toHaveBeenCalledTimes(1)
    expect(wait).not.toHaveBeenCalled()
    expect(reload).toHaveBeenCalledTimes(1)
  })
})
