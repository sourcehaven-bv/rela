import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import NextActionOffers from './NextActionOffers.vue'
import { __resetNextActionForTest, useNextAction } from '@/composables/useNextAction'
import { getNextAction, sendNextActionFeedback } from '@/api'
import type { NextActionOffer } from '@/types'

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

const stubs = { RouterLink: { template: '<a :to="to"><slot/></a>', props: ['to'] } }

function mountOffers(offers: NextActionOffer[]) {
  return mount(NextActionOffers, {
    props: { offers, entityId: 'TASK-1' },
    global: { stubs },
  })
}

/**
 * Load a suggestion into the shared composable. `respond` deliberately no-ops
 * when there is nothing to answer, so a feedback assertion needs one in hand.
 */
async function withLoadedSuggestion() {
  mockGet.mockResolvedValue({
    suggestion: {
      source: 'stale',
      band: 'stalled',
      entity_id: 'TASK-1',
      message: 'Still on it?',
    },
  })
  await useNextAction().loadOnce()
  mockFeedback.mockClear()
}

describe('NextActionOffers', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    __resetNextActionForTest()
    mockFeedback.mockResolvedValue(undefined)
    mockGet.mockResolvedValue({ suggestion: null })
  })

  describe('acting affordances', () => {
    it('renders navigate as a primary action', () => {
      const w = mountOffers([{ navigate: '/entity/task/{id}', label: 'Open it' }])

      expect(w.text()).toContain('Open it')
      expect(w.find('.btn-primary').exists()).toBe(true)
    })

    it('interpolates the entity id into the navigate target', () => {
      const w = mountOffers([{ navigate: '/entity/task/{id}' }])

      expect(w.find('a').attributes('to')).toBe('/entity/task/TASK-1')
    })

    it('renders acknowledge', () => {
      const w = mountOffers([{ acknowledge: true, label: 'Nice' }])

      expect(w.text()).toContain('Nice')
    })
  })

  describe('the defer menu', () => {
    // Flat, snooze/dismiss/mute read as peers competing with the action the
    // operator wants; collapsed, declining is one control.
    it('is closed until clicked', () => {
      const w = mountOffers([{ snooze: ['1d'] }])

      expect(w.find('.na-defer__menu').exists()).toBe(false)
      expect(w.find('.na-defer__trigger').text()).toContain('Not now')
    })

    it('opens on click', async () => {
      const w = mountOffers([{ snooze: ['1d', '7d'] }])

      await w.find('.na-defer__trigger').trigger('click')

      expect(w.find('.na-defer__menu').exists()).toBe(true)
      expect(w.text()).toContain('Remind me in 1d')
      expect(w.text()).toContain('Remind me in 7d')
    })

    it('sends the snooze duration the user chose', async () => {
      await withLoadedSuggestion()
      const w = mountOffers([{ snooze: ['1d', '7d'] }])
      await w.find('.na-defer__trigger').trigger('click')

      const items = w.findAll('.na-defer__item')
      await items[1].trigger('click')

      expect(mockFeedback).toHaveBeenCalledWith(
        expect.objectContaining({ kind: 'snooze', duration: '7d' }),
      )
    })

    it('closes after a choice', async () => {
      const w = mountOffers([{ snooze: ['1d'] }])
      await w.find('.na-defer__trigger').trigger('click')

      await w.findAll('.na-defer__item')[0].trigger('click')

      expect(w.find('.na-defer__menu').exists()).toBe(false)
    })

    it('offers dismiss only when the source configured it', async () => {
      const w = mountOffers([{ snooze: ['1d'] }])
      await w.find('.na-defer__trigger').trigger('click')

      expect(w.text()).not.toContain('Not this one')
    })

    it('offers dismiss when configured', async () => {
      const w = mountOffers([{ snooze: ['1d'] }, { dismiss: true }])
      await w.find('.na-defer__trigger').trigger('click')

      expect(w.text()).toContain('Not this one')
    })

    // Mute is never operator-configured: without a one-click way to switch a
    // source off, an annoying suggestion can only be escaped by complying.
    it('always offers mute, even when the source configured nothing', async () => {
      const w = mountOffers([])
      await w.find('.na-defer__trigger').trigger('click')

      expect(w.text()).toContain('Stop suggesting this')
    })

    // Mute governs the SOURCE, not this suggestion, so it is set apart rather
    // than listed as a peer of the snooze options.
    it('separates mute from the per-suggestion options', async () => {
      const w = mountOffers([{ snooze: ['1d'] }])
      await w.find('.na-defer__trigger').trigger('click')

      expect(w.find('.na-defer__item--sep').text()).toBe('Stop suggesting this')
    })
  })

  describe('accessibility', () => {
    it('marks the trigger as a menu control', () => {
      const w = mountOffers([{ snooze: ['1d'] }])
      const trigger = w.find('.na-defer__trigger')

      expect(trigger.attributes('aria-haspopup')).toBe('menu')
      expect(trigger.attributes('aria-expanded')).toBe('false')
    })

    it('reflects the open state', async () => {
      const w = mountOffers([{ snooze: ['1d'] }])
      await w.find('.na-defer__trigger').trigger('click')

      expect(w.find('.na-defer__trigger').attributes('aria-expanded')).toBe('true')
      expect(w.find('.na-defer__menu').attributes('role')).toBe('menu')
    })
  })
})
