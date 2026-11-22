// Form-level and step-level help texts: the form `description`, a step's
// `description` (markdown) and a step's collapsed `details`.
//
// The form `description` was documented as "help text shown below the
// heading" but never rendered; these pin that it now is.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import type { Entity } from '@/types'

const mockRoute = vi.hoisted(() => ({
  query: {} as Record<string, string>,
  params: {},
  path: '/form/help-form',
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => mockRoute,
  onBeforeRouteLeave: vi.fn(),
  onBeforeRouteUpdate: vi.fn(),
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
}))
vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, getTemplates: vi.fn().mockResolvedValue([]) }
})

const ENTITY_TYPE = {
  name: 'ticket',
  label: 'Ticket',
  id_type: 'short',
  properties: { title: { type: 'string' }, notes: { type: 'string' }, extra: { type: 'string' } },
}

const FORM = {
  id: 'help-form',
  entity: 'ticket',
  description: 'Fill in **every** register field.',
  steps: [
    {
      title: 'Name',
      description: 'See [the guide](/entity/guide/G-1).',
      details: '- first point\n- second point',
      fields: [{ property: 'title', label: 'Title' }],
    },
    {
      title: 'Notes',
      details: 'Notes guidance.',
      fields: [{ property: 'notes', label: 'Notes' }],
    },
    {
      title: 'Extra',
      fields: [{ property: 'extra', label: 'Extra' }],
    },
  ],
}

const mounted: VueWrapper[] = []
afterEach(() => {
  mounted.splice(0).forEach((w) => {
    try {
      w.unmount()
    } catch {
      /* already torn down */
    }
  })
})

async function mountForm(): Promise<VueWrapper> {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('ticket', ENTITY_TYPE as never)
  schema.loaded = true
  const entity: Entity = {
    id: 'TKT-001',
    type: 'ticket',
    properties: { title: 'x', notes: 'y', extra: 'z' },
    _actions: { update: true },
    _fields: {},
    _redacted: [],
  }
  vi.spyOn(useEntitiesStore(), 'fetchEntity').mockResolvedValue(entity)
  const wrapper = mount(DynamicForm, {
    props: { formId: FORM.id, entityId: 'TKT-001' },
    global: {
      stubs: {
        RouterLink: true,
        MarkdownEditor: true,
        RelationPicker: true,
        RelationCards: true,
        AutoSaveIndicator: true,
        SidePanel: true,
      },
    },
  })
  mounted.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('DynamicForm help texts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
    mockRoute.query = {}
  })

  it('renders the form description as markdown', async () => {
    const w = await mountForm()
    const desc = w.find('[data-testid="form-description"]')
    expect(desc.exists()).toBe(true)
    expect(desc.find('strong').text()).toBe('every')
  })

  it('renders the step description as markdown, with its link', async () => {
    const w = await mountForm()
    const link = w.find('[data-testid="step-description"] a')
    expect(link.attributes('href')).toBe('/entity/guide/G-1')
    expect(link.text()).toBe('the guide')
  })

  it('renders step details collapsed, behind a summary', async () => {
    const w = await mountForm()
    const details = w.find('[data-testid="step-details"]')
    expect(details.element.tagName).toBe('DETAILS')
    expect((details.element as HTMLDetailsElement).open).toBe(false)
    expect(details.find('summary').text()).toBe('More information')
    expect(details.findAll('li').map((li) => li.text())).toEqual(['first point', 'second point'])
  })

  it('closes the details again when the user moves to another step', async () => {
    const w = await mountForm()
    const first = w.find('[data-testid="step-details"]').element as HTMLDetailsElement
    first.open = true
    await w.findAll('.wizard-step-pill')[1].trigger('click')
    await flushPromises()
    const details = w.find('[data-testid="step-details"]')
    expect(details.text()).toContain('Notes guidance.')
    expect((details.element as HTMLDetailsElement).open).toBe(false)
  })

  it('omits the details block on a step without details', async () => {
    mockRoute.query = { step: '2' }
    const w = await mountForm()
    expect(w.find('[data-testid="step-details"]').exists()).toBe(false)
    expect(w.find('[data-testid="step-description"]').exists()).toBe(false)
  })
})

describe('DynamicForm for a form the configuration does not have', () => {
  it('shows the not-found page, not the form id', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useSchemaStore().loaded = true
    const wrapper = mount(DynamicForm, {
      props: { formId: 'nope' },
      global: { plugins: [pinia], stubs: { SidePanel: true } },
    })
    mounted.push(wrapper)
    await flushPromises()
    expect(wrapper.find('[data-testid="not-found-state"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('nope')
  })
})
