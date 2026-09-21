import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createMentionMenu, type MentionSearchBridge } from './relaMentionMenu'

/** Lets the debounce and the awaited bridge call both land. */
const settle = (): Promise<void> => new Promise((r) => setTimeout(r, 200))

function bridgeReturning(...batches: Array<Array<{ id?: unknown }>>): MentionSearchBridge {
  let i = 0
  return {
    search: vi.fn(async () => ({ data: batches[Math.min(i++, batches.length - 1)] })),
  }
}

function rows(menu: { root: HTMLElement }): string[] {
  return [...menu.root.querySelectorAll('.rela-mention-option')].map((e) => e.textContent ?? '')
}

function note(menu: { root: HTMLElement }): string {
  return menu.root.querySelector('.rela-mention-note')?.textContent ?? ''
}

describe('createMentionMenu', () => {
  let picked: string[]
  beforeEach(() => {
    picked = []
  })

  const make = (bridge: MentionSearchBridge) =>
    createMentionMenu(bridge, { pick: (id) => picked.push(id) })

  it('prompts instead of searching on a query below the minimum length', async () => {
    const bridge = bridgeReturning([])
    const menu = make(bridge)
    menu.setQuery('T')
    await settle()
    expect(bridge.search).not.toHaveBeenCalled()
    expect(note(menu)).toContain('2 characters')
  })

  it('searches and lists the results as bare IDs', async () => {
    // No titles: the app bridge has no per-principal mentions endpoint, and
    // deriving one any other way would route around the read gate.
    const menu = make(bridgeReturning([{ id: 'TKT-ABC' }, { id: 'FEAT-XY' }]))
    menu.setQuery('TK')
    await settle()
    // `FEAT-XY` is dropped, not listed last: the shared ranking filters as well
    // as orders (TKT-6MZ42J), so a row matching neither the title nor the id no
    // longer survives at the bottom. The app editor gets that from
    // `rankMentions.ts` along with the SPA; only the SPA's type picker is local
    // to it, since a sandboxed app cannot fetch a schema type list.
    expect(rows(menu)).toEqual(['TKT-ABC'])
  })

  it('ranks an ID prefix match above a looser one', async () => {
    const menu = make(bridgeReturning([{ id: 'FEAT-TKX' }, { id: 'TKT-ABC' }]))
    menu.setQuery('TKT')
    await settle()
    expect(rows(menu)[0]).toBe('TKT-ABC')
  })

  it('drops a row whose ID could not be written as a reference', async () => {
    // Offering it would insert nothing: the editor refuses the id, so the click
    // would silently do nothing at all.
    const menu = make(bridgeReturning([{ id: 'has space' }, { id: 'TKT-OK' }, { id: 42 }]))
    menu.setQuery('TK')
    await settle()
    expect(rows(menu)).toEqual(['TKT-OK'])
  })

  it('reports a failed search rather than showing an empty list', async () => {
    const menu = make({ search: vi.fn().mockRejectedValue(new Error('nope')) })
    menu.setQuery('TKT')
    await settle()
    expect(note(menu)).toBe('Search failed')
  })

  it('says so when a search returns nothing', async () => {
    const menu = make(bridgeReturning([]))
    menu.setQuery('ZZZ')
    await settle()
    expect(note(menu)).toBe('No matches')
  })

  it('moves the highlight and reports the entity under it', async () => {
    const menu = make(bridgeReturning([{ id: 'A-1' }, { id: 'A-2' }, { id: 'A-3' }]))
    menu.setQuery('A-')
    await settle()
    expect(menu.current()).toBe('A-1')
    menu.moveHighlight(1)
    expect(menu.current()).toBe('A-2')
    // Wraps, so holding the key never strands the highlight at an end.
    menu.moveHighlight(-2)
    expect(menu.current()).toBe('A-3')
  })

  it('picks the clicked row', async () => {
    const menu = make(bridgeReturning([{ id: 'TKT-ABC' }]))
    menu.setQuery('TKT')
    await settle()
    ;(menu.root.querySelector('.rela-mention-option') as HTMLElement).click()
    expect(picked).toEqual(['TKT-ABC'])
  })

  it('discards a slow response for a query the user has moved on from', async () => {
    // The bridge offers no cancellation, so a stale result is dropped on arrival
    // rather than aborted. Without this the menu would flash results for a query
    // that is no longer on screen.
    let release: (() => void) | null = null
    const gate = new Promise<void>((r) => (release = r))
    const bridge: MentionSearchBridge = {
      search: vi
        .fn()
        .mockImplementationOnce(async () => {
          await gate
          return { data: [{ id: 'STALE-1' }] }
        })
        .mockImplementationOnce(async () => ({ data: [{ id: 'FRESH-1' }] })),
    }
    const menu = make(bridge)
    menu.setQuery('ST')
    await new Promise((r) => setTimeout(r, 160))
    menu.setQuery('FR')
    release!()
    await settle()
    expect(rows(menu)).toEqual(['FRESH-1'])
  })

  it('cannot be reopened by a response that lands after close', async () => {
    let release: (() => void) | null = null
    const gate = new Promise<void>((r) => (release = r))
    const menu = make({
      search: vi.fn(async () => {
        await gate
        return { data: [{ id: 'LATE-1' }] }
      }),
    })
    menu.setQuery('LA')
    await new Promise((r) => setTimeout(r, 160))
    menu.close()
    release!()
    await settle()
    expect(menu.isOpen).toBe(false)
    expect(rows(menu)).toEqual([])
  })

  it('clears stale results when the query is backspaced below the minimum', async () => {
    const menu = make(bridgeReturning([{ id: 'TKT-ABC' }]))
    menu.setQuery('TKT')
    await settle()
    expect(rows(menu)).toEqual(['TKT-ABC'])
    menu.setQuery('T')
    expect(rows(menu)).toEqual([])
  })

  it('stops answering once destroyed', async () => {
    const menu = make(bridgeReturning([{ id: 'TKT-ABC' }]))
    menu.destroy()
    menu.setQuery('TKT')
    await settle()
    expect(menu.isOpen).toBe(false)
  })
})
