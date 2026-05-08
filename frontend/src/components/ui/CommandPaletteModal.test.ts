import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import CommandPaletteModal from './CommandPaletteModal.vue'
import { useSchemaStore } from '@/stores/schema'
import { _resetModalStack, isAnyModalOpen } from '@/composables/modalStack'
import { searchEntities } from '@/api'
import type { Entity, ListResponse } from '@/types'

// Router stub — palette navigates via router.push when an entity is selected.
const routerPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
}))

// Mock the search endpoint so each test can stub responses and we can assert
// signal forwarding without hitting the real API client.
vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return {
    ...actual,
    searchEntities: vi.fn(),
  }
})

const searchSpy = searchEntities as unknown as ReturnType<typeof vi.fn>

function makeEntity(overrides: Partial<Entity> = {}): Entity {
  return {
    id: overrides.id ?? `T-${Math.random().toString(36).slice(2, 8).toUpperCase()}`,
    type: overrides.type ?? 'ticket',
    _title: overrides._title,
    properties: overrides.properties ?? {},
    ...overrides,
  } as Entity
}

function listResponse(entities: Entity[]): ListResponse<Entity> {
  return {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
  }
}

function seedSchema() {
  const schemaStore = useSchemaStore()
  schemaStore.entityTypes.set('ticket', {
    name: 'ticket',
    label: 'Ticket',
    properties: {},
  } as never)
}

function factory(props: { open?: boolean } = {}): VueWrapper {
  return mount(CommandPaletteModal, {
    props: { open: true, ...props },
    attachTo: document.body,
  })
}

function input(): HTMLInputElement {
  const el = document.querySelector<HTMLInputElement>('.cmdk-input')
  if (!el) throw new Error('cmdk-input not in DOM')
  return el
}

function options(): HTMLLIElement[] {
  return Array.from(document.querySelectorAll<HTMLLIElement>('.cmdk-option'))
}

// Type into the palette and wait for the debounced search to settle.
// Vue's v-model needs a microtask to sync the input event into the bound ref;
// flushPromises before advancing the fake timer ensures the watcher schedules
// the debounced setTimeout, which we then advance past.
async function typeQuery(value: string): Promise<void> {
  input().value = value
  input().dispatchEvent(new Event('input'))
  await flushPromises()
  vi.advanceTimersByTime(150)
  await flushPromises()
  await flushPromises()
}

describe('CommandPaletteModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    _resetModalStack()
    routerPush.mockClear()
    searchSpy.mockClear()
    // Default implementation so tests that don't queue a value don't crash on
    // resp.data; individual tests can still override with mockResolvedValueOnce
    // / mockRejectedValueOnce / mockImplementationOnce.
    searchSpy.mockResolvedValue(listResponse([]))
    document.body.innerHTML = ''
    seedSchema()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    document.body.innerHTML = ''
    _resetModalStack()
  })

  describe('rendering', () => {
    it('does not render when closed', () => {
      factory({ open: false })
      expect(document.querySelector('.cmdk-overlay')).toBeNull()
    })

    it('renders overlay and input when open', () => {
      factory()
      expect(document.querySelector('.cmdk-overlay')).not.toBeNull()
      expect(input()).not.toBeNull()
    })

    it('shows the empty hint when query is blank', () => {
      factory()
      const hint = document.querySelector('.cmdk-hint')?.textContent?.trim()
      expect(hint).toBe('Type to search entities')
    })

    it('renders title, type label and id for each result', async () => {
      searchSpy.mockResolvedValueOnce(
        listResponse([makeEntity({ id: 'T-1', type: 'ticket', _title: 'Fix login' })])
      )
      factory()
      await typeQuery('fix')

      const opts = options()
      expect(opts).toHaveLength(1)
      expect(opts[0].textContent).toContain('Fix login')
      expect(opts[0].textContent).toContain('T-1')
      expect(opts[0].textContent).toContain('Ticket')
    })

    it('falls back to properties.title when _title missing', async () => {
      searchSpy.mockResolvedValueOnce(
        listResponse([
          makeEntity({ id: 'T-2', properties: { title: 'Legacy title' } }),
        ])
      )
      factory()
      await typeQuery('leg')

      expect(options()[0].textContent).toContain('Legacy title')
    })

    it('falls back to id when both title fields missing', async () => {
      searchSpy.mockResolvedValueOnce(listResponse([makeEntity({ id: 'T-3' })]))
      factory()
      await typeQuery('t-3')

      const titleEl = document.querySelector('.cmdk-title')
      expect(titleEl?.textContent).toBe('T-3')
    })
  })

  describe('focus and lifecycle', () => {
    it('focuses the input on open', async () => {
      const wrapper = mount(CommandPaletteModal, {
        props: { open: false },
        attachTo: document.body,
      })
      await wrapper.setProps({ open: true })
      await flushPromises()

      expect(document.activeElement).toBe(input())
      wrapper.unmount()
    })

    it('restores previously focused element on close', async () => {
      const trigger = document.createElement('button')
      document.body.appendChild(trigger)
      trigger.focus()
      expect(document.activeElement).toBe(trigger)

      const wrapper = mount(CommandPaletteModal, {
        props: { open: false },
        attachTo: document.body,
      })
      await wrapper.setProps({ open: true })
      await flushPromises()
      expect(document.activeElement).toBe(input())

      await wrapper.setProps({ open: false })
      await flushPromises()
      expect(document.activeElement).toBe(trigger)

      wrapper.unmount()
    })

    it('registers with the modal stack while open', async () => {
      const wrapper = mount(CommandPaletteModal, {
        props: { open: false },
        attachTo: document.body,
      })
      expect(isAnyModalOpen()).toBe(false)

      await wrapper.setProps({ open: true })
      await flushPromises()
      expect(isAnyModalOpen()).toBe(true)

      await wrapper.setProps({ open: false })
      await flushPromises()
      expect(isAnyModalOpen()).toBe(false)

      wrapper.unmount()
    })

    it('resets query and highlightedIndex when re-opened', async () => {
      searchSpy.mockResolvedValue(
        listResponse([makeEntity({ id: 'A' }), makeEntity({ id: 'B' })])
      )
      const wrapper = mount(CommandPaletteModal, {
        props: { open: true },
        attachTo: document.body,
      })
      await flushPromises()

      await typeQuery('foo')
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      await flushPromises()

      await wrapper.setProps({ open: false })
      await flushPromises()
      await wrapper.setProps({ open: true })
      await flushPromises()

      expect(input().value).toBe('')
      // No results yet (query is empty), so no active descendant.
      expect(input().getAttribute('aria-activedescendant')).toBeNull()

      wrapper.unmount()
    })
  })

  describe('search behavior', () => {
    it('debounces search calls', async () => {
      searchSpy.mockResolvedValue(listResponse([]))
      factory()

      // Three rapid keystrokes; only the final one should fire a request.
      input().value = 'ab'
      input().dispatchEvent(new Event('input'))
      input().value = 'abc'
      input().dispatchEvent(new Event('input'))
      input().value = 'abcd'
      input().dispatchEvent(new Event('input'))
      await flushPromises()

      expect(searchSpy).not.toHaveBeenCalled()

      vi.advanceTimersByTime(150)
      await flushPromises()
      await flushPromises()

      expect(searchSpy).toHaveBeenCalledTimes(1)
      expect(searchSpy).toHaveBeenLastCalledWith('abcd', undefined, expect.any(AbortSignal))
    })

    it('does not call /_search for empty query', async () => {
      factory()
      await typeQuery('xy')
      searchSpy.mockClear()
      await typeQuery('')

      expect(searchSpy).not.toHaveBeenCalled()
    })

    it('does not call /_search for whitespace-only query', async () => {
      factory()
      await typeQuery('   ')

      expect(searchSpy).not.toHaveBeenCalled()
    })

    it('does not call /_search for single-character queries', async () => {
      factory()
      await typeQuery('a')

      expect(searchSpy).not.toHaveBeenCalled()
      expect(document.querySelector('.cmdk-hint')?.textContent?.trim()).toBe(
        'Type to search entities'
      )
    })

    it('caps rendered results at MAX_RESULTS (50)', async () => {
      const many = Array.from({ length: 200 }, (_, i) =>
        makeEntity({ id: `BIG-${i}`, _title: `Entity ${i}` })
      )
      searchSpy.mockResolvedValueOnce(listResponse(many))
      factory()
      await typeQuery('big')

      expect(options()).toHaveLength(50)
    })

    it('shows "No matches" when results are empty', async () => {
      searchSpy.mockResolvedValueOnce(listResponse([]))
      factory()
      await typeQuery('nothing')

      const hint = document.querySelector('.cmdk-hint')?.textContent?.trim()
      expect(hint).toBe('No matches')
    })

    it('shows error message on search failure', async () => {
      searchSpy.mockRejectedValueOnce(new Error('network down'))
      factory()
      await typeQuery('foo')

      const hint = document.querySelector('.cmdk-hint')?.textContent?.trim()
      expect(hint).toBe('Search failed')
    })

    it('keeps previous results visible while a refetch is in flight', async () => {
      searchSpy.mockResolvedValueOnce(
        listResponse([makeEntity({ id: 'first', _title: 'First' })])
      )
      factory()
      await typeQuery('fi')
      expect(options()).toHaveLength(1)

      // Trigger a second search but don't resolve it.
      let resolveSecond: (value: ListResponse<Entity>) => void = () => {}
      searchSpy.mockImplementationOnce(
        () =>
          new Promise<ListResponse<Entity>>((resolve) => {
            resolveSecond = resolve
          })
      )
      input().value = 'fix'
      input().dispatchEvent(new Event('input'))
      await flushPromises()
      vi.advanceTimersByTime(150)
      await flushPromises()

      // Previous results still visible (no flicker).
      expect(options()).toHaveLength(1)
      expect(document.querySelector('.cmdk-spinner')).not.toBeNull()

      // Resolve the second request — results swap.
      resolveSecond(
        listResponse([
          makeEntity({ id: 'a' }),
          makeEntity({ id: 'b' }),
        ])
      )
      await flushPromises()
      expect(options()).toHaveLength(2)
    })

    it('aborts the previous request when a new one is issued', async () => {
      const seenSignals: AbortSignal[] = []
      searchSpy.mockImplementation(
        async (
          _q: string,
          _t?: string,
          signal?: AbortSignal
        ): Promise<ListResponse<Entity>> => {
          if (signal) seenSignals.push(signal)
          return listResponse([])
        }
      )
      factory()

      await typeQuery('aa')
      await typeQuery('aab')

      expect(seenSignals).toHaveLength(2)
      // First signal was aborted before the second request was issued.
      expect(seenSignals[0].aborted).toBe(true)
      expect(seenSignals[1].aborted).toBe(false)
    })

    it('cancels in-flight request and timer on unmount', async () => {
      let resolveFn: (value: ListResponse<Entity>) => void = () => {}
      const seenSignals: AbortSignal[] = []
      searchSpy.mockImplementationOnce(
        (_q: string, _t?: string, signal?: AbortSignal) =>
          new Promise<ListResponse<Entity>>((resolve) => {
            if (signal) seenSignals.push(signal)
            resolveFn = resolve
          })
      )
      const wrapper = factory()
      input().value = 'foo'
      input().dispatchEvent(new Event('input'))
      await flushPromises()
      vi.advanceTimersByTime(150)
      await flushPromises()

      expect(seenSignals[0].aborted).toBe(false)
      wrapper.unmount()
      expect(seenSignals[0].aborted).toBe(true)

      // Even if the request resolves after unmount, no error is thrown.
      resolveFn(listResponse([makeEntity()]))
      await flushPromises()
    })
  })

  describe('keyboard navigation', () => {
    async function setupWithResults(n: number) {
      const entities = Array.from({ length: n }, (_, i) =>
        makeEntity({ id: `E-${i}`, _title: `Entity ${i}` })
      )
      searchSpy.mockResolvedValueOnce(listResponse(entities))
      const wrapper = factory()
      await typeQuery('ee')
      return { wrapper, entities }
    }

    it('ArrowDown moves highlight forward', async () => {
      const { wrapper } = await setupWithResults(3)
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      await flushPromises()
      expect(options()[1].classList.contains('cmdk-option-active')).toBe(true)
      wrapper.unmount()
    })

    it('ArrowDown wraps from last to first', async () => {
      const { wrapper } = await setupWithResults(3)
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      await flushPromises()
      expect(options()[0].classList.contains('cmdk-option-active')).toBe(true)
      wrapper.unmount()
    })

    it('ArrowUp wraps from first to last', async () => {
      const { wrapper } = await setupWithResults(3)
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }))
      await flushPromises()
      expect(options()[2].classList.contains('cmdk-option-active')).toBe(true)
      wrapper.unmount()
    })

    it('ArrowDown does not crash with empty results', async () => {
      factory()
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      await flushPromises()
      // No throw, no options rendered.
      expect(options()).toHaveLength(0)
    })

    it('aria-activedescendant matches highlighted option id', async () => {
      const { wrapper, entities } = await setupWithResults(2)
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      await flushPromises()
      const expected = `cmdk-option-${entities[1].id}`
      expect(input().getAttribute('aria-activedescendant')).toBe(expected)
      expect(options()[1].id).toBe(expected)
      wrapper.unmount()
    })

    it('Enter navigates to the highlighted entity and emits close', async () => {
      const { wrapper, entities } = await setupWithResults(2)
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
      await flushPromises()
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
      await flushPromises()

      expect(routerPush).toHaveBeenCalledWith(`/entity/${entities[1].type}/${entities[1].id}`)
      expect(wrapper.emitted('close')).toHaveLength(1)
      wrapper.unmount()
    })

    it('Enter is a no-op when results are empty', async () => {
      const wrapper = factory()
      input().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
      await flushPromises()
      expect(routerPush).not.toHaveBeenCalled()
      expect(wrapper.emitted('close')).toBeUndefined()
    })

    it('Escape emits close and stops propagation', async () => {
      const wrapper = factory()
      const event = new KeyboardEvent('keydown', {
        key: 'Escape',
        bubbles: true,
        cancelable: true,
      })
      const stopSpy = vi.spyOn(event, 'stopPropagation')
      input().dispatchEvent(event)
      await flushPromises()

      expect(stopSpy).toHaveBeenCalled()
      expect(wrapper.emitted('close')).toHaveLength(1)
      expect(routerPush).not.toHaveBeenCalled()
    })

    it('Tab is preventDefault’d so focus stays on the input', async () => {
      factory()
      await flushPromises()
      // After mount with open=true the input is auto-focused (immediate watcher).
      expect(document.activeElement).toBe(input())

      const event = new KeyboardEvent('keydown', {
        key: 'Tab',
        bubbles: true,
        cancelable: true,
      })
      input().dispatchEvent(event)

      expect(event.defaultPrevented).toBe(true)
      expect(document.activeElement).toBe(input())
    })
  })

  describe('selection', () => {
    it('clicking a result navigates and emits close', async () => {
      searchSpy.mockResolvedValueOnce(
        listResponse([makeEntity({ id: 'X-1', type: 'ticket' })])
      )
      const wrapper = factory()
      await typeQuery('xx')

      options()[0].click()
      await flushPromises()

      expect(routerPush).toHaveBeenCalledWith('/entity/ticket/X-1')
      expect(wrapper.emitted('close')).toHaveLength(1)
    })

    it('uses custom detail view when configured for the entity type', async () => {
      const schemaStore = useSchemaStore()
      schemaStore.entityViewConfigs.set('ticket', {
        detail_view: 'ticket-detail',
      } as never)

      searchSpy.mockResolvedValueOnce(
        listResponse([makeEntity({ id: 'X-2', type: 'ticket' })])
      )
      factory()
      await typeQuery('xx')

      options()[0].click()
      await flushPromises()

      expect(routerPush).toHaveBeenCalledWith('/view/ticket-detail/X-2')
    })

    it('does not navigate when entity has no type (empty href)', async () => {
      searchSpy.mockResolvedValueOnce(
        listResponse([makeEntity({ id: 'X-3', type: '' })])
      )
      const wrapper = factory()
      await typeQuery('xx')

      options()[0].click()
      await flushPromises()

      expect(routerPush).not.toHaveBeenCalled()
      expect(wrapper.emitted('close')).toBeUndefined()
    })
  })

  describe('overlay click', () => {
    it('emits close when backdrop is clicked', () => {
      const wrapper = factory()
      const overlay = document.querySelector<HTMLElement>('.cmdk-overlay')!
      overlay.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      expect(wrapper.emitted('close')).toHaveLength(1)
    })

    it('does not emit close when clicking inside the modal', () => {
      const wrapper = factory()
      const modal = document.querySelector<HTMLElement>('.cmdk-modal')!
      modal.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      expect(wrapper.emitted('close')).toBeUndefined()
    })
  })
})
