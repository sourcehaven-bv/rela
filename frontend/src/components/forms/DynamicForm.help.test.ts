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
}))
vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, getTemplates: vi.fn().mockResolvedValue([]) }
})

const ENTITY_TYPE = {
  name: 'ticket',
  label: 'Ticket',
  id_type: 'short',
  properties: { title: { type: 'string' }, notes: { type: 'string' } },
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
      fields: [{ property: 'notes', label: 'Notes' }],
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
    properties: { title: 'x', notes: 'y' },
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

  it('omits the details block on a step without details', async () => {
    mockRoute.query = { step: '1' }
    const w = await mountForm()
    expect(w.find('[data-testid="step-details"]').exists()).toBe(false)
    expect(w.find('[data-testid="step-description"]').exists()).toBe(false)
  })
})
