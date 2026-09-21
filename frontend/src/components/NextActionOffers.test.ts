import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import NextActionOffers from './NextActionOffers.vue'
import { __resetNextActionForTest, useNextAction } from '@/composables/useNextAction'
import { getNextAction, sendNextActionFeedback } from '@/api'
import { runAction } from '@/api/actions'
import type { NextActionOffer } from '@/types'

vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return {
    ...actual,
    getNextAction: vi.fn(),
    sendNextActionFeedback: vi.fn(),
  }
})
vi.mock('@/api/actions', () => ({ runAction: vi.fn() }))
// PARTIAL: useWorld() reads useRoute(), so replacing the whole module leaves
// the composable without a route and every test fails on import.
const routerPush = vi.fn()
vi.mock('vue-router', async (orig) => ({
  ...(await orig<typeof import('vue-router')>()),
  useRouter: () => ({ push: routerPush }),
}))
// The confirm dialog is a host-bound singleton (App.vue). Here it records what
// it was asked and answers YES by default, so an action test asserts the
// WORDING without a real dialog; the refusal case overrides it.
type ConfirmOpts = { title: string; message: string; confirmLabel?: string }
const confirmMock = vi.fn<(o: ConfirmOpts) => Promise<boolean>>(async () => true)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: (o: ConfirmOpts) => confirmMock(o) }),
  withConfirmError: (fn: unknown) => fn,
}))

const mockGet = vi.mocked(getNextAction)
const mockFeedback = vi.mocked(sendNextActionFeedback)
const mockRunAction = vi.mocked(runAction)

const stubs = { RouterLink: { template: '<a :to="to"><slot/></a>', props: ['to'] } }

function mountOffers(
  offers: NextActionOffer[],
  pickOptions?: Record<string, Array<{ entity_id: string; label: string }>>
) {
  return mount(NextActionOffers, {
    props: { offers, entityId: 'TASK-1', pickOptions },
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
    mockRunAction.mockReset().mockResolvedValue(null)
    confirmMock.mockReset().mockResolvedValue(true)
    routerPush.mockClear()
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

  // BUG-G2BASF: `action` was filtered out of the render entirely, so an offer
  // the config layer validates (Kind() ranks it first in the union, and an
  // unknown action id is a load error) produced no button at all.
  describe('action', () => {
    const actOffer: NextActionOffer = { action: 'regenerate', label: 'Regenerate' }

    it('renders an action as a primary affordance', () => {
      const w = mountOffers([actOffer])

      expect(w.text()).toContain('Regenerate')
      expect(w.find('.btn-primary').exists()).toBe(true)
    })

    it('runs the configured action against the suggested entity', async () => {
      const w = mountOffers([actOffer])
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      // No entity TYPE: the server reads it off the stored row and ignores a
      // caller-supplied one (BUG-ZWTDH9).
      expect(mockRunAction).toHaveBeenCalledWith('regenerate', 'TASK-1')
    })

    // An offer mutates the graph in one click with no undo, and validation
    // already refuses `confirm` on anything but an action or set — so an
    // operator who set it is owed the prompt.
    it('prompts before running when confirm is set', async () => {
      const w = mountOffers([{ ...actOffer, confirm: true }])
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      expect(confirmMock).toHaveBeenCalled()
      expect(confirmMock.mock.calls[0][0].confirmLabel).toBe('Regenerate')
      expect(mockRunAction).toHaveBeenCalled()
    })

    it('runs nothing when the confirmation is declined', async () => {
      confirmMock.mockResolvedValue(false)
      const w = mountOffers([{ ...actOffer, confirm: true }])
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      expect(mockRunAction).not.toHaveBeenCalled()
    })

    it('does not prompt when confirm is absent', async () => {
      const w = mountOffers([actOffer])
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      expect(confirmMock).not.toHaveBeenCalled()
    })

    // The suggestion was resolved BY acting, so the slot is re-asked rather
    // than left recommending work that is now done.
    it('re-resolves the suggestion after a successful run', async () => {
      const w = mountOffers([actOffer])
      mockGet.mockClear()
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      expect(mockGet).toHaveBeenCalled()
    })

    // The button's :disabled only updates on re-render, so two clicks in one
    // tick both reach the handler. With `confirm` the latch has to be taken
    // BEFORE the dialog await, or two dialogs open and two OKs mutate twice.
    it('runs once for a double-click, even with confirm', async () => {
      let resolveConfirm: (ok: boolean) => void = () => {}
      confirmMock.mockImplementation(() => new Promise<boolean>((r) => { resolveConfirm = r }))

      const w = mountOffers([{ ...actOffer, confirm: true }])
      const btn = w.find('.btn-primary')
      await btn.trigger('click')
      await btn.trigger('click')
      resolveConfirm(true)
      await flushPromises()

      expect(confirmMock).toHaveBeenCalledTimes(1)
      expect(mockRunAction).toHaveBeenCalledTimes(1)
    })

    it('runs once for a double-click without confirm', async () => {
      const w = mountOffers([actOffer])
      const btn = w.find('.btn-primary')
      await btn.trigger('click')
      await btn.trigger('click')
      await flushPromises()

      expect(mockRunAction).toHaveBeenCalledTimes(1)
    })

    // The latch must not strand the button after a refusal.
    it('can be retried after the confirmation is declined', async () => {
      confirmMock.mockResolvedValue(false)
      const w = mountOffers([{ ...actOffer, confirm: true }])
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      confirmMock.mockResolvedValue(true)
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      expect(mockRunAction).toHaveBeenCalledTimes(1)
    })

    // A script may answer with a destination instead of a message; dropping
    // it would strand the user on a suggestion that has already been acted on.
    it('follows a redirect the script returned', async () => {
      mockRunAction.mockResolvedValue({ redirect: '/entity/task/TASK-2' })
      const w = mountOffers([actOffer])
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      expect(routerPush).toHaveBeenCalledWith('/entity/task/TASK-2')
    })

    // An advisory surface must not break the page it sits on.
    it('survives a failing action', async () => {
      mockRunAction.mockRejectedValue(new Error('boom'))
      const w = mountOffers([actOffer])
      await w.find('.btn-primary').trigger('click')
      await flushPromises()

      expect(w.text()).toContain('Regenerate')
    })

    // `set` has no endpoint behind it, so a button would have nothing to
    // call. Rendering one would be the affordance-that-lies shape.
    it('renders no button at all for a set-only offer', () => {
      const w = mountOffers([{ set: { status: 'done' } }])

      // Not just "no primary button" — a secondary one would be the same
      // defect wearing a different class.
      expect(w.findAll('button').filter((b) => !b.classes('na-defer__trigger'))).toHaveLength(0)
      expect(w.findAll('a')).toHaveLength(0)
    })

    // entityId is optional on the wire. Without one runAction sends no body
    // and the action would run against nothing, server-side and silently.
    it('renders no action button without an entity id', () => {
      const w = mount(NextActionOffers, {
        props: { offers: [actOffer] },
        global: { stubs },
      })

      expect(w.find('.btn-primary').exists()).toBe(false)
    })
  })

  describe('pick_one', () => {
    const pickOffer: NextActionOffer = {
      pick_one: { query: 'type:task prop:effort=xs', action: 'start-task' },
    }

    // The options come from a query at render time — that is the whole reason
    // this affordance exists rather than being a static button list.
    it('renders one button per resolved option', () => {
      const w = mountOffers([pickOffer], {
        '0': [
          { entity_id: 'T-9', label: 'Draft the retro' },
          { entity_id: 'T-8', label: 'Rename the bucket' },
        ],
      })

      const picks = w.findAll('.rela-na-pick')
      expect(picks).toHaveLength(2)
      expect(picks[0].text()).toBe('Draft the retro')
    })

    it('links each option to its entity', () => {
      const w = mountOffers([pickOffer], { '0': [{ entity_id: 'T-9', label: 'One' }] })

      expect(w.find('.rela-na-pick').attributes('to')).toBe('/entity/T-9')
    })

    // An empty option row is worse than no affordance: it says "choose" and
    // offers nothing.
    it('renders nothing when the query matched no options', () => {
      const w = mountOffers([pickOffer], { '0': [] })

      expect(w.find('.rela-na-pick').exists()).toBe(false)
    })

    it('renders nothing when the server sent no options at all', () => {
      const w = mountOffers([pickOffer])

      expect(w.find('.rela-na-pick').exists()).toBe(false)
    })

    // Options are keyed by the offer's INDEX, so an offer preceded by others
    // must still find its own list.
    it('matches options to the right offer by index', () => {
      const w = mountOffers([{ acknowledge: true }, pickOffer], {
        '1': [{ entity_id: 'T-9', label: 'Correct one' }],
      })

      const picks = w.findAll('.rela-na-pick')
      expect(picks).toHaveLength(1)
      expect(picks[0].text()).toBe('Correct one')
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
        expect.objectContaining({ kind: 'snooze', duration: '7d' })
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
