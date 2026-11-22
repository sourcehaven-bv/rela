import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { mount } from '@vue/test-utils'
import type { ViewSectionField } from '@/api'
import type { PropertyDef } from '@/types'
import { useSchemaStore } from '@/stores/schema'
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

  it('shows a URL as itself on one line with the whole URL on hover', () => {
    const url = 'https://example.com/a/very/long/path?with=a&long=query'
    const a = display(url).find('a')
    expect(a.text()).toBe(url)
    expect(a.attributes('title')).toBe(url)
    expect(a.classes()).toContain('url-link--truncated')
  })

  it('shows link_text instead of the URL', () => {
    const def: PropertyDef = { type: 'string', display: { link: true, link_text: 'Website' } }
    const a = mount(UrlWidget, {
      props: {
        modelValue: 'https://example.com/',
        mode: 'display' as const,
        propertyName: 'site',
        propertyDef: def,
      },
    }).find('a')
    expect(a.text()).toBe('Website')
    expect(a.attributes('href')).toBe('https://example.com/')
    // No title: it would give the link the URL as its description, and
    // some screen readers announce it in place of the text.
    expect(a.attributes('title')).toBeUndefined()
    expect(a.attributes('aria-label')).toBeUndefined()
    expect(a.classes()).not.toContain('url-link--truncated')
  })

  // Cells and cards pass only the property name and the entity type.
  it('finds link_text in the schema when no def is passed', () => {
    useSchemaStore().entityTypes.set('ship', {
      name: 'ship',
      properties: { site: { type: 'string', display: { link: true, link_text: 'Website' } } },
    } as never)
    const w = mount(UrlWidget, {
      props: {
        modelValue: 'https://example.com/',
        mode: 'display' as const,
        propertyName: 'site',
        entityType: 'ship',
      },
    })
    expect(w.find('a').text()).toBe('Website')
  })

  // A cell without an entity type must not borrow another type's label.
  it.each([
    ['all types agree', 'Website', 'Website'],
    ['the types disagree', 'Homepage', 'https://example.com/'],
    ['one type has no text', undefined, 'https://example.com/'],
  ])('without an entity type, when %s', (_, other, want) => {
    const store = useSchemaStore()
    store.entityTypes.set('ship', {
      name: 'ship',
      properties: { site: { type: 'string', display: { link: true, link_text: 'Website' } } },
    } as never)
    store.entityTypes.set('port', {
      name: 'port',
      properties: { site: { type: 'string', display: { link: true, link_text: other } } },
    } as never)
    const w = mount(UrlWidget, {
      props: { modelValue: 'https://example.com/', mode: 'display' as const, propertyName: 'site' },
    })
    expect(w.find('a').text()).toBe(want)
  })

  it('takes the own type over the others when the entity type is known', () => {
    const store = useSchemaStore()
    store.entityTypes.set('ship', {
      name: 'ship',
      properties: { site: { type: 'string', display: { link: true } } },
    } as never)
    store.entityTypes.set('port', {
      name: 'port',
      properties: { site: { type: 'string', display: { link: true, link_text: 'Website' } } },
    } as never)
    const w = mount(UrlWidget, {
      props: {
        modelValue: 'https://example.com/',
        mode: 'display' as const,
        propertyName: 'site',
        entityType: 'ship',
      },
    })
    expect(w.find('a').text()).toBe('https://example.com/')
  })

  // The hover text is the target the link opens, not the stored text.
  it('shows the link target as the title', () => {
    const w = display('mailto:a@example.com?cc=b@example.com')
    const a = w.find('a')
    expect(a.attributes('title')).toBe(a.attributes('href'))
  })

  it('keeps the URL as the text of a value that is not a link', () => {
    const def: PropertyDef = { type: 'string', display: { link: true, link_text: 'Website' } }
    const w = mount(UrlWidget, {
      props: {
        modelValue: 'example.com',
        mode: 'display' as const,
        propertyName: 'site',
        propertyDef: def,
      },
    })
    expect(w.find('a').exists()).toBe(false)
    expect(w.text()).toBe('example.com')
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

// Vitest does not apply an SFC's scoped style, so the link style is checked
// in the source, as styles/focusRing.test.ts does for focus rings.
describe('UrlWidget link style', () => {
  const source = readFileSync(resolve(__dirname, 'UrlWidget.vue'), 'utf8')

  it('takes the theme link colour, not the browser default', () => {
    const rule = source.match(/\.url-link \{([^}]*)\}/)?.[1] ?? ''
    expect(rule).toContain('color: var(--rl-color-accent)')
    expect(rule).toContain('text-decoration: none')
  })

  it('underlines the link on hover', () => {
    const rule = source.match(/\.url-link:hover \{([^}]*)\}/)?.[1] ?? ''
    expect(rule).toContain('text-decoration: underline')
  })
})
