import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useMentionMenu, rankByIdMatch } from './useMentionMenu'
import type { Entity } from '@/types'

const searchEntities = vi.fn()
vi.mock('@/api', () => ({
  searchEntities: (...args: unknown[]) => searchEntities(...args),
  listEntities: vi.fn(),
}))

function ent(id: string, title = id): Entity {
  return { id, type: 'ticket', title, properties: {} } as unknown as Entity
}

/** Advances past the debounce and lets pending promises settle. */
async function settle() {
  await vi.advanceTimersByTimeAsync(200)
  await Promise.resolve()
}

describe('rankByIdMatch', () => {
  it('puts an exact id match first, then prefix, then substring', () => {
    const items = [ent('OTHER'), ent('TKT-ABCD'), ent('TKT-AB'), ent('XTKT-AB')]
    expect(rankByIdMatch(items, 'TKT-AB').map((e) => e.id)).toEqual([
      'TKT-AB',
      'TKT-ABCD',
      'XTKT-AB',
      'OTHER',
    ])
  })

  it('is stable within a tier so backend relevance survives', () => {
    const items = [ent('TKT-B'), ent('TKT-A')]
    expect(rankByIdMatch(items, 'TKT').map((e) => e.id)).toEqual(['TKT-B', 'TKT-A'])
  })

  it('leaves the order alone for an empty query', () => {
    const items = [ent('B'), ent('A')]
    expect(rankByIdMatch(items, '')).toBe(items)
  })
})

describe('useMentionMenu', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    searchEntities.mockReset()
    searchEntities.mockResolvedValue({ data: [ent('TKT-ABC')] })
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('starts closed', () => {
    const m = useMentionMenu()
    expect(m.state.open).toBe(false)
    expect(m.state.items).toEqual([])
  })

  it('opens and prompts below the minimum query length without searching', async () => {
    const m = useMentionMenu()
    m.setQuery('a')
    await settle()
    expect(m.state.open).toBe(true)
    expect(m.state.items).toEqual([])
    expect(searchEntities).not.toHaveBeenCalled()
  })

  it('searches once the query is long enough', async () => {
    const m = useMentionMenu()
    m.setQuery('TKT')
    await settle()
    expect(searchEntities).toHaveBeenCalledWith('TKT', undefined, expect.anything())
    expect(m.state.items.map((e) => e.id)).toEqual(['TKT-ABC'])
  })

  it('searches across every type rather than one', async () => {
    const m = useMentionMenu()
    m.setQuery('TKT')
    await settle()
    // The second argument is the type filter; undefined means all types.
    expect(searchEntities.mock.calls[0][1]).toBeUndefined()
  })

  it('debounces so typing does not fire a request per keystroke', async () => {
    const m = useMentionMenu()
    m.setQuery('TK')
    m.setQuery('TKT')
    m.setQuery('TKT-')
    await settle()
    expect(searchEntities).toHaveBeenCalledTimes(1)
    expect(searchEntities.mock.calls[0][0]).toBe('TKT-')
  })

  it('clears results when the query is backspaced below the minimum', async () => {
    const m = useMentionMenu()
    m.setQuery('TKT')
    await settle()
    expect(m.state.items).toHaveLength(1)
    m.setQuery('T')
    await settle()
    expect(m.state.items).toEqual([])
  })

  it('ignores a slow response for a superseded query', async () => {
    const m = useMentionMenu()
    let releaseFirst: (v: unknown) => void = () => {}
    searchEntities.mockImplementationOnce(
      () => new Promise((res) => (releaseFirst = res))
    )
    m.setQuery('old')
    await vi.advanceTimersByTimeAsync(200)

    searchEntities.mockResolvedValueOnce({ data: [ent('TKT-NEW')] })
    m.setQuery('new')
    await settle()
    expect(m.state.items.map((e) => e.id)).toEqual(['TKT-NEW'])

    releaseFirst({ data: [ent('TKT-OLD')] })
    await Promise.resolve()
    await Promise.resolve()
    expect(m.state.items.map((e) => e.id)).toEqual(['TKT-NEW'])
  })

  it('does not reopen after close when a response lands late', async () => {
    const m = useMentionMenu()
    let release: (v: unknown) => void = () => {}
    searchEntities.mockImplementationOnce(() => new Promise((res) => (release = res)))
    m.setQuery('TKT')
    await vi.advanceTimersByTimeAsync(200)
    m.close()
    release({ data: [ent('TKT-LATE')] })
    await Promise.resolve()
    await Promise.resolve()
    expect(m.state.open).toBe(false)
    expect(m.state.items).toEqual([])
  })

  it('reports a search failure without leaving stale results', async () => {
    const m = useMentionMenu()
    searchEntities.mockRejectedValueOnce(new Error('nope'))
    m.setQuery('TKT')
    await settle()
    expect(m.state.errorMsg).toBe('Search failed')
    expect(m.state.items).toEqual([])
    expect(m.state.loading).toBe(false)
  })

  it('does not report an aborted request as a failure', async () => {
    const m = useMentionMenu()
    const abortErr = Object.assign(new Error('aborted'), { name: 'AbortError' })
    searchEntities.mockRejectedValueOnce(abortErr)
    m.setQuery('TKT')
    await settle()
    expect(m.state.errorMsg).toBe('')
  })

  it('wraps the highlight in both directions', async () => {
    const m = useMentionMenu()
    searchEntities.mockResolvedValueOnce({
      data: [ent('A'), ent('B'), ent('C')],
    })
    m.setQuery('TKT')
    await settle()
    expect(m.state.highlightedIndex).toBe(0)
    m.moveHighlight(-1)
    expect(m.state.highlightedIndex).toBe(2)
    m.moveHighlight(1)
    expect(m.state.highlightedIndex).toBe(0)
  })

  it('returns the highlighted entity, or null when there is none', async () => {
    const m = useMentionMenu()
    expect(m.current()).toBeNull()
    m.setQuery('TKT')
    await settle()
    expect(m.current()?.id).toBe('TKT-ABC')
  })

  it('ignores an out-of-range highlight', async () => {
    const m = useMentionMenu()
    m.setQuery('TKT')
    await settle()
    m.setHighlight(99)
    expect(m.state.highlightedIndex).toBe(0)
  })

  it('stops updating after dispose', async () => {
    const m = useMentionMenu()
    m.setQuery('TKT')
    m.dispose()
    await settle()
    expect(m.state.items).toEqual([])
  })
})
