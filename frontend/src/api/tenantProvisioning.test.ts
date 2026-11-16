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

  it('recognizes only the provisioning problems', () => {
    const problem = { type: 'https://rela.dev/errors/tenant_provisioning' }
    expect(isProvisioningResponse(202, problem)).toBe(true)
    expect(
      isProvisioningResponse(503, { type: 'https://rela.dev/errors/tenant_provisioning_busy' })
    ).toBe(true)
    expect(
      isProvisioningResponse(503, { type: 'https://rela.dev/errors/tenant_provisioning_failed' })
    ).toBe(true)
    expect(isProvisioningResponse(503, problem)).toBe(false)
    expect(
      isProvisioningResponse(503, { type: 'https://rela.dev/errors/tenant_unavailable' })
    ).toBe(false)
    expect(isProvisioningResponse(200, problem)).toBe(false)
    expect(isProvisioningResponse(202, { type: 'https://rela.dev/errors/other' })).toBe(false)
    expect(isProvisioningResponse(202, [{ id: 'TSK-1' }])).toBe(false)
    expect(isProvisioningResponse(202, null)).toBe(false)
  })

  it('polls until ready, then reloads once', async () => {
    const responses = [
      json(202, { status: 'provisioning' }),
      json(503, { type: 'https://rela.dev/errors/tenant_provisioning_failed' }),
      json(200, { status: 'ready' }),
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
