// BUG-NSGD6OE: an edit made on one entity must never be written to another.
//
// The router reuses FormView when only `:entityId` changes
// (`/form/f/PRC-X` → `/form/f/PRC-Y`): App.vue's RouterView has no key, and a
// param change runs `onBeforeRouteUpdate`, not `onBeforeRouteLeave`. These
// tests mount the real FormView and DynamicForm, run the guard the router
// would run, and switch the prop the way the router does.

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import type { RouteLocationNormalized } from 'vue-router'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import FormView from '@/views/FormView.vue'
import type { Entity } from '@/types'

type UpdateGuard = (to: RouteLocationNormalized, from: RouteLocationNormalized) => unknown
const updateGuards: UpdateGuard[] = []
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/form/procedure-form' }),
  onBeforeRouteLeave: vi.fn(),
  onBeforeRouteUpdate: (fn: UpdateGuard) => updateGuards.push(fn),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, getTemplates: vi.fn().mockResolvedValue([]) }
})

const ENTITY_TYPE = {
  name: 'procedure',
  label: 'Procedure',
  id_type: 'short',
  properties: { title: { type: 'string' }, owner: { type: 'string' } },
}

const FORM = {
  id: 'procedure-form',
  entity: 'procedure',
  fields: [{ property: 'title' }, { property: 'owner' }],
}

function procedure(id: string, title: string): Entity {
  return {
    id,
    type: 'procedure',
    // Same owner on both: a three-way merge sees "their value equals our
    // base" and would write, which is how the edit reached the other row.
    properties: { title, owner: 'Kim' },
    _actions: { update: true },
    _fields: {},
    _redacted: [],
  }
}

const mounted: VueWrapper[] = []

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
  updateGuards.splice(0)
  vi.useRealTimers()
})

async function mountEdit() {
  const schema = useSchemaStore()
  schema.forms.set('procedure-form', FORM as never)
  schema.entityTypes.set('procedure', ENTITY_TYPE as never)
  schema.loaded = true

  const rows: Record<string, Entity> = {
    'PRC-X': procedure('PRC-X', 'Procedure X'),
    'PRC-Y': procedure('PRC-Y', 'Procedure Y'),
  }
  const entities = useEntitiesStore()
  vi.spyOn(entities, 'fetchEntity').mockImplementation(async (_t, id) => rows[id as string]!)
  const update = vi
    .spyOn(entities, 'update')
    .mockImplementation(
      async (_t, id) => ({ ...rows[id as string]!, warnings: [] }) as Entity
    ) as unknown as Mock

  const wrapper = mount(FormView, {
    props: { id: 'procedure-form', entityId: 'PRC-X' },
    global: {
      stubs: {
        RouterLink: true,
        MarkdownEditor: true,
        RelationPicker: true,
        RelationCards: true,
        AutoSaveIndicator: true,
        SidePanel: true,
        HelpModal: true,
      },
    },
  })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, update }
}

// What the router does on `/form/procedure-form/PRC-X` → `.../PRC-Y`. The
// wrapper type loses FormView's props, hence the cast.
const switchTo = (w: VueWrapper, entityId: string) =>
  w.setProps({ entityId } as Record<string, unknown>)

const route = (entityId: string, query: Record<string, string> = {}) =>
  ({ params: { id: 'procedure-form', entityId }, query }) as unknown as RouteLocationNormalized

// The guard the form on screen registered, run the way the router runs it.
const runUpdateGuard = (to: RouteLocationNormalized, from: RouteLocationNormalized) =>
  updateGuards[updateGuards.length - 1]!(to, from)

const fieldValue = (w: VueWrapper, p: string) =>
  (w.find(`#field-${p}`).element as HTMLInputElement).value

const targets = (update: Mock) => update.mock.calls.map((c) => c[1])

describe('DynamicForm entity switch (BUG-NSGD6OE)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  it('saves an edit made on X to X before the route switches to Y', async () => {
    const { wrapper, update } = await mountEdit()
    vi.useFakeTimers()

    await wrapper.find('#field-owner').setValue('Sam')
    await expect(runUpdateGuard(route('PRC-Y'), route('PRC-X'))).resolves.toBe(true)
    expect(targets(update)).toEqual(['PRC-X'])
    expect(update.mock.calls[0]![2]).toMatchObject({ properties: { owner: 'Sam' } })

    await switchTo(wrapper, 'PRC-Y')
    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()
    expect(targets(update)).toEqual(['PRC-X'])
  })

  // Without the guard (a navigation the guard did not see), the pending edit
  // still fires, and still goes to the entity it was made on.
  it('never writes an edit made on X to Y when the switch skips the guard', async () => {
    const { wrapper, update } = await mountEdit()
    vi.useFakeTimers()

    await wrapper.find('#field-owner').setValue('Sam')
    await switchTo(wrapper, 'PRC-Y')
    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()

    expect(targets(update)).toEqual(['PRC-X'])
  })

  it('lets a query-only change through without saving', async () => {
    const { wrapper, update } = await mountEdit()
    vi.useFakeTimers()

    await wrapper.find('#field-owner').setValue('Sam')
    expect(runUpdateGuard(route('PRC-X', { step: '2' }), route('PRC-X'))).toBe(true)
    expect(update).not.toHaveBeenCalled()
  })

  it('shows the new entity after the route switches', async () => {
    const { wrapper } = await mountEdit()
    expect(fieldValue(wrapper, 'title')).toBe('Procedure X')

    await switchTo(wrapper, 'PRC-Y')
    await flushPromises()

    expect(fieldValue(wrapper, 'title')).toBe('Procedure Y')
  })

  it('writes an edit made after the switch to Y with Y as the base', async () => {
    const { wrapper, update } = await mountEdit()
    await switchTo(wrapper, 'PRC-Y')
    await flushPromises()
    // Asserted before the edit: the save response for Y is merged into the
    // form and would overwrite a stale title, hiding that X was still loaded.
    expect(fieldValue(wrapper, 'title')).toBe('Procedure Y')
    vi.useFakeTimers()

    await wrapper.find('#field-owner').setValue('Sam')
    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()

    expect(targets(update)).toEqual(['PRC-Y'])
    expect(fieldValue(wrapper, 'title')).toBe('Procedure Y')
  })
})
