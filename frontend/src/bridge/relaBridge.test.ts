import { describe, it, expect, vi, beforeEach } from 'vitest'
import { dispatchBridgeRequest, BRIDGE_METHODS } from './relaBridge'
import * as entities from '@/api/entities'
import * as schema from '@/api/schema'
import * as actions from '@/api/actions'

vi.mock('@/api/entities')
vi.mock('@/api/schema')
vi.mock('@/api/actions')

describe('relaBridge dispatcher', () => {
  beforeEach(() => vi.clearAllMocks())

  it('rejects an unknown method with a structured error and NO api call', async () => {
    const res = await dispatchBridgeRequest({ id: 1, method: 'evil', params: {} })
    expect(res.ok).toBe(false)
    expect(res.error?.code).toBe('unknown_method')
    // No entity/schema/action call should have fired.
    expect(vi.mocked(entities.listEntities)).not.toHaveBeenCalled()
    expect(vi.mocked(schema.getSchema)).not.toHaveBeenCalled()
    expect(vi.mocked(actions.runAction)).not.toHaveBeenCalled()
  })

  it('rejects a path-like passthrough attempt (no arbitrary URL)', async () => {
    const res = await dispatchBridgeRequest({ id: 2, method: '/tickets/../secret', params: {} })
    expect(res.ok).toBe(false)
    expect(res.error?.code).toBe('unknown_method')
  })

  it('maps list → listEntities with the type and params', async () => {
    vi.mocked(entities.listEntities).mockResolvedValue({ data: [], meta: {} } as never)
    const res = await dispatchBridgeRequest({
      id: 3,
      method: 'list',
      params: { type: 'ticket', params: { per_page: 5 } },
    })
    expect(res.ok).toBe(true)
    expect(entities.listEntities).toHaveBeenCalledWith('ticket', { per_page: 5 })
  })

  it('maps create → createEntity', async () => {
    vi.mocked(entities.createEntity).mockResolvedValue({ id: 'X-1' } as never)
    const res = await dispatchBridgeRequest({
      id: 4,
      method: 'create',
      params: { type: 'ticket', entity: { properties: { title: 'hi' } } },
    })
    expect(res.ok).toBe(true)
    expect(res.result).toEqual({ id: 'X-1' })
    expect(entities.createEntity).toHaveBeenCalledWith('ticket', { properties: { title: 'hi' } })
  })

  it('maps relationCreate → createRelation', async () => {
    vi.mocked(entities.createRelation).mockResolvedValue(undefined as never)
    const res = await dispatchBridgeRequest({
      id: 5,
      method: 'relationCreate',
      params: { type: 'ticket', id: 'T-1', relation: 'depends_on', targetId: 'T-2' },
    })
    expect(res.ok).toBe(true)
    expect(entities.createRelation).toHaveBeenCalledWith('ticket', 'T-1', 'depends_on', 'T-2', undefined, undefined)
  })

  it('maps action → runAction', async () => {
    vi.mocked(actions.runAction).mockResolvedValue(null)
    const res = await dispatchBridgeRequest({ id: 6, method: 'action', params: { actionId: 'do_thing' } })
    expect(res.ok).toBe(true)
    expect(actions.runAction).toHaveBeenCalledWith('do_thing', undefined, undefined)
  })

  it('rejects missing required params before any api call', async () => {
    const res = await dispatchBridgeRequest({ id: 7, method: 'get', params: { type: 'ticket' } }) // no id
    expect(res.ok).toBe(false)
    expect(res.error?.code).toBe('invalid_params')
    expect(entities.getEntity).not.toHaveBeenCalled()
  })

  it('normalizes a backend rejection into a structured error', async () => {
    vi.mocked(entities.getEntity).mockRejectedValue(new Error('forbidden'))
    const res = await dispatchBridgeRequest({ id: 8, method: 'get', params: { type: 'ticket', id: 'T-1' } })
    expect(res.ok).toBe(false)
    expect(res.error?.code).toBe('request_failed')
    expect(res.error?.message).toBe('forbidden')
  })

  it('the allow-list contains only the documented methods', () => {
    expect(Object.keys(BRIDGE_METHODS).sort()).toEqual(
      [
        'action',
        'analyze',
        'config',
        'create',
        'delete',
        'get',
        'list',
        'position',
        'relationCreate',
        'relationDelete',
        'relationUpdate',
        'schema',
        'search',
        'templates',
        'update',
      ].sort(),
    )
  })
})
