import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMentionMenuMachine } from './mentionMenuState'
import { SEARCH_DEBOUNCE_MS } from '@/utils/searchTiming'

/**
 * How many requests a typist sends with the shared debounce (TKT-T2I7P2).
 *
 * On atlas a quarter of all searches were aborted by the next keystroke. The
 * local reproduction, with a typist pressing a key about every 200 ms, sent
 * 46 requests and aborted 41 at a 150 ms debounce, and 5 with none aborted at
 * 300 ms. These rows pin the same arithmetic without timing noise.
 */
const TEXT = 'gateway policy'

async function typeWith(gaps: number[]): Promise<string[]> {
  const sent: string[] = []
  const m = createMentionMenuMachine<{ id: string }>({
    search: async (q) => {
      sent.push(q)
      return []
    },
    abortable: true,
  })
  for (let i = 1; i <= TEXT.length; i++) {
    m.setQuery(TEXT.slice(0, i))
    await vi.advanceTimersByTimeAsync(gaps[(i - 1) % gaps.length])
  }
  await vi.advanceTimersByTimeAsync(SEARCH_DEBOUNCE_MS)
  m.dispose()
  return sent
}

describe('search debounce while typing', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('sends one request for a typist with 200 ms between keys', async () => {
    expect(await typeWith([200])).toEqual([TEXT])
  })

  it('sends one request per pause longer than the debounce', async () => {
    // A slower typist, 280 ms between keys on average, alternating short and
    // long gaps. Every long gap is a pause the debounce lets through, so
    // some requests are still superseded; the debounce cuts their number,
    // it does not remove them.
    const sent = await typeWith([140, 420])
    const pauses = [...TEXT]
      .map((_, i) => i + 1)
      .filter((n) => n >= 2 && n % 2 === 0 && n < TEXT.length)
    expect(sent).toEqual([...pauses.map((n) => TEXT.slice(0, n)), TEXT])
  })
})
