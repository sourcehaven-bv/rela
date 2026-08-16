import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useNextAction, __resetNextActionForTest } from './useNextAction'
import { useSchemaStore } from '@/stores/schema'
import { getNextAction, sendNextActionFeedback } from '@/api'
import type { NextActionSuggestion } from '@/types'

vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return {
    ...actual,
    getNextAction: vi.fn(),
    sendNextActionFeedback: vi.fn(),
  }
})

const mockGet = vi.mocked(getNextAction)
const mockFeedback = vi.mocked(sendNextActionFeedback)

function suggestion(over: Partial<NextActionSuggestion> = {}): NextActionSuggestion {
  return {
    source: 'stale',
    band: 'stalled',
    entity_id: 'TASK-1',
    message: 'Still on it?',
    ...over,
  }
}

describe('useNextAction', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    __resetNextActionForTest()
    mockFeedback.mockResolvedValue(undefined)
  })

  describe('loading', () => {
    it('exposes the resolved suggestion', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const na = useNextAction()

      await na.loadOnce()

      expect(na.suggestion.value?.source).toBe('stale')
    })

    it('holds null when nothing is owed', async () => {
      mockGet.mockResolvedValue({ suggestion: null })
      const na = useNextAction()

      await na.loadOnce()

      expect(na.suggestion.value).toBeNull()
    })

    // Two components render the same suggestion (page card + status bar). A
    // second resolve would double-count the impression and could surface two
    // different suggestions at once, breaking the one-slot promise.
    it('resolves once no matter how many consumers mount', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })

      await useNextAction().loadOnce()
      await useNextAction().loadOnce()
      await useNextAction().loadOnce()

      expect(mockGet).toHaveBeenCalledTimes(1)
    })

    // An advisory surface must never break the page it sits on.
    it('stays silent when the request fails', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      mockGet.mockRejectedValue(new Error('boom'))
      const na = useNextAction()

      await na.loadOnce()

      expect(na.suggestion.value).toBeNull()
    })
  })

  describe('impressions', () => {
    // Resolving is NOT displaying. load() also runs after feedback, so
    // reporting from it would start a 24h cooldown on the replacement
    // suggestion before anyone had seen it.
    it('does not report on load', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })

      await useNextAction().loadOnce()

      expect(mockFeedback).not.toHaveBeenCalled()
    })

    it('reports the full key when the renderer marks it shown', async () => {
      mockGet.mockResolvedValue({
        suggestion: suggestion({ variant: 'status=open' }),
      })
      const na = useNextAction()
      await na.loadOnce()

      await na.markShown()

      expect(mockFeedback).toHaveBeenCalledWith({
        source: 'stale',
        entity_id: 'TASK-1',
        variant: 'status=open',
        kind: 'shown',
      })
    })

    // Both surfaces share this state and either may re-render; the same
    // suggestion must not be counted twice.
    it('reports at most once per suggestion', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()

      await na.markShown()
      await na.markShown()
      await na.markShown()

      expect(mockFeedback).toHaveBeenCalledTimes(1)
    })

    it('reports nothing when there is no suggestion', async () => {
      mockGet.mockResolvedValue({ suggestion: null })
      const na = useNextAction()
      await na.loadOnce()

      await na.markShown()

      expect(mockFeedback).not.toHaveBeenCalled()
    })

    // The replacement is a different suggestion, so it gets its own
    // impression once something renders it.
    it('reports again after feedback swaps the suggestion', async () => {
      mockGet.mockResolvedValueOnce({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()
      await na.markShown()

      mockGet.mockResolvedValueOnce({ suggestion: suggestion({ source: 'quip' }) })
      await na.respond('dismiss')
      mockFeedback.mockClear()

      await na.markShown()

      expect(mockFeedback).toHaveBeenCalledWith(
        expect.objectContaining({ source: 'quip', kind: 'shown' })
      )
    })
  })

  describe('feedback', () => {
    // The variant is part of the suggestion key: a snooze that omits it lands
    // under a key the server never checks and silently fails to suppress.
    it('echoes the variant back on snooze', async () => {
      mockGet.mockResolvedValue({
        suggestion: suggestion({ variant: 'status=open' }),
      })
      const na = useNextAction()
      await na.loadOnce()
      mockFeedback.mockClear()

      await na.respond('snooze', '7d')

      expect(mockFeedback).toHaveBeenCalledWith({
        source: 'stale',
        entity_id: 'TASK-1',
        variant: 'status=open',
        kind: 'snooze',
        duration: '7d',
      })
    })

    it('re-resolves after feedback so the next suggestion appears', async () => {
      mockGet.mockResolvedValueOnce({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()

      mockGet.mockResolvedValueOnce({ suggestion: suggestion({ source: 'quip', band: 'ambient' }) })
      await na.respond('dismiss')

      expect(na.suggestion.value?.source).toBe('quip')
    })

    it('does nothing when there is no suggestion to answer', async () => {
      mockGet.mockResolvedValue({ suggestion: null })
      const na = useNextAction()
      await na.loadOnce()
      mockFeedback.mockClear()

      await na.respond('mute')

      expect(mockFeedback).not.toHaveBeenCalled()
    })

    it('survives a failed feedback call', async () => {
      vi.spyOn(console, 'error').mockImplementation(() => {})
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()
      mockFeedback.mockRejectedValueOnce(new Error('boom'))

      await na.respond('snooze', '1d')

      expect(na.busy.value).toBe(false)
    })

    // Acknowledge is "seen it" — the impression is already recorded, so it
    // clears the slot without another round trip.
    it('acknowledge clears the slot without further feedback', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()
      mockFeedback.mockClear()

      na.acknowledge()

      expect(na.suggestion.value).toBeNull()
      expect(mockFeedback).not.toHaveBeenCalled()
    })
  })

  describe('prominence', () => {
    function withBands(bands: Array<{ id: string; label?: string; prominence?: string }>) {
      const schemaStore = useSchemaStore()
      schemaStore.nextActionBands = bands as never
    }

    // Matches the server-side default: an operator who has not thought about
    // prominence has not earned the top of the page.
    it('defaults to statusbar for an undeclared band', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()

      expect(na.prominence.value).toBe('statusbar')
      expect(na.isStatusBar.value).toBe(true)
      expect(na.isPageLevel.value).toBe(false)
    })

    it.each([
      ['banner', true, false],
      ['notice', true, false],
      ['statusbar', false, true],
    ])('routes %s to the right surface', async (prom, pageLevel, statusBar) => {
      withBands([{ id: 'stalled', prominence: prom }])
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()

      expect(na.isPageLevel.value).toBe(pageLevel)
      expect(na.isStatusBar.value).toBe(statusBar)
    })

    it('prefers the operator label over the raw band id', async () => {
      withBands([{ id: 'stalled', label: 'Waiting on you' }])
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()

      expect(na.bandLabel.value).toBe('Waiting on you')
    })

    it('falls back to the band id when no label is declared', async () => {
      withBands([{ id: 'stalled' }])
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const na = useNextAction()
      await na.loadOnce()

      expect(na.bandLabel.value).toBe('stalled')
    })

    // Neither surface may claim a suggestion that does not exist, or an empty
    // card/chip would render on every quiet page.
    it('claims no surface when nothing is owed', async () => {
      mockGet.mockResolvedValue({ suggestion: null })
      const na = useNextAction()
      await na.loadOnce()

      expect(na.isPageLevel.value).toBe(false)
      expect(na.isStatusBar.value).toBe(false)
    })
  })
})
