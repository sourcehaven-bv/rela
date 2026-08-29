import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import FilterBar from './FilterBar.vue'
import type { EntityType, ListConfig } from '@/types'

// FilterBar loads relation-filter candidates on mount; no relation controls are
// used here, but the stores must still resolve.
const fetchListMock = vi.fn()
const uiErrorMock = vi.fn()
vi.mock('@/stores', async (orig) => {
  const actual = await orig<typeof import('@/stores')>()
  return {
    ...actual,
    useEntitiesStore: () => ({ fetchList: fetchListMock }),
    useUIStore: () => ({ error: uiErrorMock }),
  }
})

// TagSelect wraps SlimSelect, whose MutationObserver doesn't mount under jsdom
// (same reason wrapperWidgets.test.ts stubs it).
const stubs = { TagSelect: true }

const GEBIEDEN_CONFIG: ListConfig = {
  entity: 'kader',
  columns: [],
  filter_controls: [{ property: 'gebieden', label: 'Gebieden' }],
}

const KADER_TYPE: EntityType = {
  label: 'Kader',
  properties: {
    gebieden: { type: 'enum', list: true, values: ['Governance', 'Technologie', 'Privacy'] },
  },
}

function mountBar(filters = {}) {
  return mount(FilterBar, {
    props: { config: GEBIEDEN_CONFIG, entityType: KADER_TYPE, filters },
    global: { stubs },
  })
}

describe('FilterBar — multi-enum filter (BUG-AMK38R)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    fetchListMock.mockReset()
    uiErrorMock.mockReset()
    fetchListMock.mockResolvedValue({ data: [], meta: {} })
  })

  it('renders a TagSelect, not a native <select multiple>', async () => {
    const wrapper = mountBar()
    await flushPromises()

    expect(wrapper.findComponent({ name: 'TagSelect' }).exists()).toBe(true)
    // The native multi-select listbox is what this replaced.
    expect(wrapper.find('select[multiple]').exists()).toBe(false)
  })

  it('passes the enum options to the tag picker', async () => {
    const wrapper = mountBar()
    await flushPromises()

    const tag = wrapper.findComponent({ name: 'TagSelect' })
    expect(tag.props('options')).toEqual(['Governance', 'Technologie', 'Privacy'])
  })

  it('emits a single selection under `=`, which is comma-safe', async () => {
    const wrapper = mountBar()
    await flushPromises()

    wrapper.findComponent({ name: 'TagSelect' }).vm.$emit('update:modelValue', ['Governance'])
    await flushPromises()

    const emitted = wrapper.emitted('filter')
    expect(emitted).toBeTruthy()
    // One value stays on `=`: it compares whole, so a value containing a comma
    // still matches. The comma-joined `in` form could not represent it.
    expect(emitted![emitted!.length - 1][0]).toEqual({
      gebieden: { value: 'Governance' },
    })
  })

  it('switches to `in` once more than one value is selected', async () => {
    const wrapper = mountBar()
    await flushPromises()

    wrapper
      .findComponent({ name: 'TagSelect' })
      .vm.$emit('update:modelValue', ['Governance', 'Privacy'])
    await flushPromises()

    const emitted = wrapper.emitted('filter')!
    expect(emitted[emitted.length - 1][0]).toEqual({
      gebieden: { value: 'Governance,Privacy', op: 'in' },
    })
  })

  it('clearing the selection drops the filter entirely', async () => {
    const wrapper = mountBar({ gebieden: { value: 'Governance', op: 'in' } })
    await flushPromises()

    wrapper.findComponent({ name: 'TagSelect' }).vm.$emit('update:modelValue', [])
    await flushPromises()

    const emitted = wrapper.emitted('filter')!
    expect(emitted[emitted.length - 1][0]).toEqual({})
  })

  it('hydrates the picker from an incoming filter value', async () => {
    const wrapper = mountBar({ gebieden: { value: 'Governance,Privacy', op: 'in' } })
    await flushPromises()

    const tag = wrapper.findComponent({ name: 'TagSelect' })
    expect(tag.props('modelValue')).toEqual(['Governance', 'Privacy'])
  })
})
