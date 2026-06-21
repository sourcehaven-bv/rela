// Unit tests for SectionEditForm — covers per-cell render gating,
// scheduleFieldSave / scheduleUnset routing, owner-identity guard
// on onPropertyApplied, verdict-flip toast via onVerdictFlip,
// and per-field error pill via FieldShell.
//
// Mocks `entitiesStore.update` at the store level so PATCH timing is
// driven by fake timers, mirroring useAutoSave.test.ts.

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { useEntitiesStore } from '@/stores/entities'
import SectionEditForm, { type SectionEditField } from './SectionEditForm.vue'
import { ApiError } from '@/api/errors'
import type { Entity, PropertyDef, AttachmentInfo } from '@/types'

const TEXT_DEF: PropertyDef = { type: 'string' } as PropertyDef
const ENUM_DEF: PropertyDef = { type: 'enum', values: ['open', 'closed'] } as PropertyDef

function makeFields(overrides: Partial<SectionEditField>[] = []): SectionEditField[] {
  const defaults: SectionEditField[] = [
    { property: 'title', label: 'Title', kind: 'schema', propertyDef: TEXT_DEF },
    { property: 'status', label: 'Status', kind: 'schema', propertyDef: ENUM_DEF },
  ]
  return defaults.map((d, i) => ({ ...d, ...(overrides[i] ?? {}) } as SectionEditField))
}

function mountForm(opts: {
  fields?: SectionEditField[]
  initialValues?: Record<string, unknown>
  entityType?: string
  entityId?: string
  attachments?: Record<string, AttachmentInfo[]>
  onPropertyApplied?: Mock
  onError?: Mock
  onVerdictFlip?: Mock
}) {
  const onPropertyApplied = opts.onPropertyApplied ?? vi.fn()
  const onError = opts.onError ?? vi.fn()
  const onVerdictFlip = opts.onVerdictFlip ?? vi.fn()
  const wrapper = mount(SectionEditForm, {
    props: {
      entityType: opts.entityType ?? 'ticket',
      entityId: opts.entityId ?? 'TKT-001',
      initialValues: opts.initialValues ?? { title: 'Original', status: 'open' },
      fields: opts.fields ?? makeFields(),
      attachments: opts.attachments,
      onPropertyApplied,
      onError,
      onVerdictFlip,
    },
  })
  return { wrapper, onPropertyApplied, onError, onVerdictFlip }
}

function makeStoreMock() {
  const store = useEntitiesStore()
  const updateMock = vi.spyOn(store, 'update').mockResolvedValue({
    id: 'TKT-001',
    type: 'ticket',
    properties: {},
  } as Entity) as unknown as Mock
  return updateMock
}

describe('SectionEditForm', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('renders one row per field; writable cells wrap widget in FieldShell, non-writable do not', () => {
    const fields = makeFields([
      { verdict: { writable: true } },
      { verdict: { writable: false } },
    ])
    makeStoreMock()
    const { wrapper } = mountForm({ fields })
    const items = wrapper.findAll('.property-item')
    expect(items).toHaveLength(2)
    // Writable cell has a .form-field (FieldShell's root class); non-writable does not.
    expect(items[0].find('.form-field').exists()).toBe(true)
    expect(items[1].find('.form-field').exists()).toBe(false)
  })

  it('scheduleFieldSave fires on update:modelValue from a writable widget', async () => {
    const fields = makeFields([{ verdict: { writable: true } }])
    const updateMock = makeStoreMock()
    const { wrapper } = mountForm({ fields: [fields[0]] })
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    expect(widget.exists()).toBe(true)
    widget.vm.$emit('update:modelValue', 'New Title')
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()
    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock.mock.calls[0][2]).toEqual({ properties: { title: 'New Title' } })
  })

  it('cleared text widget routes to scheduleUnset (not scheduleFieldSave)', async () => {
    const fields = makeFields([{ verdict: { writable: true } }])
    const updateMock = makeStoreMock()
    const { wrapper } = mountForm({
      fields: [fields[0]],
      initialValues: { title: 'Original' },
    })
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    widget.vm.$emit('update:modelValue', '')
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()
    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock.mock.calls[0][2]).toEqual({ properties_unset: ['title'] })
  })

  it('applyServerProperty deletes the local key when value is undefined', async () => {
    const fields = makeFields([{ verdict: { writable: true } }])
    const onPropertyApplied = vi.fn()
    const store = useEntitiesStore()
    vi.spyOn(store, 'update').mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: {}, // server-side unset
    } as Entity)
    const { wrapper } = mountForm({
      fields: [fields[0]],
      initialValues: { title: 'Original' },
      onPropertyApplied,
    })
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    widget.vm.$emit('update:modelValue', '')
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()
    // The PATCH unsets, the server response has properties: {}, so the
    // disappeared-key path in mergeServerResponse invokes
    // applyServerProperty(prop, undefined) — onPropertyApplied likewise.
    const undefinedCall = onPropertyApplied.mock.calls.find((c) => c[1] === undefined)
    expect(undefinedCall).toBeDefined()
    expect(undefinedCall?.[2]).toEqual({ type: 'ticket', id: 'TKT-001' })
  })

  it('onPropertyApplied receives owner identity { type, id } frozen at mount', async () => {
    const fields = makeFields([{ verdict: { writable: true } }])
    const onPropertyApplied = vi.fn()
    const store = useEntitiesStore()
    vi.spyOn(store, 'update').mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: { title: 'Server Title' },
    } as Entity)
    const { wrapper } = mountForm({
      fields: [fields[0]],
      entityType: 'ticket',
      entityId: 'TKT-001',
      onPropertyApplied,
    })
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    widget.vm.$emit('update:modelValue', 'New')
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()
    const titleCall = onPropertyApplied.mock.calls.find((c) => c[0] === 'title')
    expect(titleCall?.[2]).toEqual({ type: 'ticket', id: 'TKT-001' })
  })

  it('onPropertyApplied throw is caught; formData stays at server value', async () => {
    const fields = makeFields([{ verdict: { writable: true } }])
    const onPropertyApplied = vi.fn().mockImplementation(() => {
      throw new Error('host bug')
    })
    const store = useEntitiesStore()
    vi.spyOn(store, 'update').mockResolvedValue({
      id: 'TKT-001',
      type: 'ticket',
      properties: { title: 'Server Title' },
    } as Entity)
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const { wrapper, onError } = mountForm({
      fields: [fields[0]],
      onPropertyApplied,
    })
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    widget.vm.$emit('update:modelValue', 'New')
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()
    expect(onPropertyApplied).toHaveBeenCalled()
    expect(errSpy).toHaveBeenCalled()
    // onError is NOT invoked from the throw path (RR-UE3D semantics).
    expect(onError).not.toHaveBeenCalled()
    errSpy.mockRestore()
  })

  it('verdict flip true → false drops pending edit, fires onVerdictFlip (not onError)', async () => {
    const initial = makeFields([{ verdict: { writable: true } }])
    const updateMock = makeStoreMock()
    const { wrapper, onError, onVerdictFlip } = mountForm({ fields: [initial[0]] })
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    widget.vm.$emit('update:modelValue', 'pending edit')
    // Don't advance timers — keep the edit pending.
    await nextTick()
    // Flip the verdict.
    const flipped = makeFields([{ verdict: { writable: false } }])
    await wrapper.setProps({ fields: [flipped[0]] })
    await nextTick()
    // Advance past the debounce window — the pending edit should be gone.
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()
    expect(updateMock).not.toHaveBeenCalled()
    expect(onVerdictFlip).toHaveBeenCalledWith('title', 'Title')
    expect(onError).not.toHaveBeenCalled()
  })

  it('verdict flip false → true is silent', async () => {
    const initial = makeFields([{ verdict: { writable: false } }])
    makeStoreMock()
    const { wrapper, onVerdictFlip } = mountForm({ fields: [initial[0]] })
    const restored = makeFields([{ verdict: { writable: true } }])
    await wrapper.setProps({ fields: [restored[0]] })
    await nextTick()
    expect(onVerdictFlip).not.toHaveBeenCalled()
  })

  it('commitImmediately runs on unmount', async () => {
    const fields = makeFields([{ verdict: { writable: true } }])
    const updateMock = makeStoreMock()
    const { wrapper } = mountForm({ fields: [fields[0]] })
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    widget.vm.$emit('update:modelValue', 'pending')
    // Don't advance timers; unmount immediately.
    wrapper.unmount()
    await vi.runAllTimersAsync()
    await flushPromises()
    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock.mock.calls[0][2]).toEqual({ properties: { title: 'pending' } })
  })

  it('per-field error pill renders inside FieldShell on 422 server response', async () => {
    const fields = makeFields([{ verdict: { writable: true } }])
    const store = useEntitiesStore()
    vi.spyOn(store, 'update').mockRejectedValueOnce(
      new ApiError('invalid value', { kind: 'http', status: 422, original: null }),
    )
    const { wrapper } = mountForm({ fields: [fields[0]] })
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    widget.vm.$emit('update:modelValue', 'bad')
    await vi.advanceTimersByTimeAsync(900)
    await vi.runOnlyPendingTimersAsync()
    await flushPromises()
    const errorPill = wrapper.find('.field-error')
    expect(errorPill.exists()).toBe(true)
    expect(errorPill.text()).toBe('invalid value')
  })

  // Regression: the inline-edit display path must forward _attachments to
  // the file widget. Previously SectionEditForm dropped the attachment, so
  // a `file` property on a writable entity showed no preview even though
  // the entity GET / view payload carried _attachments.
  it('forwards attachment metadata to the file widget for a file property', () => {
    const FILE_DEF: PropertyDef = { type: 'file' } as PropertyDef
    const fields: SectionEditField[] = [
      { property: 'photo', label: 'Photo', kind: 'schema', propertyDef: FILE_DEF, verdict: { writable: true } },
    ]
    const att: AttachmentInfo = {
      id: 'shot.png',
      filename: 'shot.png',
      size: 2048,
      contentType: 'image/png',
      href: '/api/v1/tickets/TKT-001/_attachments/photo/shot.png',
    }
    makeStoreMock()
    const { wrapper } = mountForm({
      fields,
      initialValues: { photo: 'attachments/TKT-001/photo/shot.png' },
      attachments: { photo: [att] },
    })
    const widget = wrapper.findComponent({ name: 'FileWidget' })
    expect(widget.exists()).toBe(true)
    expect(widget.props('attachments')).toEqual([att])
    expect(widget.props('entityType')).toBe('ticket')
    expect(widget.props('entityId')).toBe('TKT-001')
    // The preview renders from the forwarded metadata.
    expect(wrapper.find('img.file-preview').exists()).toBe(true)
  })
})
