import { describe, it, expect } from 'vitest'
import type { FormConfig, PropertyDef } from '@/types'
import { conversionFields, createFormFor, isEmptyValue } from './conversionFields'

const properties: Record<string, PropertyDef> = {
  title: { type: 'string', required: true },
  owner: { type: 'string', required: true },
  due: { type: 'date' },
  status: { type: 'enum', values: ['open', 'done'], required: true },
  notes: { type: 'string' },
}

describe('createFormFor', () => {
  it('prefers a create form over an edit form', () => {
    const forms = new Map<string, FormConfig>([
      ['a-edit', { entity: 'task', mode: 'edit' }],
      ['b-new', { entity: 'task' }],
      ['c-other', { entity: 'note' }],
    ])
    expect(createFormFor(forms, 'task')).toBe(forms.get('b-new'))
  })

  it('falls back to an edit form', () => {
    const forms = new Map<string, FormConfig>([['edit', { entity: 'task', mode: 'edit' }]])
    expect(createFormFor(forms, 'task')).toBe(forms.get('edit'))
    expect(createFormFor(forms, 'note')).toBeUndefined()
  })
})

describe('conversionFields', () => {
  it('keeps the form field config and order, and leaves out mapped fields', () => {
    const form: FormConfig = {
      entity: 'task',
      fields: [
        { property: 'due', label: 'Deadline' },
        { property: 'owner', widget: 'user' },
        { property: 'title' },
      ],
    }
    const result = conversionFields(properties, form, new Set(['title']))
    expect(result.required.map((f) => f.property)).toEqual(['owner', 'status'])
    expect(result.required[0]).toEqual({ property: 'owner', widget: 'user' })
    expect(result.optional).toEqual([{ property: 'due', label: 'Deadline' }, { property: 'notes' }])
  })

  it('reads wizard steps, skips relation fields and keeps hidden required ones', () => {
    const form: FormConfig = {
      entity: 'task',
      steps: [
        { title: 'One', fields: [{ property: 'owner', hidden: true }, { relation: 'blocks' }] },
        { title: 'Two', fields: [{ property: 'status', label: 'State' }] },
      ],
    }
    const result = conversionFields(properties, form, new Set())
    expect(result.required).toEqual([
      { property: 'owner' },
      { property: 'status', label: 'State' },
      { property: 'title' },
    ])
  })

  it('gives every settable property a field when no form exists', () => {
    const result = conversionFields(properties, undefined, new Set(['status']))
    expect(result.required.map((f) => f.property)).toEqual(['title', 'owner'])
    expect(result.optional.map((f) => f.property)).toEqual(['due', 'notes'])
  })

  it('ignores form fields for properties the type does not have', () => {
    const form: FormConfig = { entity: 'task', fields: [{ property: 'gone' }] }
    const result = conversionFields({}, form, new Set())
    expect(result).toEqual({ required: [], optional: [], files: [] })
  })
})

describe('conversionFields hidden and file properties', () => {
  const withFile: Record<string, PropertyDef> = {
    title: { type: 'string', required: true },
    notes: { type: 'string' },
    scan: { type: 'file', required: true },
  }

  it('keeps a hidden required field visible and drops a hidden optional one', () => {
    const form: FormConfig = {
      entity: 'task',
      fields: [
        { property: 'title', hidden: true, label: 'Name' },
        { property: 'notes', hidden: true },
      ],
    }
    const result = conversionFields(withFile, form, new Set())
    expect(result.required).toEqual([{ property: 'title', label: 'Name' }])
    expect(result.optional).toEqual([])
  })

  it('lists file properties apart', () => {
    const result = conversionFields(withFile, undefined, new Set())
    expect(result.files).toEqual(['scan'])
    expect(result.required.map((f) => f.property)).toEqual(['title'])
  })
})

describe('isEmptyValue', () => {
  it.each([
    [undefined, true],
    [null, true],
    ['', true],
    [[], true],
    [false, false],
    [0, false],
    ['x', false],
    [['a'], false],
  ])('%j → %s', (value, expected) => {
    expect(isEmptyValue(value)).toBe(expected)
  })
})
