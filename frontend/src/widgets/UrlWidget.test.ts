import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ViewSectionField } from '@/api'
import type { PropertyDef } from '@/types'
import UrlWidget from './UrlWidget.vue'
import { defaultRegistry, defaultWidgetFor } from './registry'
import {
  densePropertyRoutingHint,
  viewFieldRoutingHint,
  viewFieldRoutingHintFor,
} from './viewRouting'

const linkDef: PropertyDef = { type: 'string', display: { link: true } }

function display(modelValue: unknown) {
  return mount(UrlWidget, {
    props: { modelValue, mode: 'display' as const, propertyName: 'pdf_url' },
  })
}

describe('UrlWidget (display)', () => {
  it('renders an https value as a link that opens in a new tab without an opener', () => {
    const w = display('https://example.com/invoice.pdf')
    const a = w.find('a')
    expect(a.exists()).toBe(true)
    expect(a.attributes('href')).toBe('https://example.com/invoice.pdf')
    expect(a.attributes('target')).toBe('_blank')
    expect(a.attributes('rel')).toBe('noopener noreferrer')
    expect(a.text()).toBe('https://example.com/invoice.pdf')
  })

  it('links the first element of a single-value list', () => {
    expect(display(['https://example.com/a']).find('a').attributes('href')).toBe(
      'https://example.com/a'
    )
  })

  it.each([
    ['javascript', 'javascript:alert(1)'],
    ['data', 'data:text/html,<script>alert(1)</script>'],
    ['relative path', '/files/invoice.pdf'],
  ])('renders a %s value as text, not a link', (_name, value) => {
    const w = display(value)
    expect(w.find('a').exists()).toBe(false)
    expect(w.find('span.display-value').text()).toBe(value)
  })

  it('renders nothing for an empty value', () => {
    const w = display(undefined)
    expect(w.find('a').exists()).toBe(false)
    expect(w.text()).toBe('')
  })

  it('opens a mailto link in place, not in a new tab', () => {
    const a = display('mailto:billing@example.com').find('a')
    expect(a.attributes('href')).toBe('mailto:billing@example.com')
    expect(a.attributes('target')).toBeUndefined()
  })

  // A link on a row, card or chip that opens on click must not open it too.
  it('stops a click on the link from reaching the surrounding control', async () => {
    let reached = false
    const host = document.createElement('div')
    host.addEventListener('click', () => (reached = true))
    document.body.appendChild(host)
    const w = mount(UrlWidget, {
      props: { modelValue: 'https://a.example/', mode: 'display' as const, propertyName: 'p' },
      attachTo: host,
    })
    w.find('a').element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))
    expect(reached).toBe(false)
    w.unmount()
    host.remove()
  })

  // A single root element, so a class the caller passes lands on it.
  it('keeps a class passed by the caller', () => {
    const w = mount(UrlWidget, {
      props: { modelValue: 'https://a.example/', mode: 'display' as const, propertyName: 'p' },
      attrs: { class: 'field-value' },
    })
    expect(w.find('a').classes()).toContain('field-value')
  })
})

describe('UrlWidget (edit)', () => {
  // A text input with the URL keyboard, not type="url": a form submit must
  // not refuse an entity whose stored value is not a full URL.
  it('renders a URL-keyboard input and emits the typed value', async () => {
    const w = mount(UrlWidget, {
      props: { modelValue: 'https://a.example', mode: 'edit' as const, propertyName: 'pdf_url' },
    })
    const input = w.find('input[inputmode="url"]')
    expect(input.attributes('type')).toBe('text')
    expect((input.element as HTMLInputElement).value).toBe('https://a.example')
    expect(w.find('a').exists()).toBe(false)
    await input.setValue('https://b.example')
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['https://b.example'])
  })
})

// One widget on every surface: the form and detail side (resolve), the view
// section side (viewFieldRoutingHint) and the dense side (list cells, kanban
// cards) all reach UrlWidget for a link-formatted string.
describe('url routing', () => {
  it('is the default widget for a string with display.link', () => {
    expect(defaultWidgetFor(linkDef)).toBe('url')
    expect(defaultRegistry.resolve(undefined, linkDef)).toBe(UrlWidget)
  })

  it('is selectable by name as a widget override', () => {
    expect(defaultRegistry.resolve('url', { type: 'string' })).toBe(UrlWidget)
  })

  it('leaves a plain string on the text widget', () => {
    expect(defaultWidgetFor({ type: 'string' })).toBe('text')
  })

  it('routes a view-section field to url when its def has display.link', () => {
    const field: ViewSectionField = {
      label: 'PDF',
      property: 'pdf_url',
      propType: 'string',
      values: ['https://a.example'],
    }
    const hint = viewFieldRoutingHint(field, linkDef)
    expect(hint).toEqual({ kind: 'url', propertyName: 'pdf_url' })
    expect(defaultRegistry.resolveFromHint(hint)).toBe(UrlWidget)
    // Without the def the field keeps its existing routing.
    expect(viewFieldRoutingHint(field).kind).toBe('enum-list')
  })

  it('looks up the def for a cards or list section field by entity type', () => {
    const field: ViewSectionField = { label: 'PDF', property: 'pdf_url', propType: 'string' }
    const lookups: string[] = []
    const lookup = (type: string, prop: string) => {
      lookups.push(`${type}.${prop}`)
      return type === 'invoice' && prop === 'pdf_url' ? linkDef : undefined
    }
    expect(viewFieldRoutingHintFor(field, 'invoice', lookup).kind).toBe('url')
    expect(viewFieldRoutingHintFor(field, 'other', lookup).kind).toBe('enum-list')
    expect(lookups).toEqual(['invoice.pdf_url', 'other.pdf_url'])
  })

  it('routes a dense cell to url and hands it the raw value', () => {
    const hint = densePropertyRoutingHint(linkDef, 'pdf_url')
    expect(hint).toEqual({ kind: 'url', propertyName: 'pdf_url', preformatted: false })
    expect(defaultRegistry.resolveDense(hint)).toBe(UrlWidget)
  })
})
