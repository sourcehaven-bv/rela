import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import NextActionCard from './NextActionCard.vue'
import { __resetNextActionForTest } from '@/composables/useNextAction'
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

const stubs = { RouterLink: { template: '<a><slot/></a>', props: ['to'] } }

function suggestion(over: Partial<NextActionSuggestion> = {}): NextActionSuggestion {
  return {
    source: 'stale',
    band: 'stalled',
    entity_id: 'TASK-1',
    message: 'Still on it?',
    ...over,
  }
}

async function mountCard(prominence?: string) {
  if (prominence) {
    const schemaStore = useSchemaStore()
    schemaStore.nextActionBands = [
      { id: 'stalled', label: 'Stalled', prominence },
    ] as never
  }
  const w = mount(NextActionCard, { global: { stubs } })
  await vi.waitFor(() => expect(mockGet).toHaveBeenCalled())
  await w.vm.$nextTick()
  await w.vm.$nextTick()
  return w
}

describe('NextActionCard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    __resetNextActionForTest()
    mockFeedback.mockResolvedValue(undefined)
  })

  describe('when nothing is owed', () => {
    // Silence is the normal condition of a well-configured system; an empty
    // state would turn that quiet into noise on every page load.
    it('renders nothing at all', async () => {
      mockGet.mockResolvedValue({ suggestion: null })
      const w = await mountCard()

      expect(w.find('.rela-na').exists()).toBe(false)
      expect(w.text()).toBe('')
    })
  })

  describe('surface routing', () => {
    it.each(['banner', 'notice'])('renders the %s prominence on the page', async (prom) => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const w = await mountCard(prom)

      expect(w.find(`.na--${prom}`).exists()).toBe(true)
      expect(w.text()).toContain('Still on it?')
    })

    // The status bar owns that tier; rendering here too would show the same
    // suggestion twice.
    it('renders nothing for the statusbar prominence', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const w = await mountCard('statusbar')

      expect(w.find('.rela-na').exists()).toBe(false)
    })

    // The band label answers "why am I seeing this?" — the insistent tier
    // states it, the quiet one does not shout.
    it('shows the band label on banner but not notice', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })

      const banner = await mountCard('banner')
      expect(banner.find('.rela-na-band').text()).toBe('Stalled')

      __resetNextActionForTest()
      vi.clearAllMocks()
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const notice = await mountCard('notice')
      expect(notice.find('.rela-na-band').exists()).toBe(false)
    })
  })

  // docs/customisation.md tier 1 + tier 2. These are the operator's documented
  // hooks for skinning the companion (e.g. a character image per band), so a
  // rename here is a contract break, not a refactor.
  describe('operator customisation hooks', () => {
    it('emits the tier-1 companion slot carrying the band', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const w = await mountCard('banner')

      const slot = w.find('rela-slot')
      expect(slot.exists()).toBe(true)
      expect(slot.attributes('name')).toBe('companion')
      expect(slot.attributes('data-band')).toBe('stalled')
      expect(slot.attributes('data-prominence')).toBe('banner')
    })

    it('exposes the tier-2 classes and data attributes', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const w = await mountCard('banner')

      const root = w.find('.rela-na')
      expect(root.exists()).toBe(true)
      expect(root.attributes('data-band')).toBe('stalled')
      expect(root.attributes('data-prominence')).toBe('banner')
      expect(root.attributes('data-source')).toBe('stale')
      expect(root.attributes('data-entity-id')).toBe('TASK-1')
      expect(w.find('.rela-na-message').text()).toBe('Still on it?')
    })

    // The slot is inert without a custom.js definition, so a stock deployment
    // renders as if it were absent.
    it('leaves the slot empty when nothing defines it', async () => {
      mockGet.mockResolvedValue({ suggestion: suggestion() })
      const w = await mountCard('banner')

      expect(w.find('rela-slot').text()).toBe('')
    })
  })
})
