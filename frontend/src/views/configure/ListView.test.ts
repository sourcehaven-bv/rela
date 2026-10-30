import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlTable from 'rela-components/components/table/RlTable.vue'
import ListView from './ListView.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { dataEntryFixture, snapshot, toTree } from '@/configure/__tests__/fixture'
import { get, getIn, isList } from '@/configure/tree'

// The Configure list page with a busy column (TKT-TU3XH4). The page has no
// control to add one, but it must show and keep one that exists.

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ path: '/configure/lists/tickets', query: {}, params: {} }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

function load() {
  const store = useConfigDraftStore()
  store.load({
    ...snapshot(),
    data_entry: toTree({
      ...dataEntryFixture,
      lists: {
        tickets: {
          title: 'Tickets',
          entity_type: 'ticket',
          columns: [{ property: 'title' }, { busy: 'run' }, { property: 'status', sortable: true }],
        },
      },
    }),
  })
  return store
}

/** The columns of the tickets list in the current draft. */
function draftColumns(store: ReturnType<typeof useConfigDraftStore>) {
  const cols = getIn(store.currentDataEntry, ['lists', 'tickets', 'columns'])
  if (!isList(cols)) throw new Error('no columns')
  return cols
}

describe('Configure ListView with a busy column', () => {
  const mounted: VueWrapper[] = []
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })
  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
  })

  async function mountView() {
    const store = load()
    const wrapper = mount(ListView, { props: { name: 'tickets' }, attachTo: document.body })
    mounted.push(wrapper)
    await flushPromises()
    return { store, wrapper }
  }

  it('busy row has no Sortable switch and shows busy: run', async () => {
    const { wrapper } = await mountView()
    const rows = wrapper.findAll('.column-row')
    expect(rows).toHaveLength(3)
    const busy = rows[1]
    expect(busy.find('.column-row__source').text()).toBe('busy: run')
    // The placeholder and the preview header are the list's own default.
    expect(busy.find('input[placeholder="Busy"]').exists()).toBe(true)
    const preview = (
      wrapper.findComponent(RlTable).props() as unknown as { columns: Array<{ header: string }> }
    ).columns
    expect(preview.map((c) => c.header)).toEqual(['Busy', 'status'])
    expect(busy.text()).not.toContain('Sortable')
    expect(rows[2].text()).toContain('Sortable')
    expect(wrapper.text()).toContain('A busy column cannot be the first column.')
  })

  it('move to the top is refused', async () => {
    const { store, wrapper } = await mountView()
    const list = wrapper.findComponent(RlSortableList)
    const items = (list.props() as unknown as { items: Array<{ index: number }> }).items
    expect(wrapper.find('[data-testid="config-column-refused"]').exists()).toBe(false)

    // Pick the busy row up with the keyboard, move it to the top, put it down.
    const handle = wrapper.find('button[aria-label="Reorder run, 2 of 3"]')
    await handle.trigger('keydown', { key: 'Enter' })
    await wrapper.find('button[aria-pressed="true"]').trigger('keydown', { key: 'Home' })
    await wrapper.find('button[aria-pressed="true"]').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(draftColumns(store).map((c) => get(c, 'busy') ?? get(c, 'property'))).toEqual([
      'title',
      'run',
      'status',
    ])
    const status = list.find('[role="status"]').text()
    expect(status).toContain('A busy column cannot be the first column.')
    expect(status).not.toContain('dropped')
    expect(wrapper.find('[data-testid="config-column-refused"]').text()).toContain(
      'A busy column cannot be the first column.'
    )

    // Removing the title above the busy column is refused too.
    const remove = wrapper.find('button[aria-label="Remove title"]')
    expect(remove.attributes('disabled')).toBeDefined()
    await remove.trigger('click')
    expect(draftColumns(store)).toHaveLength(3)

    // A move that keeps the title first is applied, keeps busy, and clears
    // the refusal.
    list.vm.$emit('move', items[1], 2)
    await flushPromises()
    expect(wrapper.find('[data-testid="config-column-refused"]').exists()).toBe(false)
    expect(draftColumns(store).map((c) => get(c, 'busy') ?? get(c, 'property'))).toEqual([
      'title',
      'status',
      'run',
    ])
  })

  it('relabelling a busy column keeps busy', async () => {
    const { store, wrapper } = await mountView()
    await wrapper.findAll('.column-row')[1].find('input').setValue('Agent')
    const busy = draftColumns(store)[1]
    expect(get(busy, 'busy')).toBe('run')
    expect(get(busy, 'label')).toBe('Agent')
  })
})
