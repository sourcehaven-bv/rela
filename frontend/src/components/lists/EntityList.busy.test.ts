import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { toValue, type MaybeRefOrGetter } from 'vue'
import EntityList from './EntityList.vue'
import RlTable from 'rela-components/components/table/RlTable.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { i18n } from '@/i18n'
import type { BusyChannel, Entity, ListResponse } from '@/types'

// The busy column (TKT-TU3XH4): `busy: run` on a list column asks the list
// for the busy channels of type run and shows an indicator in that column.

const listEntitiesMock = vi.fn()
const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))

// The live path getters EntityList registers, read after the mount.
const livePaths: Array<MaybeRefOrGetter<string | null>> = []
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (_label: string, path: MaybeRefOrGetter<string | null>) => {
    livePaths.push(path)
  },
}))

const mockRoute: { query: Record<string, string>; path: string; name: string } = {
  query: {},
  path: '/list/tickets',
  name: 'list',
}
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => mockRoute,
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

const LIST_ID = 'tickets'
const ENTITY_TYPE = 'ticket'
const HOUR = 3_600_000

/** A busy channel of type that entered label hours ago. */
function busy(label: string, hours: number, type = 'run'): BusyChannel {
  return {
    type,
    type_label: type === 'run' ? 'Agent run' : undefined,
    state: label.toLowerCase(),
    label,
    since: new Date(Date.now() - hours * HOUR - 60_000).toISOString(),
  }
}

function ticket(id: string, extra: Partial<Entity> = {}): Entity {
  return { id, type: ENTITY_TYPE, properties: { title: `Ticket ${id}`, status: 'todo' }, ...extra }
}

describe('EntityList busy column', () => {
  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(ENTITY_TYPE, 'tickets')
    listEntitiesMock.mockReset()
    listAllEntitiesMock.mockReset()
    livePaths.length = 0
    mockRoute.query = {}
    localStorage.clear()
  })

  const mounted: VueWrapper[] = []
  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
    i18n.global.locale.value = 'en'
  })

  interface MountOptions {
    columns?: Array<Record<string, unknown>>
    rows?: Entity[]
    groupBy?: Record<string, unknown>
  }

  async function mountList({ columns, rows, groupBy }: MountOptions = {}) {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(LIST_ID, {
      id: LIST_ID,
      title: 'Tickets',
      entity: ENTITY_TYPE,
      columns: columns ?? [{ property: 'title', label: 'Title' }, { busy: 'run' }],
      group_by: groupBy,
    } as never)
    schemaStore.entityTypes.set(ENTITY_TYPE, {
      name: ENTITY_TYPE,
      label: 'Ticket',
      properties: {
        title: { type: 'string', values: null },
        status: { type: 'enum', values: ['todo', 'done'] },
      },
    } as never)
    const data = rows ?? [ticket('T-1')]
    const response = {
      data,
      meta: { total: data.length, page: 1, per_page: 25, has_more: false },
      included: {},
    } as ListResponse<Entity>
    listEntitiesMock.mockResolvedValue(response)
    listAllEntitiesMock.mockResolvedValue(response)
    const wrapper = mount(EntityList, {
      props: { listId: LIST_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada], stubs: { InlineCreateFormModal: true } },
    })
    mounted.push(wrapper)
    await flushPromises()
    return wrapper
  }

  /** The params of the most recent list read. */
  function lastParams(mock = listEntitiesMock): Record<string, unknown> {
    const call = mock.mock.calls[mock.mock.calls.length - 1]
    expect(call).toBeDefined()
    return call![1] as Record<string, unknown>
  }

  /** The columns EntityList hands the table. */
  function tableColumns(wrapper: VueWrapper): TableColumn[] {
    return (wrapper.findComponent(RlTable).props() as unknown as { columns: TableColumn[] }).columns
  }

  function busyColumn(wrapper: VueWrapper): TableColumn {
    const columns = tableColumns(wrapper)
    const col = columns.find((c) => c.key === 'busy')
    expect(col).toBeDefined()
    return col!
  }

  /** The busy cell of each row, in row order. */
  function busyCells(wrapper: VueWrapper) {
    const columns = tableColumns(wrapper)
    const index = columns.findIndex((c) => c.key === 'busy')
    return wrapper.findAll('.rl-table-row').map((row) => row.findAll('.rl-table-row__cell')[index]!)
  }

  it('sends busy for a busy column', async () => {
    await mountList()
    expect(lastParams()).toMatchObject({ busy: 'run', list_id: LIST_ID })
  })

  it('no busy without a busy column', async () => {
    await mountList({ columns: [{ property: 'title' }, { property: 'status' }] })
    expect(lastParams()).not.toHaveProperty('busy')
    expect(toValue(livePaths[0])).not.toContain('busy=')
  })

  it('live path carries busy', async () => {
    await mountList()
    expect(livePaths).toHaveLength(1)
    expect(toValue(livePaths[0])).toContain('busy=run')
  })

  it('grouped loader receives busy', async () => {
    await mountList({ groupBy: { property: 'status', max_rows: 300 } })
    expect(listEntitiesMock).not.toHaveBeenCalled()
    // listAllEntities forwards these params to every page; its own test
    // "forwards busy and list_id to every page" pins that.
    expect(lastParams(listAllEntitiesMock)).toMatchObject({ busy: 'run' })
    expect(toValue(livePaths[0])).toContain('busy=run')
  })

  it('searched list still sends busy', async () => {
    mockRoute.query = { q: 'needle' }
    await mountList()
    expect(lastParams()).toMatchObject({ busy: 'run', q: 'needle' })
    // The server refuses q on a live path, so a searched list has none.
    expect(toValue(livePaths[0])).toBeNull()
  })

  it('indicator in the cell', async () => {
    const wrapper = await mountList({ rows: [ticket('T-1', { _busy: [busy('Planning', 3)] })] })
    const [cell] = busyCells(wrapper)
    expect(cell!.find('.rl-busy').exists()).toBe(true)
    expect(cell!.find('.rl-visually-hidden').text()).toBe('Agent run: Planning, for 3 hours')
  })

  it('+1 for two entries', async () => {
    const wrapper = await mountList({
      rows: [ticket('T-1', { _busy: [busy('Planning', 3), busy('Review', 1)] })],
    })
    const [cell] = busyCells(wrapper)
    expect(cell!.find('.rl-busy__label').text()).toBe('Planning')
    expect(cell!.find('.rl-busy__tail').text()).toContain('+1')
    const name = cell!.find('.rl-visually-hidden').text()
    expect(name).toContain('Agent run: Planning, for 3 hours')
    expect(name).toContain('Agent run: Review, for 1 hour')
  })

  it('empty for [] and absent', async () => {
    const wrapper = await mountList({ rows: [ticket('T-1', { _busy: [] }), ticket('T-2')] })
    for (const cell of busyCells(wrapper)) {
      expect(cell.find('.rl-busy').exists()).toBe(false)
      expect(cell.text()).not.toContain('run')
    }
  })

  it('ignores another type', async () => {
    const wrapper = await mountList({
      rows: [ticket('T-1', { _busy: [busy('Working', 2, 'job')] })],
    })
    const [cell] = busyCells(wrapper)
    expect(cell!.find('.rl-busy').exists()).toBe(false)
  })

  it('header label and i18n', async () => {
    let wrapper = await mountList({
      columns: [{ property: 'title' }, { busy: 'run', label: 'Agent' }],
    })
    expect(busyColumn(wrapper)).toMatchObject({ header: 'Agent', sortable: false, width: 160 })
    wrapper.unmount()
    mounted.length = 0

    wrapper = await mountList()
    expect(busyColumn(wrapper).header).toBe('Busy')
    wrapper.unmount()
    mounted.length = 0

    i18n.global.locale.value = 'nl'
    wrapper = await mountList()
    expect(busyColumn(wrapper).header).toBe('Bezig')
  })

  it('primary and isEmpty', async () => {
    const idle = ticket('T-1', { _busy: [] })
    const working = ticket('T-2', { _busy: [busy('Planning', 3)] })
    const wrapper = await mountList({ rows: [idle, working] })
    const col = busyColumn(wrapper)
    expect(col.primary).toBe(true)
    expect(col.isEmpty!(idle)).toBe(true)
    expect(col.isEmpty!(ticket('T-3'))).toBe(true)
    expect(col.isEmpty!(working)).toBe(false)
  })

  it('grouped by the first column keeps the busy column', async () => {
    const wrapper = await mountList({
      columns: [{ property: 'status', label: 'Status' }, { busy: 'run' }, { property: 'title' }],
      groupBy: { property: 'status', max_rows: 300 },
      rows: [ticket('T-1', { _busy: [busy('Planning', 3)] })],
    })
    const columns = tableColumns(wrapper)
    expect(columns.map((c) => c.key)).toEqual(['busy', 'prop:title'])
    expect(wrapper.find('.rl-busy').exists()).toBe(true)
  })
})
