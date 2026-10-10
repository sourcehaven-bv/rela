// TKT-CADCFX: a relation shown as a field among the entry's properties.
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import InlineRelationValue from './InlineRelationValue.vue'
import { useSchemaStore } from '@/stores/schema'
import { useEntitiesStore } from '@/stores'
import type { Entity } from '@/types'

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, listAllEntities: vi.fn() }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {}, path: '/', name: 'entity' }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
}))

import { listAllEntities } from '@/api'

const statuses: Entity[] = [
  { id: 'ST-1', type: 'status', _title: 'Backlog', properties: { titel: 'Backlog', categorie: 'open' } },
  { id: 'ST-2', type: 'status', _title: 'Gereed', properties: { titel: 'Gereed', categorie: 'gereed' } },
] as Entity[]

function seed(maxOutgoing?: number) {
  const schema = useSchemaStore()
  schema.entityTypes.set('status', {
    name: 'status',
    label: 'Status',
    primary_property: 'titel',
    properties: { titel: { type: 'string' }, categorie: { type: 'statuscategorie' } },
  } as never)
  schema.customTypes.set('statuscategorie', { values: ['open', 'gereed'] } as never)
  schema.styles = { statuscategorie: { open: 'badge-blue', gereed: 'badge-green' } }
  schema.relationTypes.set('heeft_status', {
    name: 'heeft_status',
    from: ['taak'],
    to: ['status'],
    max_outgoing: maxOutgoing,
  } as never)
  ;(listAllEntities as ReturnType<typeof vi.fn>).mockResolvedValue({ data: statuses })
}

async function mountField(
  targets: { id: string; title: string; style?: string }[],
  writable = true,
  styleFrom?: string
) {
  const wrapper = mount(InlineRelationValue, {
    props: {
      entityType: 'taak',
      entityId: 'TASK-1',
      relation: 'heeft_status',
      label: 'Status',
      targets,
      writable,
      styleFrom,
    },
    global: { stubs: { 'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' } } },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

async function openAndPick(wrapper: Awaited<ReturnType<typeof mountField>>, title: string) {
  await wrapper.find('.inline-relation-trigger').trigger('click')
  await flushPromises()
  const item = [...document.querySelectorAll<HTMLElement>('[role=menuitem]')].find(
    (el) => el.textContent?.trim() === title
  )
  expect(item, `menu item ${title}`).toBeTruthy()
  item!.click()
  await flushPromises()
}

describe('InlineRelationValue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
  })

  it('shows the targets as links when read-only', async () => {
    seed(1)
    const wrapper = await mountField([{ id: 'ST-1', title: 'Backlog' }], false)
    const link = wrapper.find('a')
    expect(link.text()).toBe('Backlog')
    expect(link.attributes('href')).toBe('/entity/status/ST-1')
    expect(wrapper.find('.inline-relation-trigger').exists()).toBe(false)
  })

  it('re-points a single-valued relation with a full linkage', async () => {
    seed(1)
    const update = vi.spyOn(useEntitiesStore(), 'update').mockResolvedValue({} as Entity)
    const wrapper = await mountField([{ id: 'ST-1', title: 'Backlog' }])
    await openAndPick(wrapper, 'Gereed')
    expect(update).toHaveBeenCalledWith('taak', 'TASK-1', {
      relations: { heeft_status: { data: [{ type: 'status', id: 'ST-2' }] } },
    })
    expect(wrapper.find('.inline-relation-trigger').text()).toBe('Gereed')
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('adds and removes single edges on a multi-valued relation', async () => {
    seed(undefined)
    const update = vi.spyOn(useEntitiesStore(), 'update').mockResolvedValue({} as Entity)
    const wrapper = await mountField([{ id: 'ST-1', title: 'Backlog' }])
    await openAndPick(wrapper, 'Gereed')
    expect(update).toHaveBeenLastCalledWith('taak', 'TASK-1', {
      relations: { heeft_status: { add: [{ type: 'status', id: 'ST-2' }] } },
    })
    await openAndPick(wrapper, 'Backlog')
    expect(update).toHaveBeenLastCalledWith('taak', 'TASK-1', {
      relations: { heeft_status: { remove: [{ type: 'status', id: 'ST-1' }] } },
    })
    expect(wrapper.find('.inline-relation-trigger').text()).toBe('Gereed')
  })

  it('restores the shown targets when the write fails', async () => {
    seed(1)
    vi.spyOn(useEntitiesStore(), 'update').mockRejectedValue(new Error('denied'))
    const wrapper = await mountField([{ id: 'ST-1', title: 'Backlog' }])
    await openAndPick(wrapper, 'Gereed')
    expect(wrapper.find('.inline-relation-trigger').text()).toBe('Backlog')
    expect(wrapper.emitted('error')?.[0]).toEqual(['denied'])
  })

  it('shows each target as a badge coloured by style_from', async () => {
    seed(1)
    vi.spyOn(useEntitiesStore(), 'update').mockResolvedValue({} as Entity)
    const wrapper = await mountField([{ id: 'ST-1', title: 'Backlog', style: 'open' }], true, 'categorie')
    const badge = wrapper.find('.inline-relation-trigger .badge')
    expect(badge.text()).toBe('Backlog')
    expect(badge.classes()).toContain('badge--blue')
    await openAndPick(wrapper, 'Gereed')
    const picked = wrapper.find('.inline-relation-trigger .badge')
    expect(picked.text()).toBe('Gereed')
    expect(picked.classes()).toContain('badge--green')
  })
})
