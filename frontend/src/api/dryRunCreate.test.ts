import { describe, it, expect, vi, beforeEach } from 'vitest'
import { dryRunCreateEntity, _setEntityPluralForTest } from './entities'
import { api } from './client'
import type { Entity } from '@/types'

vi.mock('./client', () => ({
  api: { post: vi.fn() },
}))

describe('dryRunCreateEntity (TKT-3I5U)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Register a plural so getPlural resolves ticket -> tickets. The API
    // layer no longer reads the schema store (B1a).
    _setEntityPluralForTest('ticket', 'tickets')
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

  // The dry-run gates fields per keystroke, so it must be judged against the
  // SAME face the submit will write. The server maps `world` through
  // `worlds.<name>.create`; sending it on one call and not the other would
  // gate the form against a row it is not writing.
  it('forwards the world so the verdict matches the create', async () => {
    await dryRunCreateEntity('ticket', {
      properties: { title: 'x' },
      world: 'editorial',
    })
    const [, body] = vi.mocked(api.post).mock.calls[0]
    expect((body as Record<string, unknown>).world).toBe('editorial')
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
