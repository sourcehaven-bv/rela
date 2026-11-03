import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import RlBoard from 'rela-components/components/board/RlBoard.vue'
import type { Section } from 'rela-components/types'
import { useSchemaStore } from '@/stores/schema'
import { usePageStore } from '@/stores/pages'
import { useUIStore } from '@/stores'
import { _setEntityPluralForTest } from '@/api/entities'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import type { Entity, ListResponse } from '@/types'

// A wall (`layout: wall`, TKT-0VBQW0H). Without columns it is one grid of
// every card. With `columns_from` the loose cards are a pile and the targets
// are groups: a card dropped on a loose card makes a group, offered by the
// page's entity; a drop on a grouped card joins its group; a drop on the pile
// leaves it. Groups move by their handle and are renamed in place.

const listAllEntitiesMock = vi.fn()
const getEntityRelationsMock = vi.fn()
const createRelationMock = vi.fn()
const moveRelationMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  getEntityRelations: (...args: unknown[]) => getEntityRelationsMock(...args),
  createRelation: (...args: unknown[]) => createRelationMock(...args),
  moveRelation: (...args: unknown[]) => moveRelationMock(...args),
}))

const createEntityMock = vi.fn()
const updateEntityMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  createEntity: (...args: unknown[]) => createEntityMock(...args),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/p/sessie/SES-1/wand' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

function page(data: Entity[]): ListResponse<Entity> {
  return {
    data,
    meta: { total: data.length, page: 1, per_page: 100, has_more: false },
    included: {},
    _actions: { create: true },
  } as unknown as ListResponse<Entity>
}

function note(id: string, tekst: string, cluster?: string): Entity {
  return {
    id,
    type: 'notitie',
    properties: { tekst },
    relations: cluster ? { in_cluster: [cluster] } : {},
    _actions: { update: true },
  } as unknown as Entity
}

const clusters = [
  { id: 'CL-1', type: 'cluster', properties: { titel: 'Groep A' } },
  { id: 'CL-2', type: 'cluster', properties: { titel: 'Groep B' } },
] as unknown as Entity[]

let notes: Entity[]

const scope = { page: 'sessie', tab: 'wand', entity: 'SES-1' }

function seed() {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set('wand', {
    entity: 'notitie',
    title: 'Wand',
    column_property: '',
    layout: 'wall',
    columns_from: { relation: 'in_cluster', offered_by: 'heeft_cluster' },
    card: { title: 'tekst', style: 'note' },
  } as never)
  schemaStore.kanbans.set('muur', {
    entity: 'notitie',
    title: 'Muur',
    column_property: '',
    layout: 'wall',
    card: { title: 'tekst', style: 'note' },
  } as never)
  schemaStore.kanbans.set('bord', {
    entity: 'notitie',
    title: 'Bord',
    column_property: '',
    columns_from: { relation: 'in_cluster', offered_by: 'heeft_cluster' },
    card: { title: 'tekst' },
  } as never)
  schemaStore.entityTypes.set('notitie', {
    label: 'Notitie',
    properties: { tekst: { type: 'string' } },
  } as never)
  schemaStore.entityTypes.set('cluster', {
    label: 'Cluster',
    primary: 'titel',
    properties: { titel: { type: 'string' } },
  } as never)
  schemaStore.relationTypes.set('in_cluster', {
    label: 'in cluster',
    from: ['notitie'],
    to: ['cluster'],
    max_outgoing: 1,
  } as never)
  schemaStore.relationTypes.set('heeft_cluster', {
    label: 'heeft cluster',
    from: ['sessie'],
    to: ['cluster'],
    orderable: { outgoing: true },
  } as never)
  usePageStore().set({
    sessie: {
      label: 'Sessie',
      entity_type: 'sessie',
      tabs: [
        {
          id: 'wand', label: 'Wand', view: 'kanban', target: 'wand',
          scope: 'relation', relation: 'bevat', direction: 'outgoing',
          links: [{ type: 'notitie', relation: 'bevat', direction: 'outgoing' }],
        },
      ],
    },
  })
  listAllEntitiesMock.mockImplementation((type: string) =>
    Promise.resolve(type === 'cluster' ? page(clusters) : page(notes))
  )
  getEntityRelationsMock.mockResolvedValue([
    { id: 'CL-1', meta: { _order_out: 1 } },
    { id: 'CL-2', meta: { _order_out: 2 } },
  ])
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('notitie', 'notities')
  _setEntityPluralForTest('cluster', 'clusters')
  _setEntityPluralForTest('sessie', 'sessies')
  notes = [note('N-1', 'In een groep', 'CL-1'), note('N-2', 'Los een'), note('N-3', 'Los twee')]
  for (const m of [
    listAllEntitiesMock,
    getEntityRelationsMock,
    createRelationMock,
    moveRelationMock,
    createEntityMock,
    updateEntityMock,
  ]) m.mockReset()
  createEntityMock.mockResolvedValue({ id: 'CL-NEW', type: 'cluster', properties: {} })
  updateEntityMock.mockImplementation((type: string, id: string) => Promise.resolve({ id, type, properties: {} }))
  createRelationMock.mockResolvedValue(undefined)
  moveRelationMock.mockResolvedValue(undefined)
})

afterEach(() => {
  document.body.innerHTML = ''
})

// `setup` runs after the seed, to change what it set up.
async function mountBoard(id = 'wand', setup?: () => void) {
  seed()
  setup?.()
  const wrapper = mount(withPageHeader(KanbanView), {
    attrs: { id, pageScope: { ...scope, tab: id } },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

type Wrapper = Awaited<ReturnType<typeof mountBoard>>

function board(wrapper: Wrapper) {
  return wrapper.findComponent(RlBoard)
}

interface BoardProps {
  sections: Section[]
  group: boolean
  sectionsMovable?: (s: Section) => boolean
}

function props(wrapper: Wrapper): BoardProps {
  return board(wrapper).props() as unknown as BoardProps
}

function sections(wrapper: Wrapper): Section[] {
  return props(wrapper).sections
}

function card(wrapper: Wrapper, id: string) {
  for (const s of sections(wrapper)) {
    const item = s.items.find((i) => i.id === id)
    if (item) return { item, section: s }
  }
  throw new Error(`no card ${id}`)
}

function section(wrapper: Wrapper, id: string) {
  const found = sections(wrapper).find((s) => s.id === id)
  if (!found) throw new Error(`no section ${id}`)
  return found
}

async function dropOnto(wrapper: Wrapper, dragged: string, onto: string) {
  const { section: to } = card(wrapper, onto)
  board(wrapper).vm.$emit('move', { item: card(wrapper, dragged).item, to, at: { targetId: onto, placement: 'onto' } })
  await flushPromises()
}

function relationPatches() {
  return updateEntityMock.mock.calls
    .filter(([type]) => type === 'notitie')
    .map(([, id, body]) => [id, body.relations.in_cluster.data.map((d: { id: string }) => d.id)])
}

describe('KanbanView wall with groups', () => {
  it('shows the loose cards as a pile and the targets as groups', async () => {
    const wrapper = await mountBoard()
    expect(sections(wrapper).map((s) => [s.id, s.items.map((i) => i.id)])).toEqual([
      ['CL-1', ['N-1']],
      ['CL-2', []],
      ['__other__', ['N-2', 'N-3']],
    ])
    expect(props(wrapper).group).toBe(true)
    expect(wrapper.find('.kanban-board--grouping').exists()).toBe(true)
    expect(wrapper.text()).toContain('Not grouped yet')
  })

  it('creates a group when a card is dropped on a loose card', async () => {
    const wrapper = await mountBoard()
    await dropOnto(wrapper, 'N-3', 'N-2')

    expect(createEntityMock).toHaveBeenCalledWith('cluster', { properties: { titel: 'Los een' } })
    expect(createRelationMock).toHaveBeenCalledWith('sessie', 'SES-1', 'heeft_cluster', 'CL-NEW')
    // The target first, then the dropped card: a failure between them
    // leaves a group of one, not a group with only the dragged card.
    expect(relationPatches()).toEqual([
      ['N-2', ['CL-NEW']],
      ['N-3', ['CL-NEW']],
    ])
  })

  it('cuts a long card title for the new group', async () => {
    notes = [note('N-2', 'x'.repeat(60)), note('N-3', 'Los twee')]
    const wrapper = await mountBoard()
    await dropOnto(wrapper, 'N-3', 'N-2')
    expect(createEntityMock.mock.calls[0][1].properties.titel).toBe(`${'x'.repeat(40)}…`)
  })

  it('creates an untitled group, named by the server, when the type has no display property', async () => {
    seed()
    useSchemaStore().entityTypes.set('cluster', {
      label: 'Cluster',
      properties: { titel: { type: 'string' } },
    } as never)
    const wrapper = mount(withPageHeader(KanbanView), {
      attrs: { id: 'wand', pageScope: scope },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    expect(wrapper.find('input.wall-heading__title').exists()).toBe(false)
    await dropOnto(wrapper, 'N-3', 'N-2')
    expect(createEntityMock).toHaveBeenCalledWith('cluster', { properties: {} })
  })

  it('refuses to group a card that is in a group this board does not show', async () => {
    notes = [note('N-2', 'Elders', 'CL-9'), note('N-3', 'Los twee')]
    const wrapper = await mountBoard()
    await dropOnto(wrapper, 'N-3', 'N-2')
    expect(createEntityMock).not.toHaveBeenCalled()
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(useUIStore().toasts.map((x) => x.message)).toEqual(['This card is in a group this board does not show'])
  })

  it.each([
    ['the target card may not be updated', () => {
      notes = [{ ...note('N-2', 'Los een'), _actions: { update: false } } as Entity, note('N-3', 'Los twee')]
    }],
    ['the reader may not create a group', () => {
      listAllEntitiesMock.mockImplementation((type: string) =>
        Promise.resolve(type === 'cluster' ? { ...page(clusters), _actions: { create: false } } : page(notes))
      )
    }],
  ])('makes no group when %s', async (_name, deny) => {
    const wrapper = await mountBoard('wand', deny)
    await dropOnto(wrapper, 'N-3', 'N-2')
    expect(createEntityMock).not.toHaveBeenCalled()
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(useUIStore().toasts.map((x) => x.message)).toEqual(['You may not make a group of these cards'])
  })

  it('handles quick drops on one loose card one at a time, making one group', async () => {
    notes = [note('N-2', 'Los een'), note('N-3', 'Los twee'), note('N-4', 'Los drie')]
    updateEntityMock.mockImplementation((type: string, id: string, body: { relations: Record<string, { data: { id: string }[] }> }) => {
      const at = notes.findIndex((n) => n.id === id)
      notes[at] = note(id, String(notes[at].properties.tekst), body.relations.in_cluster.data[0]?.id)
      return Promise.resolve({ id, type, properties: {} })
    })
    const made = () => createEntityMock.mock.calls.length > 0
    const wrapper = await mountBoard('wand', () => {
      getEntityRelationsMock.mockImplementation(() =>
        Promise.resolve(made() ? [{ id: 'CL-NEW', meta: { _order_out: 1 } }] : [])
      )
      listAllEntitiesMock.mockImplementation((type: string) =>
        Promise.resolve(type === 'cluster'
          ? page(made() ? [{ id: 'CL-NEW', type: 'cluster', properties: { titel: 'Los een' } } as unknown as Entity] : [])
          : page(notes))
      )
    })
    const target = card(wrapper, 'N-2').section
    board(wrapper).vm.$emit('move', { item: card(wrapper, 'N-3').item, to: target, at: { targetId: 'N-2', placement: 'onto' } })
    board(wrapper).vm.$emit('move', { item: card(wrapper, 'N-4').item, to: target, at: { targetId: 'N-2', placement: 'onto' } })
    await flushPromises()

    expect(createEntityMock).toHaveBeenCalledTimes(1)
    expect(relationPatches()).toEqual([
      ['N-2', ['CL-NEW']],
      ['N-3', ['CL-NEW']],
      ['N-4', ['CL-NEW']],
    ])
  })

  it('cuts a title without splitting a character made of two code units', async () => {
    notes = [note('N-2', `${'x'.repeat(39)}😀😀`), note('N-3', 'Los twee')]
    const wrapper = await mountBoard()
    await dropOnto(wrapper, 'N-3', 'N-2')
    expect(createEntityMock.mock.calls[0][1].properties.titel).toBe(`${'x'.repeat(39)}😀…`)
  })

  it('shows the name of a group the reader may not update as text', async () => {
    clusters[0] = { ...clusters[0], _actions: { update: false } } as Entity
    try {
      const wrapper = await mountBoard()
      const heading = wrapper.find('[data-section-id="CL-1"] .wall-heading__title')
      expect(heading.element.tagName).toBe('SPAN')
      expect(wrapper.find('[data-section-id="CL-2"] input.wall-heading__title').exists()).toBe(true)
    } finally {
      clusters[0] = { id: 'CL-1', type: 'cluster', properties: { titel: 'Groep A' } } as unknown as Entity
    }
  })

  it('joins the group of a grouped card without creating one', async () => {
    const wrapper = await mountBoard()
    await dropOnto(wrapper, 'N-2', 'N-1')
    expect(createEntityMock).not.toHaveBeenCalled()
    expect(relationPatches()).toEqual([['N-2', ['CL-1']]])
  })

  it('writes nothing when a card is dropped on a card of its own group', async () => {
    notes = [note('N-1', 'Een', 'CL-1'), note('N-2', 'Twee', 'CL-1')]
    const wrapper = await mountBoard()
    await dropOnto(wrapper, 'N-2', 'N-1')
    expect(updateEntityMock).not.toHaveBeenCalled()
  })

  it('reports a failed create and writes nothing else', async () => {
    createEntityMock.mockRejectedValue(new Error('titel is required'))
    const wrapper = await mountBoard()
    const ui = useUIStore()
    const error = vi.spyOn(ui, 'error')
    await dropOnto(wrapper, 'N-3', 'N-2')
    expect(error).toHaveBeenCalled()
    expect(createRelationMock).not.toHaveBeenCalled()
    expect(updateEntityMock).not.toHaveBeenCalled()
  })

  it('takes a card out of its group when dropped on the pile', async () => {
    const wrapper = await mountBoard()
    board(wrapper).vm.$emit('move', { item: card(wrapper, 'N-1').item, to: section(wrapper, '__other__') })
    await flushPromises()
    expect(relationPatches()).toEqual([['N-1', []]])
  })

  it('moves a group by its handle in the offered_by order', async () => {
    const wrapper = await mountBoard()
    const movable = props(wrapper).sectionsMovable as (s: Section) => boolean
    expect(movable(section(wrapper, 'CL-1'))).toBe(true)
    expect(movable(section(wrapper, '__other__'))).toBe(false)

    board(wrapper).vm.$emit('moveSection', { sectionId: 'CL-2', targetId: 'CL-1', placement: 'before' })
    await flushPromises()
    expect(moveRelationMock).toHaveBeenCalledWith('sessie', 'SES-1', 'heeft_cluster', 'CL-2', { before: 'CL-1' })
  })

  it('renames a group from its heading and restores an empty name', async () => {
    const wrapper = await mountBoard()
    const input = wrapper.find<HTMLInputElement>('input.wall-heading__title')
    const before = input.element.value
    await input.setValue('Nieuwe naam')
    await flushPromises()
    expect(updateEntityMock).toHaveBeenCalledWith('cluster', 'CL-1', { properties: { titel: 'Nieuwe naam' } }, undefined, undefined)

    updateEntityMock.mockClear()
    await input.setValue('   ')
    await flushPromises()
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(input.element.value).toBe(before)
  })

  it('refetches the groups when a card points at one it has not loaded', async () => {
    notes = [note('N-1', 'Een', 'CL-9')]
    await mountBoard()
    // Once for the board, once for the unknown group, and not again.
    expect(getEntityRelationsMock).toHaveBeenCalledTimes(2)
  })
})

describe('KanbanView plain wall', () => {
  it('shows every card in one section without grouping', async () => {
    const wrapper = await mountBoard('muur')
    expect(sections(wrapper).map((s) => s.items.map((i) => i.id))).toEqual([['N-1', 'N-2', 'N-3']])
    expect(props(wrapper).group).toBe(false)
    expect(wrapper.find('.wall-heading').exists()).toBe(false)
    expect(wrapper.find('.kanban-board--single').exists()).toBe(true)
  })

  it('does not group on a drop on a card', async () => {
    const wrapper = await mountBoard('muur')
    await dropOnto(wrapper, 'N-3', 'N-2')
    expect(createEntityMock).not.toHaveBeenCalled()
    expect(updateEntityMock).not.toHaveBeenCalled()
  })
})

describe('KanbanView columns_from without a wall', () => {
  it('does not group on a drop on a card', async () => {
    const wrapper = await mountBoard('bord')
    expect(props(wrapper).group).toBe(false)
    await dropOnto(wrapper, 'N-3', 'N-1')
    expect(createEntityMock).not.toHaveBeenCalled()
    expect(updateEntityMock).not.toHaveBeenCalled()
  })
})
