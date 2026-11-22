import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import FilterBar from './FilterBar.vue'
import type { EntityType, ListConfig } from '@/types'

// The drop-down itself, with the library's real popover: closed by default,
// opened by the trigger, closed by Escape, and the trigger counts the active
// filters. The controls inside are covered by FilterBar.test.ts.

vi.mock('@/stores', async (orig) => ({
  ...(await orig<typeof import('@/stores')>()),
  useEntitiesStore: () => ({ fetchList: vi.fn().mockResolvedValue({ data: [], meta: {} }) }),
  useUIStore: () => ({ error: vi.fn() }),
}))

const CONFIG: ListConfig = {
  entity: 'taak',
  columns: [],
  filter_controls: [
    { property: 'status', label: 'Status' },
    { property: 'owner', label: 'Owner' },
  ],
}
const TYPE: EntityType = {
  label: 'Taak',
  properties: { status: { type: 'enum', values: ['todo', 'done'] }, owner: { type: 'string' } },
}

function mountBar(filters = {}) {
  return mount(FilterBar, {
    props: { config: CONFIG, entityType: TYPE, filters },
    attachTo: document.body,
  })
}

const panel = () => document.querySelector('[data-testid="filter-panel"]')

afterEach(() => {
  document.body.innerHTML = ''
})

describe('FilterBar drop-down', () => {
  beforeEach(() => {
    vi.useRealTimers()
  })

  it('is closed until the trigger is pressed', async () => {
    const wrapper = mountBar()
    await flushPromises()

    const trigger = wrapper.find('[data-testid="filter-menu-button"]')
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(panel()).toBeNull()

    await trigger.trigger('click')
    await flushPromises()

    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(panel()).not.toBeNull()
    expect(panel()?.textContent).toContain('Owner')
    wrapper.unmount()
  })

  it('closes on Escape and gives the focus back to the trigger', async () => {
    const wrapper = mountBar()
    await flushPromises()
    await wrapper.find('[data-testid="filter-menu-button"]').trigger('click')
    await flushPromises()

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    expect(dialog.getAttribute('aria-label')).toBe('Filters')
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()

    expect(panel()).toBeNull()
    expect(document.activeElement).toBe(wrapper.find('[data-testid="filter-menu-button"]').element)
    wrapper.unmount()
  })

  it('counts the active filters on the trigger', async () => {
    const wrapper = mountBar({ status: { value: 'todo' }, owner: { value: 'Anna' } })
    await flushPromises()

    expect(wrapper.find('[data-testid="filter-menu-count"]').text()).toBe('2')
    wrapper.unmount()
  })

  it('shows no count when nothing is filtered', async () => {
    const wrapper = mountBar()
    await flushPromises()

    expect(wrapper.find('[data-testid="filter-menu-count"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
