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

// Fields default to `render: 'input'` here so the existing suites keep
// exercising the edit path they were written for. `render` defaults to
// 'display' in production (TKT-HOIX1); an override can set it back to test the
// display arm.
function makeFields(overrides: Partial<SectionEditField>[] = []): SectionEditField[] {
  const defaults: SectionEditField[] = [
    { property: 'title', label: 'Title', kind: 'schema', propertyDef: TEXT_DEF, render: 'input' },
    { property: 'status', label: 'Status', kind: 'schema', propertyDef: ENUM_DEF, render: 'input' },
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
  heading?: string
}) {
  const onPropertyApplied = opts.onPropertyApplied ?? vi.fn()
  const onError = opts.onError ?? vi.fn()
  const onVerdictFlip = opts.onVerdictFlip ?? vi.fn()
  const wrapper = mount(SectionEditForm, {
    props: {
      heading: opts.heading,
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

  // ── render: input | display (TKT-HOIX1) ──────────────────────────────

  it('render: display renders the display arm, not a FieldShell input (AC 1)', () => {
    const fields = makeFields([{ render: 'display', verdict: { writable: true } }])
    const { wrapper } = mountForm({ fields: [fields[0]] })
    // No form chrome: the display arm is a bare widget in mode="display".
    expect(wrapper.find('.form-field').exists()).toBe(false)
    const widget = wrapper.findComponent({ name: 'TextWidget' })
    expect(widget.exists()).toBe(true)
    expect(widget.props('mode')).toBe('display')
  })

  it('render: input on an ACL-read-only field still renders display (AC 3)', () => {
    // SECURITY-CRITICAL. Config downgrades editability; it must never
    // upgrade a read-only field into an editable input.
    const fields = makeFields([{ render: 'input', verdict: { writable: false } }])
    const { wrapper } = mountForm({ fields: [fields[0]] })
    expect(wrapper.find('.form-field').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'TextWidget' }).props('mode')).toBe('display')
  })

  it('render: input with a writable verdict renders the edit arm (AC 2)', () => {
    const fields = makeFields([{ render: 'input', verdict: { writable: true } }])
    const { wrapper } = mountForm({ fields: [fields[0]] })
    expect(wrapper.find('.form-field').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'TextWidget' }).props('mode')).toBe('edit')
  })

  it('mixes input and display arms within one section (AC 6)', () => {
    const fields = makeFields([
      { render: 'input', verdict: { writable: true } },
      { render: 'display', verdict: { writable: true } },
    ])
    const { wrapper } = mountForm({ fields })
    const items = wrapper.findAll('.property-item')
    expect(items).toHaveLength(2)
    expect(items[0].find('.form-field').exists()).toBe(true)
    expect(items[1].find('.form-field').exists()).toBe(false)
  })

  it('a display-flagged machine field skips the StatusControl (AC 7)', () => {
    // Status fields are the likeliest to be flagged display; a disabled
    // StatusControl is exactly the disabled-input outcome to avoid.
    const fields = makeFields([
      {},
      { render: 'display', transitions: [], verdict: { writable: true } },
    ])
    const { wrapper } = mountForm({ fields: [fields[1]] })
    expect(wrapper.findComponent({ name: 'StatusControl' }).exists()).toBe(false)
  })

  it('an input-flagged machine field still renders the StatusControl', () => {
    const fields = makeFields([
      {},
      { render: 'input', transitions: [], verdict: { writable: true } },
    ])
    const { wrapper } = mountForm({ fields: [fields[1]] })
    expect(wrapper.findComponent({ name: 'StatusControl' }).exists()).toBe(true)
  })

  it('switching a field to display does not fire onVerdictFlip (AC 8)', async () => {
    // RR-PGGRBD: `render` must stay out of `verdict`, or the flip-watcher
    // reads a config change as a revoked permission and toasts.
    const initial = makeFields([{ render: 'input', verdict: { writable: true } }])
    makeStoreMock()
    const { wrapper, onVerdictFlip } = mountForm({ fields: [initial[0]] })
    const flipped = makeFields([{ render: 'display', verdict: { writable: true } }])
    await wrapper.setProps({ fields: [flipped[0]] })
    await nextTick()
    expect(onVerdictFlip).not.toHaveBeenCalled()
  })

  it('gives a long display value its own full-width row', () => {
    const fields = makeFields([{ render: 'display', verdict: { writable: true } }])
    const { wrapper } = mountForm({
      fields: [fields[0]],
      initialValues: { title: 'x'.repeat(61) },
    })
    expect(wrapper.find('.property-item').classes()).toContain('property-long')
  })

  it('does not force full width on an edit-arm field', () => {
    // An edit widget sizes itself; stretching it would widen every textarea.
    const fields = makeFields([{ render: 'input', verdict: { writable: true } }])
    const { wrapper } = mountForm({
      fields: [fields[0]],
      initialValues: { title: 'x'.repeat(61) },
    })
    expect(wrapper.find('.property-item').classes()).not.toContain('property-long')
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
      {
        property: 'photo',
        label: 'Photo',
        kind: 'schema',
        propertyDef: FILE_DEF,
        verdict: { writable: true },
        render: 'input',
      },
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

  // TKT-U62DVR: heading-row placement of the auto-save indicator.
  describe('heading row (indicator placement)', () => {
    it('renders its own heading row with the indicator as a flex sibling when `heading` is set', () => {
      const { wrapper } = mountForm({ heading: 'Properties' })
      const header = wrapper.find('.section-edit-form-header')
      expect(header.exists()).toBe(true)
      // Heading and indicator are siblings inside the one header row.
      expect(header.find('.section-heading').text()).toBe('Properties')
      expect(header.find('[data-testid="autosave-indicator"]').exists()).toBe(true)
      // Exactly one indicator (no duplicate headless one).
      expect(wrapper.findAll('[data-testid="autosave-indicator"]')).toHaveLength(1)
    })

    it('indicator starts idle-hidden in the heading row (no floating check)', () => {
      const { wrapper } = mountForm({ heading: 'Properties' })
      const indicator = wrapper.find('.section-edit-form-header [data-testid="autosave-indicator"]')
      expect(indicator.classes()).toContain('autosave-hidden')
    })

    it('renders no heading row and defers indicator placement to a slot when `heading` is omitted', () => {
      const { wrapper } = mountForm({})
      expect(wrapper.find('.section-edit-form-header').exists()).toBe(false)
      // Headless default still renders an indicator inline (host may override
      // via the #indicator slot, as cards/list do).
      expect(wrapper.findAll('[data-testid="autosave-indicator"]')).toHaveLength(1)
    })

    it('treats an empty-string heading as headless (no header row, single indicator) — RR-32ARO9', () => {
      // The EntityDetail wiring passes `:heading="section.heading"`, which is
      // '' for a headingless configured properties section. That must NOT
      // render an empty header row; it falls back to the headless path.
      const { wrapper } = mountForm({ heading: '' })
      expect(wrapper.find('.section-edit-form-header').exists()).toBe(false)
      expect(wrapper.findAll('[data-testid="autosave-indicator"]')).toHaveLength(1)
    })

    it('lets a host #indicator slot override the default fallback (cards/list path) — RR-95OACT', () => {
      const wrapper = mount(SectionEditForm, {
        props: {
          entityType: 'ticket',
          entityId: 'TKT-001',
          initialValues: { title: 'Original', status: 'open' },
          fields: makeFields(),
          onPropertyApplied: vi.fn(),
          onError: vi.fn(),
          onVerdictFlip: vi.fn(),
        },
        slots: {
          indicator: '<span class="host-indicator">HOST</span>',
        },
      })
      // Host slot content wins; the default AutoSaveIndicator is not rendered.
      expect(wrapper.find('.host-indicator').exists()).toBe(true)
      expect(wrapper.find('[data-testid="autosave-indicator"]').exists()).toBe(false)
    })
  })
})

// TKT-3R7RF3: the `widget:` override selects which registered widget renders a
// field, instead of the type-derived default.
describe('widget override (TKT-3R7RF3)', () => {
  it('renders the overridden widget instead of the type default', () => {
    const { wrapper } = mountForm({
      fields: [
        {
          property: 'title',
          label: 'Title',
          kind: 'schema',
          propertyDef: TEXT_DEF,
          render: 'input',
          widget: 'textarea',
        },
      ],
    })
    // string would ordinarily resolve to TextWidget.
    expect(wrapper.findComponent({ name: 'TextareaWidget' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'TextWidget' }).exists()).toBe(false)
  })

  it('omitting the override keeps the type default', () => {
    const { wrapper } = mountForm({
      fields: [
        { property: 'title', label: 'Title', kind: 'schema', propertyDef: TEXT_DEF, render: 'input' },
      ],
    })
    expect(wrapper.findComponent({ name: 'TextWidget' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'TextareaWidget' }).exists()).toBe(false)
  })

  it('applies on the display arm too, not just edit', () => {
    const { wrapper } = mountForm({
      fields: [
        {
          property: 'title',
          label: 'Title',
          kind: 'schema',
          propertyDef: TEXT_DEF,
          render: 'display',
          widget: 'textarea',
        },
      ],
    })
    const w = wrapper.findComponent({ name: 'TextareaWidget' })
    expect(w.exists()).toBe(true)
    expect(w.props('mode')).toBe('display')
  })

  // RR-2GBB0V: the hint arm has no PropertyDef, so the server could not have
  // type-checked the override — it must be provably DROPPED, not silently
  // honoured. Guards against a refactor that plumbs `widget` into
  // resolveFromHint or reads it before the kind check.
  it('is DROPPED on the hint arm', () => {
    const { wrapper } = mountForm({
      fields: [
        {
          property: 'mystery',
          label: 'Mystery',
          kind: 'hint',
          routingHint: { kind: 'text', propertyName: 'mystery' },
          render: 'input',
          widget: 'textarea',
        },
      ],
      initialValues: { mystery: 'x' },
    })
    // The hint's own kind wins: 'text' -> TextWidget, override ignored.
    expect(wrapper.findComponent({ name: 'TextWidget' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'TextareaWidget' }).exists()).toBe(false)
  })

  // RR-66MT0D: the StatusControl interaction is TWO-AXIS, not simply inert.
  it('is inert on a machine field with render: input (StatusControl owns it)', () => {
    const { wrapper } = mountForm({
      fields: [
        {
          property: 'status',
          label: 'Status',
          kind: 'schema',
          propertyDef: ENUM_DEF,
          render: 'input',
          transitions: [],
          widget: 'textarea',
        },
      ],
    })
    expect(wrapper.findComponent({ name: 'StatusControl' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'TextareaWidget' }).exists()).toBe(false)
  })

  it('IS honoured on a machine field with render: display', () => {
    const { wrapper } = mountForm({
      fields: [
        {
          property: 'status',
          label: 'Status',
          kind: 'schema',
          propertyDef: ENUM_DEF,
          render: 'display',
          transitions: [],
          widget: 'textarea',
        },
      ],
    })
    // render: display deliberately falls through to the display arm rather
    // than rendering a disabled StatusControl (TKT-HOIX1), so the widget wins.
    expect(wrapper.findComponent({ name: 'StatusControl' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'TextareaWidget' }).props('mode')).toBe('display')
  })

  // The ACL conjunction is untouched by widget selection: config can only
  // DOWNGRADE editability. A widget override must not create a path around it.
  it('does not make a read-only field editable', () => {
    const { wrapper } = mountForm({
      fields: [
        {
          property: 'title',
          label: 'Title',
          kind: 'schema',
          propertyDef: TEXT_DEF,
          render: 'input',
          verdict: { writable: false },
          widget: 'textarea',
        },
      ],
    })
    expect(wrapper.findComponent({ name: 'TextareaWidget' }).props('mode')).toBe('display')
  })

  // An unknown name must not throw or blank the field — resolve() falls back
  // to the type default and warns. Defensive: the server rejects these at
  // config load, so this only fires on shape drift.
  it('falls back to the type default on an unknown widget name', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const { wrapper } = mountForm({
      fields: [
        {
          property: 'title',
          label: 'Title',
          kind: 'schema',
          propertyDef: TEXT_DEF,
          render: 'input',
          widget: 'no-such-widget',
        },
      ],
    })
    expect(wrapper.findComponent({ name: 'TextWidget' }).exists()).toBe(true)
    warn.mockRestore()
  })
})
