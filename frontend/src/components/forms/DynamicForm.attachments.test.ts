// Create-mode staged-attachment tests for DynamicForm (TKT-7K3BJF).
//
// Attaching a file while creating an entity is necessarily TWO requests: an
// attachment cannot be written before the entity row exists (every store
// backend returns ErrNotFound, pinned by storetest) and there is no
// id-reservation primitive, so the only possible order is create-then-attach.
//
// That makes the orchestration — not the widget — where this feature can go
// wrong, which is what this file drives: a real widget pick, through
// FieldRenderer and FormFieldList into DynamicForm's staging map, and out to
// an asserted `uploadAttachment` call after `entitiesStore.create` resolves.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import type { Entity } from '@/types'

const push = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push, replace: vi.fn(), back: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/form/bug-form' }),
  onBeforeRouteLeave: vi.fn(),
}))

// Create mode fetches templates and runs the staged-affordance dry-run on
// mount; both must be stubbed or the form never leaves its loading state.
vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return {
    ...actual,
    getTemplates: vi.fn().mockResolvedValue([]),
    // The real endpoint returns the STRIPPED candidate — every property the
    // caller may see, including ones not yet filled. That echo is what
    // populates `stagedVisibleProps`; echoing only the submitted keys would
    // mark every untouched field policy-hidden and unrender the whole form.
    dryRunCreateEntity: vi.fn().mockImplementation(async (_type: string, body: unknown) => ({
      properties: {
        title: '',
        screenshot: '',
        evidence: [],
        needs_evidence: false,
        ...((body as { properties?: Record<string, unknown> })?.properties ?? {}),
      },
      _fields: {},
      _relations: {},
      warnings: [],
    })),
    createRelation: vi.fn().mockResolvedValue(undefined),
  }
})

// The attachment API is the boundary this feature crosses; MockAttachmentError
// mirrors the real class (carries an HTTP status) so the status branches fire.
const { mockUpload, MockAttachmentError } = vi.hoisted(() => {
  class MockAttachmentError extends Error {
    status: number
    constructor(message: string, status: number) {
      super(message)
      this.status = status
    }
  }
  return { mockUpload: vi.fn().mockResolvedValue({}), MockAttachmentError }
})
vi.mock('@/api/attachments', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api/attachments')
  return {
    ...actual,
    uploadAttachment: mockUpload,
    deleteAttachment: vi.fn().mockResolvedValue(undefined),
    AttachmentError: MockAttachmentError,
  }
})

const ENTITY_TYPE = {
  name: 'bug',
  label: 'Bug',
  id_type: 'sequential',
  properties: {
    title: { type: 'string' },
    screenshot: { type: 'file' },
    evidence: { type: 'file', max: 3 },
    needs_evidence: { type: 'boolean' },
  },
}

const FORM = {
  id: 'bug-form',
  entity: 'bug',
  fields: [
    { property: 'title', label: 'Title' },
    { property: 'screenshot', label: 'Screenshot' },
    { property: 'evidence', label: 'Evidence' },
  ],
}

// A wizard whose second step (holding the file property) is conditional, so a
// revealed-then-hidden branch can be exercised (RR-OI6P51).
const WIZARD_FORM = {
  id: 'bug-wizard',
  entity: 'bug',
  steps: [
    { id: 's1', title: 'Basics', fields: [{ property: 'title' }, { property: 'needs_evidence' }] },
    {
      id: 's2',
      title: 'Evidence',
      visible_when: 'form.needs_evidence == true',
      fields: [{ property: 'screenshot' }],
    },
  ],
}

const CREATED: Entity = {
  id: 'BUG-7',
  type: 'bug',
  properties: { title: 'x' },
  warnings: [],
}

const mounted: VueWrapper[] = []

afterEach(() => {
  const wrappers = mounted.splice(0)
  wrappers.forEach((w) => {
    try {
      w.unmount()
    } catch {
      /* already torn down */
    }
  })
})

async function mountCreate(form: { id: string } = FORM) {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.forms.set(WIZARD_FORM.id, WIZARD_FORM as never)
  schema.entityTypes.set('bug', ENTITY_TYPE as never)
  schema.loaded = true

  const entities = useEntitiesStore()
  const create = vi.spyOn(entities, 'create').mockResolvedValue(CREATED)

  const wrapper = mount(DynamicForm, {
    props: { formId: form.id },
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

function textFile(name: string, type = 'text/plain'): File {
  return new File(['bytes'], name, { type })
}

/** Drive a REAL pick on the file widget for `property`, the way a user does. */
async function stage(wrapper: VueWrapper, property: string, file: File) {
  const input = wrapper.find(`#field-${property} input[type="file"]`)
  Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
  await input.trigger('change')
  await flushPromises()
}

/** Click the wizard step button whose label contains `text`. */
async function clickButton(wrapper: VueWrapper, text: string) {
  const btn = wrapper.findAll('button').find((b) => b.text().includes(text))
  if (!btn) throw new Error(`no button matching ${text}`)
  await btn.trigger('click')
  await flushPromises()
}

async function submit(wrapper: VueWrapper) {
  await wrapper.find('form').trigger('submit')
  await flushPromises()
}

describe('DynamicForm — attachments on create', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockUpload.mockResolvedValue({})
  })

  it('renders a usable file control in create mode', async () => {
    // The regression this ticket fixes: the widget used to render a dead
    // "Attachment editing unavailable" note because there is no entity id.
    const { wrapper } = await mountCreate()
    expect(wrapper.find('#field-screenshot .file-dropzone').exists()).toBe(true)
  })

  it('stages a picked file without uploading, then uploads it after create', async () => {
    const { wrapper, create } = await mountCreate()
    const file = textFile('shot.png', 'image/png')

    await stage(wrapper, 'screenshot', file)
    // Phase 1 has not happened yet, so nothing may be uploaded.
    expect(mockUpload).not.toHaveBeenCalled()

    await submit(wrapper)

    expect(create).toHaveBeenCalledTimes(1)
    // The id must be the SERVER's, never a client-side placeholder.
    expect(mockUpload).toHaveBeenCalledWith('bug', 'BUG-7', 'screenshot', file)
  })

  it('never sends the file property in the create payload', async () => {
    // Staged files live outside formData precisely so a File (or a fake path)
    // is not POSTed as the property's value — the server stamps it itself.
    const { wrapper, create } = await mountCreate()
    await stage(wrapper, 'screenshot', textFile('a.png', 'image/png'))
    await submit(wrapper)

    const payload = create.mock.calls[0][1] as { properties: Record<string, unknown> }
    expect(payload.properties).not.toHaveProperty('screenshot')
  })

  it('does not upload a staged file that was removed before save', async () => {
    const { wrapper } = await mountCreate()
    await stage(wrapper, 'screenshot', textFile('gone.txt'))

    await wrapper.find('#field-screenshot .file-remove').trigger('click')
    await flushPromises()

    await submit(wrapper)
    expect(mockUpload).not.toHaveBeenCalled()
  })

  it('uploads every file of a multi-cap property', async () => {
    const { wrapper } = await mountCreate()
    const a = textFile('a.txt')
    const b = textFile('b.txt')
    await stage(wrapper, 'evidence', a)
    await stage(wrapper, 'evidence', b)

    await submit(wrapper)

    expect(mockUpload).toHaveBeenCalledTimes(2)
    expect(mockUpload).toHaveBeenCalledWith('bug', 'BUG-7', 'evidence', a)
    expect(mockUpload).toHaveBeenCalledWith('bug', 'BUG-7', 'evidence', b)
  })

  it('makes the form dirty while a file is staged (RR-EUX4BX)', async () => {
    // Staged files are deliberately outside formData, so without an explicit
    // term they are invisible to checkDirty and the navigation guards let the
    // user leave with no prompt — silent loss of a file they picked.
    const { wrapper } = await mountCreate()
    const vm = wrapper.vm as unknown as { isDirty: () => boolean }
    expect(vm.isDirty()).toBe(false)

    await stage(wrapper, 'screenshot', textFile('draft.txt'))
    expect(vm.isDirty()).toBe(true)
  })

  describe('post-create upload failure', () => {
    it('surfaces the failure and does not claim success', async () => {
      const ui = useUIStore()
      const error = vi.spyOn(ui, 'error')
      const success = vi.spyOn(ui, 'success')
      mockUpload.mockRejectedValue(new MockAttachmentError('too big', 413))

      const { wrapper } = await mountCreate()
      await stage(wrapper, 'screenshot', textFile('huge.bin'))
      await submit(wrapper)

      expect(error).toHaveBeenCalledTimes(1)
      expect(error.mock.calls[0][0]).toContain('huge.bin')
      // A green "created successfully" next to a red failure would describe
      // one user action twice, contradicting itself.
      expect(success).not.toHaveBeenCalled()
    })

    it('clears staged state rather than pretending a retry is possible (RR-4QO887)', async () => {
      // The form unmounts on the navigation below, so retained File objects
      // would be unreachable anyway — and a still-dirty form whose submit
      // re-runs the whole create path turns "retry" into a second entity.
      mockUpload.mockRejectedValue(new MockAttachmentError('nope', 403))
      const { wrapper } = await mountCreate()
      await stage(wrapper, 'screenshot', textFile('secret.txt'))
      await submit(wrapper)

      expect((wrapper.vm as unknown as { isDirty: () => boolean }).isDirty()).toBe(false)
    })

    it('names where to re-attach, since the staged copies are gone', async () => {
      const ui = useUIStore()
      const error = vi.spyOn(ui, 'error')
      mockUpload.mockRejectedValue(new MockAttachmentError('too big', 413))

      const { wrapper } = await mountCreate()
      await stage(wrapper, 'screenshot', textFile('huge.bin'))
      await submit(wrapper)

      const msg = error.mock.calls[0][0] as string
      expect(msg).toContain('huge.bin')
      expect(msg).toContain('too large')
      expect(msg).toContain('Re-attach on the entity')
    })

    it('a second submit after a failure does not create a second entity', async () => {
      mockUpload.mockRejectedValue(new MockAttachmentError('nope', 413))
      const { wrapper, create } = await mountCreate()
      await stage(wrapper, 'screenshot', textFile('huge.bin'))
      await submit(wrapper)
      await submit(wrapper)

      expect(create).toHaveBeenCalledTimes(1)
    })

    it('still navigates to the created entity — it exists', async () => {
      mockUpload.mockRejectedValue(new MockAttachmentError('nope', 413))
      const { wrapper } = await mountCreate()
      await stage(wrapper, 'screenshot', textFile('huge.bin'))
      await submit(wrapper)

      expect(push).toHaveBeenCalledWith('/entity/bug/BUG-7')
    })

    it('attempts every file and names each failure (RR-Z7C3CY)', async () => {
      // Continue-on-error: aborting after the first rejection would leave
      // later files unattempted with nothing telling the user.
      const ui = useUIStore()
      const error = vi.spyOn(ui, 'error')
      mockUpload
        .mockResolvedValueOnce({})
        .mockRejectedValueOnce(new MockAttachmentError('too big', 413))
        .mockRejectedValueOnce(new MockAttachmentError('bad type', 422))

      const { wrapper } = await mountCreate()
      await stage(wrapper, 'evidence', textFile('ok.txt'))
      await stage(wrapper, 'evidence', textFile('big.txt'))
      await stage(wrapper, 'evidence', textFile('weird.exe'))
      await submit(wrapper)

      expect(mockUpload).toHaveBeenCalledTimes(3)
      const msg = error.mock.calls[0][0] as string
      expect(msg).toContain('big.txt')
      expect(msg).toContain('weird.exe')
      expect(msg).not.toContain('ok.txt')
    })
  })

  it('ignores a second submit while one is in flight (RR-HJLLUF)', async () => {
    // handleKeydown calls handleSubmit() directly, bypassing PendingButton's
    // own guard, and the in-flight window now spans the create POST plus N
    // uploads. Without the re-entrancy guard this yields two entities and two
    // sets of uploads.
    const { wrapper, create } = await mountCreate()
    await stage(wrapper, 'screenshot', textFile('shot.png', 'image/png'))

    const form = wrapper.find('form')
    // Fire both before awaiting either, so the second lands mid-flight.
    const first = form.trigger('submit')
    const second = form.trigger('submit')
    await Promise.all([first, second])
    await flushPromises()

    expect(create).toHaveBeenCalledTimes(1)
    expect(mockUpload).toHaveBeenCalledTimes(1)
  })

  it('does not navigate when the component unmounted mid-upload (RR-QQQ2JR)', async () => {
    let release: (v: unknown) => void = () => {}
    mockUpload.mockImplementation(() => new Promise((r) => (release = r)))

    const { wrapper } = await mountCreate()
    await stage(wrapper, 'screenshot', textFile('slow.bin'))
    const submitted = wrapper.find('form').trigger('submit')
    await flushPromises()

    wrapper.unmount()
    release({})
    await submitted
    await flushPromises()

    expect(push).not.toHaveBeenCalled()
  })

  it('does not upload a file staged under a then-hidden wizard branch (RR-OI6P51)', async () => {
    // The property itself is pruned by pruneWizardHidden; the staged file must
    // be pruned by the same rule, or the bytes land, the server stamps the
    // property, and the hidden branch persists anyway (TKT-CHLAJ).
    //
    // Driven through the wizard's own step navigation: step 2 only renders
    // once its condition is true, so revealing it is what makes the file
    // control reachable at all.
    const { wrapper, create } = await mountCreate(WIZARD_FORM)

    const toggle = wrapper.find('#field-needs_evidence')
    await toggle.setValue(true)
    await flushPromises()

    // Advance to the now-visible evidence step and stage a file there.
    await clickButton(wrapper, 'Next step')
    await stage(wrapper, 'screenshot', textFile('leak.png', 'image/png'))

    // Go back and hide the branch again.
    await clickButton(wrapper, 'Prev step')
    await wrapper.find('#field-needs_evidence').setValue(false)
    await flushPromises()

    await submit(wrapper)

    expect(create).toHaveBeenCalledTimes(1)
    expect(mockUpload).not.toHaveBeenCalled()
  })

  it('leaves the no-attachment create path untouched', async () => {
    // By far the most common case: it must not gain a request or a toast.
    const ui = useUIStore()
    const success = vi.spyOn(ui, 'success')
    const { wrapper, create } = await mountCreate()

    await submit(wrapper)

    expect(create).toHaveBeenCalledTimes(1)
    expect(mockUpload).not.toHaveBeenCalled()
    expect(success).toHaveBeenCalledWith('Entity created successfully')
  })
})
