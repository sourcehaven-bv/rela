import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ExternalRefWidget from './ExternalRefWidget.vue'
import { externalRefHref, parseExternalRef } from './externalRef'
import { defaultWidgetFor } from './registry'
import { formatCellValue } from '@/utils/format'
import type { EntityType, PropertyDef } from '@/types'

const def = { type: 'external_ref', system: 'jira' } as PropertyDef

describe('parseExternalRef', () => {
  it.each([
    [
      { id: '42', url: 'https://x.test/42' },
      { id: '42', url: 'https://x.test/42' },
    ],
    [{ id: '42' }, { id: '42' }],
    [{ id: '42', url: '' }, { id: '42' }],
    ['42', { id: '42' }],
    [['42'], { id: '42' }],
    [{}, undefined],
    [{ id: 42 }, undefined],
    ['', undefined],
    [null, undefined],
  ])('%j', (input, want) => {
    expect(parseExternalRef(input)).toEqual(want)
  })
})

describe('externalRefHref', () => {
  it('links http and https only', () => {
    expect(externalRefHref({ id: 'a', url: 'https://x.test/a' })).toBe('https://x.test/a')
    expect(externalRefHref({ id: 'a', url: 'http://x.test/a' })).toBe('http://x.test/a')
    expect(externalRefHref({ id: 'a', url: 'javascript:alert(1)' })).toBeUndefined()
    expect(externalRefHref({ id: 'a', url: 'not a url' })).toBeUndefined()
    expect(externalRefHref({ id: 'a' })).toBeUndefined()
  })
})

describe('ExternalRefWidget', () => {
  it('renders the id as a link that opens a new tab', () => {
    const w = mount(ExternalRefWidget, {
      props: {
        modelValue: { id: 'J-1', url: 'https://jira.test/J-1' },
        mode: 'display',
        propertyName: 'jira',
        propertyDef: def,
      },
    })
    const a = w.get('a')
    expect(a.text()).toBe('J-1')
    expect(a.attributes('href')).toBe('https://jira.test/J-1')
    expect(a.attributes('rel')).toContain('noopener')
  })

  it('renders no link for a non-http url', () => {
    const w = mount(ExternalRefWidget, {
      props: {
        modelValue: { id: 'J-1', url: 'javascript:alert(1)' },
        mode: 'display',
        propertyName: 'jira',
      },
    })
    expect(w.find('a').exists()).toBe(false)
    expect(w.text()).toBe('J-1')
  })

  it('is read-only in edit mode and never emits a value', () => {
    const w = mount(ExternalRefWidget, {
      props: { modelValue: { id: 'J-1' }, mode: 'edit', propertyName: 'jira', propertyDef: def },
    })
    expect(w.find('input').exists()).toBe(false)
    expect(w.text()).toContain('Set by the jira sync')
    expect(w.emitted('update:modelValue')).toBeUndefined()
  })

  it('is the default widget for the type', () => {
    expect(defaultWidgetFor(def)).toBe('external-ref')
  })

  it('formats a table cell as the id', () => {
    const type = { label: 'T', properties: { jira: def } } as unknown as EntityType
    expect(formatCellValue({ id: 'J-1', url: 'https://jira.test/J-1' }, 'jira', type)).toBe('J-1')
    expect(formatCellValue({}, 'jira', type)).toBe('')
  })
})
