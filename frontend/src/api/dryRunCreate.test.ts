import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { dryRunCreateEntity } from './entities'
import { api } from './client'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'

vi.mock('./client', () => ({
  api: { post: vi.fn() },
}))

describe('dryRunCreateEntity (TKT-3I5U)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setActivePinia(createPinia())
    // Seed a plural so getPlural resolves ticket -> tickets.
    const schema = useSchemaStore()
    schema.entityTypes.set('ticket', {
      name: 'ticket',
      plural: 'tickets',
      properties: {},
    } as never)
    vi.mocked(api.post).mockResolvedValue({
      id: '',
      type: 'ticket',
      properties: { title: 'x' },
      _fields: { status: { writable: false } },
    } as Entity)
  })

  it('POSTs to the collection with ?dry_run=true', async () => {
    await dryRunCreateEntity('ticket', { properties: { title: 'x' } })
    const [url] = vi.mocked(api.post).mock.calls[0]
    expect(url).toBe('/tickets?dry_run=true')
  })

  it('sends properties + content but NEVER relations or the staged sentinel', async () => {
    await dryRunCreateEntity('ticket', {
      properties: { title: 'x', status: 'open' },
      content: 'body',
    })
    const [, body] = vi.mocked(api.post).mock.calls[0]
    const sent = body as Record<string, unknown>
    expect(sent.properties).toEqual({ title: 'x', status: 'open' })
    expect(sent.content).toBe('body')
    // The form-only sentinel must never reach the wire (no id field, and
    // certainly not '++new++').
    expect(JSON.stringify(sent)).not.toContain('++new++')
    expect('relations' in sent).toBe(false)
  })

  it('forwards an AbortSignal for stale-drop', async () => {
    const controller = new AbortController()
    await dryRunCreateEntity('ticket', { properties: {} }, controller.signal)
    const opts = vi.mocked(api.post).mock.calls[0][2] as { signal?: AbortSignal }
    expect(opts?.signal).toBe(controller.signal)
  })

  it('returns the server verdict (_fields) for the form to consume', async () => {
    const res = await dryRunCreateEntity('ticket', { properties: { title: 'x' } })
    expect(res._fields?.status?.writable).toBe(false)
  })
})
