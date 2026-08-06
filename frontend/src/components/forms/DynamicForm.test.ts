// Unit tests for DynamicForm's edit-mode affordance filter (BUG-MLT9DE,
// DEC-T0XIWQ).
//
// Why this file exists: before it, DynamicForm had no mounting test at all
// (DynamicForm.guard.test.ts deliberately replicates its guard in a stub), and
// every fixture elsewhere seeded entities whose `properties` already carried a
// key for each field under test. The edit-mode gate `f.property in
// formData.value` therefore always matched, and its failure mode — a property
// that is CONFIGURED but simply UNSET never rendering, and so never becoming
// settable — was invisible to CI.
//
// The three things pinned here:
//   1. A configured-but-unset property renders and saves. (The bug.)
//   2. A server-redacted property does not render, and — critically — is not
//      submitted as empty when untouched, which would clobber the hidden
//      stored value.
//   3. Both hold on the wizard path, which carries the same gate.

import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import type { Entity } from '@/types'

// DynamicForm and its composables (useFormWizard) call useRoute/useRouter
// directly, so `global.mocks` doesn't reach them — mock the module instead.
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/form/ticket-form' }),
  onBeforeRouteLeave: vi.fn(),
}))

// `notes` is the property at the heart of the bug: declared in the metamodel,
// listed in the form config, but absent from the stored entity because it was
// added to the schema after this entity was created.
const ENTITY_TYPE = {
  name: 'ticket',
  label: 'Ticket',
  id_type: 'short',
  properties: {
    title: { type: 'string' },
    notes: { type: 'string' },
    salary: { type: 'string' },
  },
}

const FLAT_FORM = {
  id: 'ticket-form',
  entity: 'ticket',
  fields: [
    { property: 'title', label: 'Title' },
    { property: 'notes', label: 'Notes' },
    { property: 'salary', label: 'Salary' },
  ],
}

const WIZARD_FORM = {
  id: 'ticket-wizard',
  entity: 'ticket',
  steps: [
    {
      id: 'step1',
      title: 'Step 1',
      fields: [
        { property: 'title', label: 'Title' },
        { property: 'notes', label: 'Notes' },
        { property: 'salary', label: 'Salary' },
      ],
    },
  ],
}

// Seed the real store state rather than stubbing its getters: `getForm` /
// `getEntityType` are computeds returning lookup fns over these Maps, so
// populating the Maps exercises the same resolution path the app uses.
function stubStores(form: object) {
  const schema = useSchemaStore()
  schema.forms.set((form as { id: string }).id, form as never)
  schema.entityTypes.set('ticket', ENTITY_TYPE as never)
  schema.loaded = true
  return schema
}

// Serve an entity the way the real API does after ACL redaction: hidden and
// never-set properties are BOTH simply absent from `properties`. `_redacted`
// is the only thing that tells them apart — which is the whole point.
function stubEntity(opts: { properties: Record<string, unknown>; redacted?: string[] }) {
  const entities = useEntitiesStore()
  const entity: Entity = {
    id: 'TKT-001',
    type: 'ticket',
    properties: opts.properties,
    _actions: { update: true },
    _fields: {}, // permissive default — the sparse "nothing deviates" signal
    _redacted: opts.redacted ?? [],
  }
  vi.spyOn(entities, 'fetchEntity').mockResolvedValue(entity)
  const update = vi
    .spyOn(entities, 'update')
    .mockResolvedValue({ ...entity, warnings: [] } as Entity) as unknown as Mock
  return { update }
}

async function mountEdit(form: object, entityOpts: Parameters<typeof stubEntity>[0]) {
  stubStores(form)
  const { update } = stubEntity(entityOpts)
  const wrapper = mount(DynamicForm, {
    props: { formId: (form as { id: string }).id, entityId: 'TKT-001' },
    global: {
      stubs: {
        RouterLink: true,
        MarkdownEditor: true,
        RelationPicker: true,
        RelationCards: true,
        AutoSaveIndicator: true,
      },
      mocks: {
        $router: { push: vi.fn(), replace: vi.fn() },
        $route: { query: {}, params: {}, path: '/form/ticket-form' },
      },
    },
  })
  await flushPromises()
  return { wrapper, update }
}

// FieldRenderer gives each rendered widget `id="field-<property>"`
// (FieldRenderer.vue:41), so presence of that id is exactly "this property
// rendered an input the user can reach".
function renderedProperties(wrapper: { find: (s: string) => { exists: () => boolean } }) {
  return ['title', 'notes', 'salary'].filter((p) => wrapper.find(`#field-${p}`).exists())
}

describe('DynamicForm edit-mode affordance filter', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  describe('flat form', () => {
    // THE BUG. `notes` is configured on the form and declared in the
    // metamodel, but the entity predates it, so the stored properties omit it
    // and `_fields` (sparse) says nothing about it. It must still render —
    // otherwise it can never be filled in, and never being filled in is
    // exactly why it is missing.
    it('renders a configured property the entity has never had', async () => {
      const { wrapper } = await mountEdit(FLAT_FORM, {
        properties: { title: 'Original' },
      })
      expect(renderedProperties(wrapper)).toContain('notes')
    })

    it('renders every configured property when nothing is redacted', async () => {
      const { wrapper } = await mountEdit(FLAT_FORM, {
        properties: { title: 'Original' },
        redacted: [],
      })
      expect(renderedProperties(wrapper)).toEqual(['title', 'notes', 'salary'])
    })

    // The inverse: absence alone must never hide, but a POSITIVE redaction
    // verdict still must. Losing this would trade a silent-hiding bug for a
    // silent-exposure one.
    it('hides a property the server reports as redacted', async () => {
      const { wrapper } = await mountEdit(FLAT_FORM, {
        properties: { title: 'Original' },
        redacted: ['salary'],
      })
      const rendered = renderedProperties(wrapper)
      expect(rendered).not.toContain('salary')
      expect(rendered).toContain('notes') // unset ≠ redacted
    })
  })

  // The data-loss question the fix raises: now that previously-hidden fields
  // render, can an untouched one overwrite a redacted stored value with empty?
  //
  // It cannot, and these pin why. Edit mode has NO bulk submit — handleSubmit
  // returns early when isEdit (DynamicForm.vue), and every write goes through
  // per-property autosave, which only fires for a property the user actually
  // typed into. So an untouched field is never in a payload at all, and a
  // redacted field additionally never renders to be typed into.
  describe('edit-mode writes are per-property, never a bulk overwrite', () => {
    it('does not render a redacted field for the user to reach', async () => {
      const { wrapper } = await mountEdit(FLAT_FORM, {
        properties: { title: 'Original' },
        redacted: ['salary'],
      })
      expect(wrapper.find('#field-salary').exists()).toBe(false)
    })

    // A form-level submit in edit mode must be inert: if it ever started
    // sending formData wholesale, every unset field would post as empty and
    // every redacted one would clobber its hidden value.
    it('sends nothing on a form submit (no bulk PATCH in edit mode)', async () => {
      const { wrapper, update } = await mountEdit(FLAT_FORM, {
        properties: { title: 'Original' },
        redacted: ['salary'],
      })
      await wrapper.find('form').trigger('submit')
      await flushPromises()
      expect(update).not.toHaveBeenCalled()
    })
  })

  describe('wizard form', () => {
    // The wizard path (visibleStepFields → affordanceVisible) is a second
    // consumer of the same predicate. Before the fix it held a hand-synced
    // copy of the gate and carried the identical defect.
    it('renders a configured-but-unset property inside a step', async () => {
      const { wrapper } = await mountEdit(WIZARD_FORM, {
        properties: { title: 'Original' },
      })
      expect(renderedProperties(wrapper)).toContain('notes')
    })

    it('hides a redacted property inside a step', async () => {
      const { wrapper } = await mountEdit(WIZARD_FORM, {
        properties: { title: 'Original' },
        redacted: ['salary'],
      })
      expect(renderedProperties(wrapper)).not.toContain('salary')
    })
  })
})
