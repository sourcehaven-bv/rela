import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { BusyChannel, Entity, ListResponse } from '@/types'

// The busy card field (TKT-21G6JS): `busy: run` asks the list for the busy
// channels of type run and shows an indicator on a card that has one.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'ticket'
const HOUR = 3_600_000

/** A busy run channel that entered `label` `hours` hours ago. */
function busy(name: string, label: string, hours: number): BusyChannel {
  return {
    type: 'run',
    type_label: 'Agent run',
    name,
    state: label.toLowerCase(),
    label,
    since: new Date(Date.now() - hours * HOUR - 60_000).toISOString(),
  }
}

describe('KanbanView busy card field', () => {
  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(ENTITY_TYPE, 'tickets')
    listAllEntitiesMock.mockReset()
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  async function mountBoard(fields: Array<Record<string, unknown>>, entity: Partial<Entity>) {
    const schemaStore = useSchemaStore()
    schemaStore.kanbans.set(KANBAN_ID, {
      entity: ENTITY_TYPE,
      title: 'Board',
      column_property: 'status',
      columns: [{ value: 'todo', label: 'Todo' }],
      card: { title: 'title', fields },
    } as never)
    schemaStore.entityTypes.set(ENTITY_TYPE, {
      name: ENTITY_TYPE,
      label: 'Ticket',
      properties: { title: { type: 'string' }, status: { type: 'enum', values: ['todo'] } },
    } as never)
    const response: ListResponse<Entity> = {
      data: [
        {
          id: 'T-1',
          type: ENTITY_TYPE,
          properties: { title: 'Ticket T-1', status: 'todo' },
          relations: {},
          ...entity,
        },
      ],
      meta: { total: 1, page: 1, per_page: 25, has_more: false },
      included: {},
    }
    listAllEntitiesMock.mockResolvedValue(response)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return wrapper
  }

  function requestParams(): Record<string, unknown> {
    return (listAllEntitiesMock.mock.calls[0][1] ?? {}) as Record<string, unknown>
  }

  /** The text a screen reader hears for the indicator. */
  function accessibleName(wrapper: Awaited<ReturnType<typeof mountBoard>>): string {
    return wrapper.find('.rl-busy .rl-visually-hidden').text()
  }

  it('asks the list for the busy channels of its type', async () => {
    await mountBoard([{ busy: 'run' }], {})
    expect(requestParams().busy).toBe('run')
  })

  it('does not ask when no card field is busy', async () => {
    await mountBoard([{ comments: true }], {})
    expect(requestParams().busy).toBeUndefined()
  })

  it('shows the state and the time in it, and names both once', async () => {
    const wrapper = await mountBoard([{ busy: 'run', show_label: false }], {
      _busy: [busy('run-1', 'Planning', 3)],
    })
    const indicator = wrapper.find('.rl-busy')
    expect(indicator.find('.rl-busy__text').text()).toBe('Planning · 3h')
    expect(indicator.find('.rl-busy__text').attributes('aria-hidden')).toBe('true')
    expect(accessibleName(wrapper)).toBe('Agent run: Planning, for 3 hours')
    // Not a live region: a board refresh must not announce every card.
    expect(indicator.attributes('role')).toBeUndefined()
    expect(indicator.attributes('aria-live')).toBeUndefined()
    expect(indicator.find('[role],[aria-live]').exists()).toBe(false)
  })

  it('shows the first entry with +1 and names both', async () => {
    const wrapper = await mountBoard([{ busy: 'run', show_label: false }], {
      _busy: [busy('run-1', 'Planning', 3), busy('run-2', 'Implementing', 1)],
    })
    expect(wrapper.find('.rl-busy__text').text()).toBe('Planning · 3h +1')
    expect(accessibleName(wrapper)).toBe(
      'Agent run: Planning, for 3 hours; Agent run: Implementing, for 1 hour'
    )
  })

  it('shows no label by default, and the type or the label when asked', async () => {
    const plain = await mountBoard([{ busy: 'run' }], { _busy: [busy('run-1', 'Planning', 3)] })
    expect(plain.find('.card-field .field-label').exists()).toBe(false)
    expect(accessibleName(plain)).toBe('Agent run: Planning, for 3 hours')
    const typed = await mountBoard([{ busy: 'run', show_label: true }], {
      _busy: [busy('run-1', 'Planning', 3)],
    })
    expect(typed.find('.card-field .field-label').text()).toBe('run:')
    const labelled = await mountBoard([{ busy: 'run', label: 'Agent', show_label: true }], {
      _busy: [busy('run-1', 'Planning', 3)],
    })
    expect(labelled.find('.card-field .field-label').text()).toBe('Agent:')
  })

  it.each([
    ['no busy channel', { _busy: [] }],
    ['a row the server did not answer for', {}],
  ])('renders nothing for %s', async (_name, entity) => {
    const wrapper = await mountBoard([{ busy: 'run' }], entity as Partial<Entity>)
    expect(wrapper.find('.rl-busy').exists()).toBe(false)
    expect(wrapper.find('.card-field').exists()).toBe(false)
  })
})
