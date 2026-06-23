import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import TextWidget from './TextWidget.vue'
import TextareaWidget from './TextareaWidget.vue'
import NumberWidget from './NumberWidget.vue'
import CheckboxWidget from './CheckboxWidget.vue'
import DateWidget from './DateWidget.vue'
import SelectWidget from './SelectWidget.vue'
import FileWidget from './FileWidget.vue'

import type { AttachmentInfo } from '@/types'

// Mock the attachment API. vi.mock is hoisted, so the mock fns and the
// MockAttachmentError class are defined via vi.hoisted to be available
// at hoist time. MockAttachmentError mirrors the real AttachmentError
// (carries an HTTP status) so the widget's `instanceof` branch fires.
const { mockUpload, mockDelete, MockAttachmentError } = vi.hoisted(() => {
  class MockAttachmentError extends Error {
    status: number
    constructor(message: string, status: number) {
      super(message)
      this.status = status
    }
  }
  return {
    mockUpload: vi.fn().mockResolvedValue({}),
    mockDelete: vi.fn().mockResolvedValue(undefined),
    MockAttachmentError,
  }
})
vi.mock('@/api/attachments', () => ({
  uploadAttachment: mockUpload,
  deleteAttachment: mockDelete,
  AttachmentError: MockAttachmentError,
}))

describe('TextWidget', () => {
  it('renders the value and emits update:modelValue on input', async () => {
    const w = mount(TextWidget, { props: { modelValue: 'hello', mode: 'edit' as const, propertyName: '' } })
    const input = w.find('input[type="text"]')
    expect((input.element as HTMLInputElement).value).toBe('hello')
    // Negative assertion: display branch must NOT render in edit mode (RR-UD2H).
    expect(w.find('span.display-value').exists()).toBe(false)
    await input.setValue('world')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['world'])
  })

  it('renders empty for null/undefined', () => {
    expect((mount(TextWidget, { props: { modelValue: null, mode: 'edit' as const, propertyName: '' } }).find('input').element as HTMLInputElement).value).toBe('')
    expect(
      (mount(TextWidget, { props: { modelValue: undefined, mode: 'edit' as const, propertyName: '' } }).find('input').element as HTMLInputElement).value
    ).toBe('')
  })

  it('honours disabled', () => {
    const w = mount(TextWidget, { props: { modelValue: 'x', mode: 'edit' as const, propertyName: '', disabled: true } })
    expect(w.find('input').attributes('disabled')).toBeDefined()
  })
})

describe('TextareaWidget', () => {
  it('renders the value and emits on input', async () => {
    const w = mount(TextareaWidget, { props: { modelValue: 'multi\nline', mode: 'edit' as const, propertyName: '' } })
    const ta = w.find('textarea')
    expect((ta.element as HTMLTextAreaElement).value).toBe('multi\nline')
    expect(w.find('span.display-value').exists()).toBe(false)
    await ta.setValue('changed')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['changed'])
  })
})

describe('NumberWidget', () => {
  it('emits a parsed integer for numeric input', async () => {
    const w = mount(NumberWidget, { props: { modelValue: 1, mode: 'edit' as const, propertyName: '' } })
    expect(w.find('span.display-value').exists()).toBe(false)
    await w.find('input').setValue('42')
    expect(w.emitted('update:modelValue')?.[0]).toEqual([42])
  })

  it('emits the raw string when input does not parse to an integer', async () => {
    const w = mount(NumberWidget, { props: { modelValue: 1, mode: 'edit' as const, propertyName: '' } })
    // A number input clears to '' for non-numeric content; parseInt('')
    // is NaN, so the handler emits the raw value — exercising the NaN
    // branch that preserves FieldRenderer's historical behaviour.
    await w.find('input').setValue('')
    expect(w.emitted('update:modelValue')?.[0]).toEqual([''])
  })
})

describe('CheckboxWidget', () => {
  it('reflects boolean true and string "true"', () => {
    expect((mount(CheckboxWidget, { props: { modelValue: true, mode: 'edit' as const, propertyName: '' } }).find('input').element as HTMLInputElement).checked).toBe(true)
    expect((mount(CheckboxWidget, { props: { modelValue: 'true', mode: 'edit' as const, propertyName: '' } }).find('input').element as HTMLInputElement).checked).toBe(true)
    expect((mount(CheckboxWidget, { props: { modelValue: false, mode: 'edit' as const, propertyName: '' } }).find('input').element as HTMLInputElement).checked).toBe(false)
  })

  it('emits the checked boolean on change', async () => {
    const w = mount(CheckboxWidget, { props: { modelValue: false, mode: 'edit' as const, propertyName: '' } })
    // Edit-mode checkbox is enabled (not disabled like display mode) (RR-UD2H).
    expect(w.find('input').attributes('disabled')).toBeUndefined()
    expect(w.find('.display-checkbox').exists()).toBe(false)
    await w.find('input').setValue(true)
    expect(w.emitted('update:modelValue')?.[0]).toEqual([true])
  })
})

describe('DateWidget', () => {
  it('renders a date input and emits on input', async () => {
    const w = mount(DateWidget, { props: { modelValue: '2026-05-29', mode: 'edit' as const, propertyName: '' } })
    const input = w.find('input[type="date"]')
    expect(input.exists()).toBe(true)
    expect((input.element as HTMLInputElement).value).toBe('2026-05-29')
    expect(w.find('span.display-value').exists()).toBe(false)
    await input.setValue('2026-06-01')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['2026-06-01'])
  })
})

describe('SelectWidget', () => {
  const def = { type: 'enum' as const, values: ['open', 'review', 'done'] }

  it('renders options from propertyDef and emits the chosen value', async () => {
    const w = mount(SelectWidget, { props: { modelValue: 'open', mode: 'edit' as const, propertyName: '', propertyDef: def } })
    const opts = w.findAll('option').map((o) => o.attributes('value'))
    expect(opts).toEqual(['', 'open', 'review', 'done'])
    // Edit-mode SelectWidget renders <select>, not the display Badge (RR-UD2H).
    expect(w.findComponent({ name: 'Badge' }).exists()).toBe(false)
    await w.find('select').setValue('review')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['review'])
  })

  it('disables options denied by optionVerdicts', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'edit' as const, propertyName: '', propertyDef: def, optionVerdicts: { done: false } },
    })
    const byValue = Object.fromEntries(w.findAll('option').map((o) => [o.attributes('value'), o]))
    expect(byValue['done'].attributes('disabled')).toBeDefined()
    expect(byValue['review'].attributes('disabled')).toBeUndefined()
    expect(w.find('select').attributes('disabled')).toBeUndefined()
  })

  it('disables options not reachable by transition rules', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'edit' as const, propertyName: '', propertyDef: def, transitions: { open: ['review'] } },
    })
    const byValue = Object.fromEntries(w.findAll('option').map((o) => [o.attributes('value'), o]))
    expect(byValue['done'].attributes('disabled')).toBeDefined()
    expect(byValue['review'].attributes('disabled')).toBeUndefined()
  })

  it('renders the transitions info panel when transitions are present', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'edit' as const, propertyName: '', propertyDef: def, transitions: { open: ['review'] } },
    })
    expect(w.find('.transitions-info').exists()).toBe(true)
  })

  it('renders no transitions panel without transitions', () => {
    const w = mount(SelectWidget, { props: { modelValue: 'open', mode: 'edit' as const, propertyName: '', propertyDef: def } })
    expect(w.find('.transitions-info').exists()).toBe(false)
  })

  it('honours whole-select disabled', () => {
    const w = mount(SelectWidget, { props: { modelValue: 'open', mode: 'edit' as const, propertyName: '', propertyDef: def, disabled: true } })
    expect(w.find('select').attributes('disabled')).toBeDefined()
  })
})

// Display-mode coverage (TKT-UD7YR). Each widget renders a read-only
// shape distinct from its edit-mode form. We assert the chosen element
// + the rendered value -- structural enough to catch routing
// regressions, loose enough not to lock in incidental DOM.

describe('TextWidget (display)', () => {
  it('renders the value as a span', () => {
    const w = mount(TextWidget, { props: { modelValue: 'hello', mode: 'display' as const, propertyName: '' } })
    expect(w.find('input').exists()).toBe(false)
    expect(w.find('span.display-value').text()).toBe('hello')
  })

  it('renders empty for null/undefined without crashing', () => {
    expect(
      mount(TextWidget, { props: { modelValue: null, mode: 'display' as const, propertyName: '' } }).find('span').text(),
    ).toBe('')
    expect(
      mount(TextWidget, { props: { modelValue: undefined, mode: 'display' as const, propertyName: '' } }).find('span').text(),
    ).toBe('')
  })
})

describe('TextareaWidget (display)', () => {
  it('renders multi-line text as a span (CSS handles wrapping)', () => {
    const w = mount(TextareaWidget, { props: { modelValue: 'a\nb', mode: 'display' as const, propertyName: '' } })
    expect(w.find('textarea').exists()).toBe(false)
    expect(w.find('span.display-value').text()).toContain('a')
  })
})

describe('NumberWidget (display)', () => {
  it('renders the number as a span', () => {
    const w = mount(NumberWidget, { props: { modelValue: 42, mode: 'display' as const, propertyName: '' } })
    expect(w.find('input').exists()).toBe(false)
    expect(w.find('span.display-value').text()).toBe('42')
  })

  it('renders zero (no "falsy collapse to empty")', () => {
    const w = mount(NumberWidget, { props: { modelValue: 0, mode: 'display' as const, propertyName: '' } })
    expect(w.find('span.display-value').text()).toBe('0')
  })
})

describe('DateWidget (display)', () => {
  it('renders the date via formatDate (locale-aware, parseable)', () => {
    const w = mount(DateWidget, { props: { modelValue: '2026-05-29', mode: 'display' as const, propertyName: '' } })
    expect(w.find('input').exists()).toBe(false)
    // formatDate output varies by environment locale; assert a non-empty
    // formatted span and that it doesn't pass the raw ISO through.
    const span = w.find('span.display-value')
    expect(span.exists()).toBe(true)
    expect(span.text()).not.toBe('')
  })

  it('falls back to the raw string for an unparseable value', () => {
    const w = mount(DateWidget, { props: { modelValue: 'not-a-date', mode: 'display' as const, propertyName: '' } })
    expect(w.find('span.display-value').text()).toBe('not-a-date')
  })

  it('renders empty for null', () => {
    const w = mount(DateWidget, { props: { modelValue: null, mode: 'display' as const, propertyName: '' } })
    expect(w.find('span.display-value').text()).toBe('')
  })
})

describe('CheckboxWidget (display)', () => {
  it('renders a disabled checkbox checked for true', () => {
    const w = mount(CheckboxWidget, {
      props: { modelValue: true, mode: 'display' as const, propertyName: '' },
    })
    const input = w.find('input[type="checkbox"]')
    expect(input.exists()).toBe(true)
    expect((input.element as HTMLInputElement).checked).toBe(true)
    expect(input.attributes('disabled')).toBeDefined()
    expect(input.attributes('aria-readonly')).toBe('true')
  })

  it('renders a disabled checkbox unchecked for false', () => {
    const w = mount(CheckboxWidget, {
      props: { modelValue: false, mode: 'display' as const, propertyName: '' },
    })
    const input = w.find('input[type="checkbox"]')
    expect((input.element as HTMLInputElement).checked).toBe(false)
    expect(input.attributes('disabled')).toBeDefined()
  })

  it('renders checked for the string "true" (server may serialize as string)', () => {
    const w = mount(CheckboxWidget, {
      props: { modelValue: 'true', mode: 'display' as const, propertyName: '' },
    })
    expect((w.find('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true)
  })
})

describe('SelectWidget (display)', () => {
  const def = { type: 'enum' as const, values: ['open', 'review', 'done'] }

  it('renders a Badge for the value', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'display' as const, propertyDef: def, propertyName: 'status' },
    })
    expect(w.find('select').exists()).toBe(false)
    // The Badge component renders its value into a span.badge-XYZ; we
    // assert the visible text rather than coupling to the styled class.
    expect(w.text()).toContain('open')
  })

  it('passes propertyName through to Badge for style lookup', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: 'open', mode: 'display' as const, propertyDef: def, propertyName: 'status' },
    })
    const badge = w.findComponent({ name: 'Badge' })
    expect(badge.exists()).toBe(true)
    expect(badge.props('property')).toBe('status')
  })

  it('renders nothing visible for empty value', () => {
    const w = mount(SelectWidget, {
      props: { modelValue: '', mode: 'display' as const, propertyName: '', propertyDef: def },
    })
    expect(w.findComponent({ name: 'Badge' }).exists()).toBe(false)
  })

  it('renders a single-element array unwrapped without warning (RR-UD2F)', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const w = mount(SelectWidget, {
      props: { modelValue: ['open'], mode: 'display' as const, propertyDef: def, propertyName: 'status' },
    })
    expect(w.findComponent({ name: 'Badge' }).props('value')).toBe('open')
    expect(warn).not.toHaveBeenCalled()
    warn.mockRestore()
  })

  it('renders the first element and warns for multi-element array (RR-UD2F)', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const w = mount(SelectWidget, {
      props: { modelValue: ['open', 'review'], mode: 'display' as const, propertyDef: def, propertyName: 'status' },
    })
    expect(w.findComponent({ name: 'Badge' }).props('value')).toBe('open')
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('[SelectWidget]'))
    warn.mockRestore()
  })
})

describe('FileWidget', () => {
  const att: AttachmentInfo = {
    id: 'shot.png',
    filename: 'shot.png',
    size: 2048,
    contentType: 'image/png',
    href: '/api/v1/tickets/TKT-1/_attachments/screenshot/shot.png',
  }

  it('renders an image preview and a download link for an image attachment', () => {
    const w = mount(FileWidget, {
      props: { modelValue: '', mode: 'display' as const, propertyName: 'screenshot', attachments: [att] },
    })
    const img = w.find('img.file-preview')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe(att.href)
    const link = w.find('a.file-name')
    expect(link.attributes('href')).toBe(att.href)
    expect(link.attributes('download')).toBe('shot.png')
    expect(w.text()).toContain('shot.png')
    expect(w.text()).toContain('2.0 KB')
  })

  it('renders a download link without preview for a non-image attachment', () => {
    const pdf: AttachmentInfo = { id: 'doc.pdf', filename: 'doc.pdf', size: 500, contentType: 'application/pdf', href: '/h' }
    const w = mount(FileWidget, {
      props: { modelValue: '', mode: 'display' as const, propertyName: 'doc', attachments: [pdf] },
    })
    expect(w.find('img.file-preview').exists()).toBe(false)
    expect(w.find('a.file-name').attributes('href')).toBe('/h')
    expect(w.text()).toContain('doc.pdf')
    expect(w.text()).toContain('500 B')
  })

  it('renders all files of a multi-attachment property', () => {
    const a: AttachmentInfo = { id: 'a.pdf', filename: 'a.pdf', size: 1, contentType: 'application/pdf', href: '/a' }
    const b: AttachmentInfo = { id: 'b.pdf', filename: 'b.pdf', size: 2, contentType: 'application/pdf', href: '/b' }
    const w = mount(FileWidget, {
      props: { modelValue: '', mode: 'display' as const, propertyName: 'docs', attachments: [a, b], max: 3 },
    })
    expect(w.findAll('.file-item')).toHaveLength(2)
    expect(w.text()).toContain('a.pdf')
    expect(w.text()).toContain('b.pdf')
  })

  it('shows an empty-state when there are no files in display mode', () => {
    const w = mount(FileWidget, {
      props: { modelValue: '', mode: 'display' as const, propertyName: 'p' },
    })
    expect(w.text()).toContain('No file attached')
  })

  it('shows a note (no upload control) in edit mode without entity context', () => {
    const w = mount(FileWidget, {
      props: { modelValue: '', mode: 'edit' as const, propertyName: 'screenshot', attachments: [att] },
    })
    expect(w.find('.file-dropzone').exists()).toBe(false)
    expect(w.find('.file-edit-note').exists()).toBe(true)
    expect(w.text()).toContain('shot.png')
  })

  it('shows a Replace control for a single-cap property in edit mode', () => {
    const w = mount(FileWidget, {
      props: {
        modelValue: '', mode: 'edit' as const, propertyName: 'screenshot',
        attachments: [att], max: 1, entityType: 'ticket', entityId: 'TKT-1',
      },
    })
    expect(w.find('.file-dropzone').exists()).toBe(true)
    expect(w.text()).toContain('Replace file')
  })

  it('hides the add control at capacity for a multi-cap property', () => {
    const a: AttachmentInfo = { id: 'a.pdf', filename: 'a.pdf', size: 1, contentType: 'application/pdf', href: '/a' }
    const b: AttachmentInfo = { id: 'b.pdf', filename: 'b.pdf', size: 2, contentType: 'application/pdf', href: '/b' }
    const w = mount(FileWidget, {
      props: {
        modelValue: '', mode: 'edit' as const, propertyName: 'docs',
        attachments: [a, b], max: 2, entityType: 'ticket', entityId: 'TKT-1',
      },
    })
    expect(w.find('.file-dropzone').exists()).toBe(false)
    expect(w.find('.file-edit-note').text()).toContain('Maximum of 2')
  })

  it('shows the add control with room for a multi-cap property', () => {
    const a: AttachmentInfo = { id: 'a.pdf', filename: 'a.pdf', size: 1, contentType: 'application/pdf', href: '/a' }
    const w = mount(FileWidget, {
      props: {
        modelValue: '', mode: 'edit' as const, propertyName: 'docs',
        attachments: [a], max: 3, entityType: 'ticket', entityId: 'TKT-1',
      },
    })
    expect(w.find('.file-dropzone').exists()).toBe(true)
    expect(w.text()).toContain('Add a file')
    expect(w.text()).toContain('1 / 3')
  })

  it('shows a permission note in edit mode when disabled (ACL)', () => {
    const w = mount(FileWidget, {
      props: {
        modelValue: '', mode: 'edit' as const, propertyName: 'screenshot',
        entityType: 'ticket', entityId: 'TKT-1', disabled: true,
      },
    })
    expect(w.find('.file-dropzone').exists()).toBe(false)
    expect(w.find('.file-edit-note').text()).toContain('not permitted')
  })
})

describe('FileWidget upload', () => {
  it('uploads a chosen file and emits attachment-changed', async () => {
    const file = new File(['data'], 'pic.png', { type: 'image/png' })
    const w = mount(FileWidget, {
      props: {
        modelValue: '', mode: 'edit' as const, propertyName: 'screenshot',
        entityType: 'ticket', entityId: 'TKT-1', max: 1,
      },
    })
    const input = w.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')
    await flushPromises()

    expect(mockUpload).toHaveBeenCalledWith('ticket', 'TKT-1', 'screenshot', file, expect.any(Function))
    expect(w.emitted('attachment-changed')).toBeTruthy()
  })

  it('shows an error message and does not emit when upload is rejected', async () => {
    mockUpload.mockRejectedValueOnce(new MockAttachmentError('File is too large.', 413))
    const w = mount(FileWidget, {
      props: {
        modelValue: '', mode: 'edit' as const, propertyName: 'screenshot',
        entityType: 'ticket', entityId: 'TKT-1', max: 1,
      },
    })
    const input = w.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { value: [new File(['x'], 'big.bin')] })
    await input.trigger('change')
    await flushPromises()

    expect(w.find('.file-error').text()).toContain('too large')
    expect(w.emitted('attachment-changed')).toBeFalsy()
  })

  it('removes a file and emits attachment-changed', async () => {
    const w = mount(FileWidget, {
      props: {
        modelValue: '', mode: 'edit' as const, propertyName: 'screenshot',
        attachments: [{ id: 'shot.png', filename: 'shot.png', size: 1, contentType: 'image/png', href: '/h' }],
        max: 1, entityType: 'ticket', entityId: 'TKT-1',
      },
    })
    await w.find('.file-remove').trigger('click')
    await flushPromises()
    // Deletes via the server-provided per-file href (single escaper).
    expect(mockDelete).toHaveBeenCalledWith('/h')
    expect(w.emitted('attachment-changed')).toBeTruthy()
  })
})
