// Template dropdown tests for DynamicForm.
//
// The create form lists template variants in a labelled dropdown rather than
// a pill row, which overflowed once a type had many variants.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import { getTemplates } from '@/api'
import type { Entity } from '@/types'

const routeQuery: { value: Record<string, string> } = { value: {} }
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  useRoute: () => ({ query: routeQuery.value, params: {}, path: '/form/ticket-form' }),
  onBeforeRouteLeave: vi.fn(),
  onBeforeRouteUpdate: vi.fn(),
}))

// `DynamicForm` destructures `confirm` at setup, so the module has to be
// mocked before mount.
const confirmMock = vi.fn(async () => true)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmMock }),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return {
    ...actual,
    getTemplates: vi.fn().mockResolvedValue([]),
    // Echo onto the full key set, as the real dry-run does: echoing only the
    // submitted keys would mark the untouched title policy-hidden.
    dryRunCreateEntity: vi.fn().mockImplementation(async (_t: string, body: unknown) => ({
      properties: {
        title: '',
        ...((body as { properties?: Record<string, unknown> })?.properties ?? {}),
      },
      _fields: {},
      _relations: {},
      warnings: [],
    })),
    createRelation: vi.fn().mockResolvedValue(undefined),
  }
})

const FORM = {
  id: 'ticket-form',
  entity: 'ticket',
  fields: [{ property: 'title', label: 'Title' }],
}

const TEMPLATES = [
  { name: '', properties: {}, content: 'default body', relations: [] },
  { name: 'corrective_action', properties: {}, content: 'corrective body', relations: [] },
  { name: 'follow-up', properties: {}, content: 'follow-up body', relations: [] },
]

const CREATED: Entity = { id: 'TKT-9', type: 'ticket', properties: {}, warnings: [] }

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

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  routeQuery.value = {}
})

async function mountCreate(
  templates: unknown[],
  props: { embedded?: boolean; embeddedTemplate?: string } = {}
) {
  vi.mocked(getTemplates).mockResolvedValueOnce(templates as never)
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('ticket', {
    name: 'ticket',
    label: 'Ticket',
    id_type: 'short',
    properties: { title: { type: 'string' } },
  } as never)
  schema.loaded = true
  const create = vi.spyOn(useEntitiesStore(), 'create').mockResolvedValue(CREATED)

  const wrapper = mount(DynamicForm, {
    props: { formId: FORM.id, ...props },
    global: {
      stubs: {
        RouterLink: true,
        MarkdownEditor: true,
        RelationPicker: true,
        RelationCards: true,
        AutoSaveIndicator: true,
        HelpModal: true,
      },
    },
  })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, create }
}

function templateSelect(wrapper: VueWrapper) {
  return wrapper.find('.template-selector select')
}

function selectedValue(wrapper: VueWrapper) {
  return (templateSelect(wrapper).element as HTMLSelectElement).value
}

async function submittedContent(wrapper: VueWrapper, create: ReturnType<typeof vi.spyOn>) {
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  return (create.mock.calls[0]?.[1] as { content?: string }).content
}

describe('DynamicForm — template dropdown', () => {
  it('is absent when the type has a single template', async () => {
    const { wrapper } = await mountCreate([TEMPLATES[0]])

    expect(wrapper.find('.template-selector').exists()).toBe(false)
  })

  it('is labelled "Template" and shows - and _ as spaces', async () => {
    const { wrapper } = await mountCreate(TEMPLATES)

    expect(wrapper.find('.template-selector label').text()).toContain('Template')
    expect(
      templateSelect(wrapper)
        .findAll('option')
        .map((o) => o.text())
    ).toEqual(['Default', 'Corrective action', 'Follow up'])
    expect(selectedValue(wrapper)).toBe(TEMPLATES[0].name)
  })

  it('shows a name of separators only as written', async () => {
    const { wrapper } = await mountCreate([TEMPLATES[0], { ...TEMPLATES[1], name: '--' }])

    expect(
      templateSelect(wrapper)
        .findAll('option')
        .map((o) => o.text())
    ).toEqual(['Default', '--'])
  })

  it('preselects the variant requested by the embedding host', async () => {
    const { wrapper } = await mountCreate(TEMPLATES, {
      embedded: true,
      embeddedTemplate: 'follow-up',
    })

    expect(selectedValue(wrapper)).toBe('follow-up')
  })

  it('preselects the variant requested by ?template=', async () => {
    routeQuery.value = { template: 'corrective_action' }
    const { wrapper, create } = await mountCreate(TEMPLATES)

    expect(selectedValue(wrapper)).toBe('corrective_action')
    expect(await submittedContent(wrapper, create)).toBe('corrective body')
  })

  it('falls back to the first template for an unknown request', async () => {
    routeQuery.value = { template: 'deleted-variant' }
    const { wrapper } = await mountCreate(TEMPLATES)

    expect(selectedValue(wrapper)).toBe(TEMPLATES[0].name)
  })

  it('applies a chosen variant without asking when nothing was entered', async () => {
    const { wrapper, create } = await mountCreate(TEMPLATES)

    await templateSelect(wrapper).setValue('corrective_action')
    await flushPromises()

    expect(confirmMock).not.toHaveBeenCalled()
    expect(await submittedContent(wrapper, create)).toBe('corrective body')
  })

  it('asks before replacing entered values, and keeps them on cancel', async () => {
    confirmMock.mockResolvedValueOnce(false)
    const { wrapper, create } = await mountCreate(TEMPLATES)

    await wrapper.find('#field-title').setValue('my title')
    await templateSelect(wrapper).setValue('corrective_action')
    await flushPromises()

    expect(confirmMock).toHaveBeenCalledTimes(1)
    expect(selectedValue(wrapper)).toBe(TEMPLATES[0].name)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const payload = create.mock.calls[0]?.[1] as {
      content?: string
      properties?: Record<string, unknown>
    }
    expect(payload.properties?.title).toBe('my title')
    expect(payload.content).toBe('default body')
  })

  it('replaces entered values once the switch is confirmed', async () => {
    const { wrapper, create } = await mountCreate(TEMPLATES)

    await wrapper.find('#field-title').setValue('my title')
    await templateSelect(wrapper).setValue('follow-up')
    await flushPromises()

    expect(confirmMock).toHaveBeenCalledTimes(1)
    expect(selectedValue(wrapper)).toBe('follow-up')
    expect((wrapper.find('#field-title').element as HTMLInputElement).value).toBe('')
    expect(await submittedContent(wrapper, create)).toBe('follow-up body')
  })
})
