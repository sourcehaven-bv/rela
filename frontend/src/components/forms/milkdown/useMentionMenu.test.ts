import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useMentionMenu } from './useMentionMenu'
import type { Entity } from '@/types'

const searchEntities = vi.fn()
vi.mock('@/api', () => ({
  searchEntities: (...args: unknown[]) => searchEntities(...args),
  listEntities: vi.fn(),
}))

/**
 * Builds a candidate in the shape `/_search` returns: the display name is
 * `_title` (see `entityDisplayTitle`), not `properties.title`.
 */
function ent(id: string, title?: string): Entity {
  return { id, type: 'ticket', _title: title ?? id, properties: {} } as unknown as Entity
}

const TYPES = ['ticket', 'research', 'review-checklist', 'review-response', 'decision']

/** Advances past the debounce and lets pending promises settle. */
async function settle() {
  await vi.advanceTimersByTimeAsync(200)
  await Promise.resolve()
}

/** A menu with the schema's types already supplied, as the editor does. */
function menuWithTypes() {
  const m = useMentionMenu()
  m.setAvailableTypes(TYPES)
  return m
}

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
    const m = menuWithTypes()
    expect(m.state.open).toBe(false)
    expect(m.state.items).toEqual([])
  })

  it('narrows the types on one letter, without searching entities', async () => {
    const m = menuWithTypes()
    m.setQuery('r')
    await settle()
    expect(m.state.open).toBe(true)
    expect(m.state.items).toEqual([])
    // One letter is already enough to narrow, so the type list is NOT tied to
    // MIN_SEARCH_LEN — that threshold only governs the entity search, which
    // must still not fire on a single character.
    expect(m.state.typeItems.length).toBeGreaterThan(0)
    expect(m.state.typeItems.length).toBeLessThanOrEqual(3)
    expect(m.state.typeItems).not.toContain('ticket')
    expect(searchEntities).not.toHaveBeenCalled()
  })

  it('lists every type for a bare @, before any letter is typed', async () => {
    const m = menuWithTypes()
    m.setQuery('')
    await settle()
    expect(m.state.typeItems).toEqual(TYPES)
    expect(searchEntities).not.toHaveBeenCalled()
  })

  it('searches once the query is long enough', async () => {
    const m = menuWithTypes()
    m.setQuery('TKT')
    await settle()
    expect(searchEntities).toHaveBeenCalledWith('TKT', undefined, expect.anything())
    expect(m.state.items.map((e) => e.id)).toEqual(['TKT-ABC'])
  })

  it('searches across every type until one is picked', async () => {
    const m = menuWithTypes()
    m.setQuery('TKT')
    await settle()
    // The second argument is the type filter; undefined means all types.
    expect(searchEntities.mock.calls[0][1]).toBeUndefined()
  })

  it('sends the query unmodified, so backend ID ranking survives', async () => {
    const m = menuWithTypes()
    // A hyphenated query must NOT be split before sending: word-split forms
    // destroy ID ranking on bleve (BUG-O09QUC) and match nothing on the
    // linear and postgres backends.
    m.setQuery('fancy-some-word')
    await settle()
    expect(searchEntities.mock.calls[0][0]).toBe('fancy-some-word')
  })

  it('debounces so typing does not fire a request per keystroke', async () => {
    const m = menuWithTypes()
    m.setQuery('TK')
    m.setQuery('TKT')
    m.setQuery('TKT-')
    await settle()
    expect(searchEntities).toHaveBeenCalledTimes(1)
    expect(searchEntities.mock.calls[0][0]).toBe('TKT-')
  })

  it('clears results when the query is backspaced below the minimum', async () => {
    const m = menuWithTypes()
    m.setQuery('TKT')
    await settle()
    expect(m.state.items).toHaveLength(1)
    m.setQuery('T')
    await settle()
    expect(m.state.items).toEqual([])
  })

  it('ignores a slow response for a superseded query', async () => {
    const m = menuWithTypes()
    let releaseFirst: (v: unknown) => void = () => {}
    searchEntities.mockImplementationOnce(() => new Promise((res) => (releaseFirst = res)))
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
    const m = menuWithTypes()
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
    const m = menuWithTypes()
    searchEntities.mockRejectedValueOnce(new Error('nope'))
    m.setQuery('TKT')
    await settle()
    expect(m.state.errorMsg).toBe('Search failed')
    expect(m.state.items).toEqual([])
    expect(m.state.loading).toBe(false)
  })

  it('does not report an aborted request as a failure', async () => {
    const m = menuWithTypes()
    const abortErr = Object.assign(new Error('aborted'), { name: 'AbortError' })
    searchEntities.mockRejectedValueOnce(abortErr)
    m.setQuery('TKT')
    await settle()
    expect(m.state.errorMsg).toBe('')
  })

  it('does not report a failure for a query the ranker cannot tokenize', async () => {
    const m = menuWithTypes()
    searchEntities.mockResolvedValueOnce({ data: [ent('JP-1', '日本語のタイトル')] })
    m.setQuery('日本語')
    await settle()
    // An unguarded uf.info() throws here and surfaces as "Search failed"
    // (RR-66JTAL); the server's rows must come through instead.
    expect(m.state.errorMsg).toBe('')
    expect(m.state.items.map((e) => e.id)).toEqual(['JP-1'])
  })

  it('shows no matches rather than an error for an all-separator query', async () => {
    const m = menuWithTypes()
    searchEntities.mockResolvedValueOnce({ data: [] })
    m.setQuery('---')
    await settle()
    expect(m.state.errorMsg).toBe('')
    expect(m.state.items).toEqual([])
  })

  it('returns the highlighted choice, or null when there is none', async () => {
    const m = useMentionMenu()
    expect(m.current()).toBeNull()
    m.setQuery('TKT')
    await settle()
    expect(m.current()).toEqual({
      kind: 'entity',
      entity: expect.objectContaining({ id: 'TKT-ABC' }),
    })
  })

  it('ignores an out-of-range highlight', async () => {
    const m = useMentionMenu()
    m.setQuery('TKT')
    await settle()
    m.setHighlight(99)
    expect(m.highlightedIndex()).toBe(0)
  })

  it('stops updating after dispose', async () => {
    const m = menuWithTypes()
    m.setQuery('TKT')
    m.dispose()
    await settle()
    expect(m.state.items).toEqual([])
  })

  describe('type suggestions', () => {
    it.each([
      { len: 0, query: '', expect: 'all' },
      { len: 1, query: 'r', expect: 'capped' },
      { len: 2, query: 're', expect: 'capped' },
      { len: 5, query: 'revie', expect: 'capped' },
      { len: 6, query: 'review', expect: 'capped' },
      { len: 7, query: 'review-', expect: 'none' },
    ])('shows $expect type rows for a $len-character query', async ({ query, expect: want }) => {
      const m = menuWithTypes()
      m.setQuery(query)
      await settle()
      if (want === 'all') expect(m.state.typeItems).toEqual(TYPES)
      else if (want === 'none') expect(m.state.typeItems).toEqual([])
      else {
        expect(m.state.typeItems.length).toBeGreaterThan(0)
        expect(m.state.typeItems.length).toBeLessThanOrEqual(3)
      }
    })

    it('ranks the matching types for a short query', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      expect(m.state.typeItems).not.toContain('ticket')
      for (const name of m.state.typeItems) expect(TYPES).toContain(name)
    })

    it('hides the types section once a type is picked', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      m.selectType('research')
      await settle()
      expect(m.state.typeItems).toEqual([])
    })
  })

  describe('type scoping', () => {
    it('scopes the next search to the picked type', async () => {
      const m = menuWithTypes()
      m.setQuery('rank')
      await settle()
      searchEntities.mockClear()
      m.selectType('ticket')
      await settle()
      expect(searchEntities).toHaveBeenCalledWith('rank', 'ticket', expect.anything())
      expect(m.state.selectedType).toBe('ticket')
    })

    it('refuses a type the schema never offered, so ?type= stays an allowlist', async () => {
      const m = menuWithTypes()
      m.setQuery('rank')
      await settle()
      m.selectType('../../etc/passwd')
      expect(m.state.selectedType).toBeNull()
    })

    it('unscopes the search when the type is cleared', async () => {
      const m = menuWithTypes()
      m.setQuery('rank')
      await settle()
      m.selectType('ticket')
      await settle()
      searchEntities.mockClear()
      m.clearType()
      await settle()
      expect(m.state.selectedType).toBeNull()
      expect(searchEntities.mock.calls[0][1]).toBeUndefined()
    })

    it('keeps the scope while the query is edited', async () => {
      const m = menuWithTypes()
      m.selectType('ticket')
      m.setQuery('ra')
      await settle()
      m.setQuery('rank')
      await settle()
      expect(m.state.selectedType).toBe('ticket')
      expect(searchEntities.mock.calls[searchEntities.mock.calls.length - 1][1]).toBe('ticket')
    })

    it('drops a scope that a schema reload retired', async () => {
      const m = menuWithTypes()
      m.selectType('ticket')
      expect(m.state.selectedType).toBe('ticket')
      m.setAvailableTypes(['research', 'decision'])
      expect(m.state.selectedType).toBeNull()
    })

    it('clears the scope when the menu closes', async () => {
      const m = menuWithTypes()
      m.selectType('ticket')
      m.close()
      expect(m.state.selectedType).toBeNull()
      expect(m.state.typeItems).toEqual([])
    })
  })

  // These assert BEFORE `settle()`, in the window between a keystroke and the
  // search response. The highlight bugs this guards against all lived there:
  // `runSearch` resets the highlight on arrival, so any test that settles first
  // cannot observe them. A fast typist spends all their time in this window.
  describe('the highlight survives rows changing underneath it', () => {
    beforeEach(() => {
      searchEntities.mockResolvedValue({
        data: [ent('RE-1', 'research notes'), ent('RE-2', 'review log')],
      })
    })

    it('never points past the end when the type section disappears', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      const typeCount = m.state.typeItems.length
      expect(typeCount).toBeGreaterThan(0)
      // The LAST entity, so a stale index cannot coincidentally land on another
      // valid row — that is what let an earlier version of this test pass while
      // the bug was still present.
      const lastEntity = m.state.items[m.state.items.length - 1]
      m.setHighlight(typeCount + m.state.items.length - 1)
      expect(m.current()).toEqual({ kind: 'entity', entity: lastEntity })

      // Crossing TYPE_SECTION_MAX_QUERY hides the types, shortening the list.
      m.setQuery('review-x')
      expect(m.state.typeItems).toEqual([])
      // Must still resolve to a real row, and to the SAME entity: a stale index
      // here made Enter a no-op that fell through and inserted a paragraph
      // break, or worse, silently moved to a different row.
      expect(m.highlightedIndex()).toBeLessThan(m.state.items.length)
      expect(m.current()).toEqual({ kind: 'entity', entity: lastEntity })
    })

    it('keeps the same type highlighted when the suggestions re-rank', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      const target = m.state.typeItems[m.state.typeItems.length - 1]
      m.setHighlight(m.state.typeItems.indexOf(target))
      expect(m.current()).toEqual({ kind: 'type', name: target })

      // A keystroke that re-ranks (and may shorten) the type list must not
      // slide the highlight onto a different row — that inserted an entity
      // when the user meant to pick a type.
      m.setQuery('revi')
      const after = m.current()
      if (m.state.typeItems.includes(target)) {
        expect(after).toEqual({ kind: 'type', name: target })
      } else {
        // Gone from the list: fall back to the first row, never to whatever
        // moved into the old position.
        expect(m.highlightedIndex()).toBe(0)
      }
    })

    it('does not leave the highlight on a stale entity after a scope is picked', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      m.setHighlight(m.state.typeItems.length + 1) // the second entity row

      m.selectType('ticket')
      // The old rows answered an unscoped question; they must not still be
      // highlighted (nor still be on screen) under the new chip.
      expect(m.state.items).toEqual([])
      expect(m.current()).toBeNull()
    })

    it('drops results from the previous scope so the chip cannot lie', async () => {
      const m = menuWithTypes()
      searchEntities.mockResolvedValue({ data: [ent('RK-1', 'ranking fix')] })
      m.setQuery('rank')
      await settle()
      expect(m.state.items.length).toBeGreaterThan(0)

      m.selectType('ticket')
      // Immediately, not after the debounce: the chip already says `ticket`.
      expect(m.state.selectedType).toBe('ticket')
      expect(m.state.items).toEqual([])
    })

    it('drops results again when the scope is cleared', async () => {
      const m = menuWithTypes()
      searchEntities.mockResolvedValue({ data: [ent('RK-1', 'ranking fix')] })
      m.setQuery('rank')
      m.selectType('ticket')
      await settle()
      expect(m.state.items.length).toBeGreaterThan(0)
      m.clearType()
      expect(m.state.items).toEqual([])
    })
  })

  describe('closed-menu and disposed guards', () => {
    it('does not repopulate the type list behind a closed menu', () => {
      const m = menuWithTypes()
      m.setQuery('re')
      m.close()
      expect(m.state.typeItems).toEqual([])
      // A schema reload (SSE, settings change) must not refill a closed menu:
      // that would make `close()` a liar and let the panel show itself.
      m.setAvailableTypes([...TYPES, 'extra-type'])
      expect(m.state.typeItems).toEqual([])
      expect(m.state.open).toBe(false)
    })

    it('ignores setAvailableTypes after dispose', () => {
      const m = menuWithTypes()
      m.setQuery('re')
      m.dispose()
      m.setAvailableTypes(['only-this'])
      expect(m.state.typeItems).not.toContain('only-this')
    })

    it('is idempotent for an unchanged type list', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      const before = [...m.state.typeItems]
      m.setAvailableTypes([...TYPES])
      expect([...m.state.typeItems]).toEqual(before)
    })

    it('does not fire a search for a no-op scope change', async () => {
      const m = menuWithTypes()
      searchEntities.mockResolvedValue({ data: [ent('RK-1', 'ranking fix')] })
      m.setQuery('rank')
      await settle()
      searchEntities.mockClear()
      m.clearType() // already unscoped
      await settle()
      expect(searchEntities).not.toHaveBeenCalled()
      m.selectType('ticket')
      await settle()
      searchEntities.mockClear()
      m.selectType('ticket') // already this scope
      await settle()
      expect(searchEntities).not.toHaveBeenCalled()
    })
  })

  describe('combined highlight', () => {
    beforeEach(() => {
      // These must MATCH the 're' query used below. The ranker drops
      // non-matching rows, so unrelated ids would leave the entity section
      // empty and the traversal assertions would pass vacuously.
      searchEntities.mockResolvedValue({
        data: [ent('RE-1', 'research notes'), ent('RE-2', 'review log')],
      })
    })

    it('addresses types first, then entities', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      const typeCount = m.state.typeItems.length
      expect(typeCount).toBeGreaterThan(0)

      expect(m.current()).toEqual({ kind: 'type', name: m.state.typeItems[0] })
      m.setHighlight(typeCount)
      expect(m.current()).toEqual({
        kind: 'entity',
        entity: expect.objectContaining({ id: 'RE-1' }),
      })
    })

    it('wraps across both sections in one sequence', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      const total = m.state.typeItems.length + m.state.items.length

      expect(m.highlightedIndex()).toBe(0)
      m.moveHighlight(-1)
      expect(m.highlightedIndex()).toBe(total - 1)
      expect(m.current()?.kind).toBe('entity')
      m.moveHighlight(1)
      expect(m.highlightedIndex()).toBe(0)
      expect(m.current()?.kind).toBe('type')
    })

    it('steps from the last type row onto the first entity row', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      const typeCount = m.state.typeItems.length
      m.setHighlight(typeCount - 1)
      expect(m.current()?.kind).toBe('type')
      m.moveHighlight(1)
      expect(m.current()?.kind).toBe('entity')
    })

    it('rejects an index past the combined length', async () => {
      const m = menuWithTypes()
      m.setQuery('re')
      await settle()
      const total = m.state.typeItems.length + m.state.items.length
      m.setHighlight(total)
      expect(m.highlightedIndex()).toBe(0)
    })

    it('has nothing to pick when both sections are empty', async () => {
      searchEntities.mockResolvedValue({ data: [] })
      const m = useMentionMenu()
      m.setQuery('zzzz')
      await settle()
      expect(m.current()).toBeNull()
      m.moveHighlight(1)
      // -1, not 0: there is no row to be on, and claiming row 0 exists is how
      // an out-of-range highlight used to look valid.
      expect(m.highlightedIndex()).toBe(-1)
    })
  })
})
