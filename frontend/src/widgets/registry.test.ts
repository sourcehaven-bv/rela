import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent } from 'vue'
import type { PropertyDef } from '@/types'
import { defineWidgetRegistry, defaultWidgetFor, defaultRegistry } from './registry'
import TextWidget from './TextWidget.vue'
import CheckboxWidget from './CheckboxWidget.vue'
import SelectWidget from './SelectWidget.vue'
import MultiSelectWidget from './MultiSelectWidget.vue'
import DateWidget from './DateWidget.vue'
import NumberWidget from './NumberWidget.vue'
import RruleWidget from './RruleWidget.vue'

function makeStub(name: string) {
  return defineComponent({ name, render: () => null })
}
const Stub = makeStub('Stub')
const Stub2 = makeStub('Stub2')

describe('defaultWidgetFor', () => {
  // Pins the historical FieldRenderer dispatch order (RR-0Z1P6).
  it.each<[string, PropertyDef | undefined, string]>([
    ['undefined propertyDef', undefined, 'text'],
    ['plain string', { type: 'string' }, 'text'],
    ['file', { type: 'file' }, 'file'],
    ['boolean', { type: 'boolean' }, 'checkbox'],
    ['date', { type: 'date' }, 'date'],
    ['integer', { type: 'integer' }, 'number'],
    ['rrule', { type: 'rrule' }, 'rrule'],
    ['enum with values', { type: 'enum', values: ['a', 'b'] }, 'select'],
    ['string with values', { type: 'string', values: ['a'] }, 'select'],
    ['empty values array', { type: 'enum', values: [] }, 'text'],
    ['list wins over values', { type: 'enum', values: ['a'], list: true }, 'multi-select'],
    ['list wins over boolean', { type: 'boolean', list: true }, 'multi-select'],
  ])('%s -> %s', (_name, def, expected) => {
    expect(defaultWidgetFor(def)).toBe(expected)
  })
})

describe('defineWidgetRegistry', () => {
  let warnSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
  })
  afterEach(() => {
    warnSpy.mockRestore()
  })

  it('resolves an explicitly named widget over the type default', () => {
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget })
    r.register('textarea', { component: Stub })
    // type default for string would be text, but explicit name wins.
    expect(r.resolve('textarea', { type: 'string' })).toBe(Stub)
  })

  it('falls back to the type default when no name is given', () => {
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget })
    r.register('checkbox', { component: Stub })
    expect(r.resolve(undefined, { type: 'boolean' })).toBe(Stub)
  })

  it('treats an empty-string widget name as no name', () => {
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget })
    r.register('checkbox', { component: Stub })
    expect(r.resolve('   ', { type: 'boolean' })).toBe(Stub)
  })

  it('warns and falls back when an explicit name is unknown', () => {
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget })
    const got = r.resolve('does-not-exist', { type: 'string' })
    expect(got).toBe(TextWidget)
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('unknown widget "does-not-exist"'))
  })

  it('falls back to text when the type default is not registered', () => {
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget })
    // No date widget registered; should fall back to text.
    expect(r.resolve(undefined, { type: 'date' })).toBe(TextWidget)
  })

  it('throws when nothing — not even text — can be resolved', () => {
    const r = defineWidgetRegistry()
    expect(() => r.resolve(undefined, { type: 'string' })).toThrow(/no widget could be resolved/)
  })

  it('warns when the resolved widget does not declare the property type', () => {
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget })
    r.register('checkbox', { component: Stub, supportedPropertyTypes: ['boolean'] })
    r.resolve('checkbox', { type: 'string' })
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining('does not declare support for property type "string"')
    )
  })

  it('does not warn when the property type is supported', () => {
    const r = defineWidgetRegistry()
    r.register('checkbox', { component: Stub, supportedPropertyTypes: ['boolean'] })
    r.resolve('checkbox', { type: 'boolean' })
    expect(warnSpy).not.toHaveBeenCalled()
  })

  it('does not warn about type support when propertyDef is absent', () => {
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget, supportedPropertyTypes: ['string'] })
    r.resolve('text', undefined)
    expect(warnSpy).not.toHaveBeenCalled()
  })

  it('warns on re-registration of the same name and the last one wins', () => {
    const r = defineWidgetRegistry()
    r.register('text', { component: TextWidget })
    r.register('text', { component: Stub2 })
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('re-registering widget "text"'))
    expect(r.resolve('text', { type: 'string' })).toBe(Stub2)
  })

  it('builds isolated registries that do not share state', () => {
    const a = defineWidgetRegistry()
    const b = defineWidgetRegistry()
    a.register('text', { component: TextWidget })
    b.register('text', { component: Stub })
    expect(a.resolve('text', { type: 'string' })).toBe(TextWidget)
    expect(b.resolve('text', { type: 'string' })).toBe(Stub)
  })
})

describe('defaultRegistry', () => {
  it('resolves every production widget by name', () => {
    expect(defaultRegistry.resolve('text', { type: 'string' })).toBe(TextWidget)
    expect(defaultRegistry.resolve('checkbox', { type: 'boolean' })).toBe(CheckboxWidget)
    expect(defaultRegistry.resolve('select', { type: 'enum', values: ['a'] })).toBe(SelectWidget)
    expect(defaultRegistry.resolve('multi-select', { type: 'enum', list: true })).toBe(
      MultiSelectWidget
    )
    expect(defaultRegistry.resolve('date', { type: 'date' })).toBe(DateWidget)
    expect(defaultRegistry.resolve('number', { type: 'integer' })).toBe(NumberWidget)
    expect(defaultRegistry.resolve('rrule', { type: 'rrule' })).toBe(RruleWidget)
  })

  it('resolves by type default with no explicit name', () => {
    expect(defaultRegistry.resolve(undefined, { type: 'boolean' })).toBe(CheckboxWidget)
    expect(defaultRegistry.resolve(undefined, { type: 'date' })).toBe(DateWidget)
    expect(defaultRegistry.resolve(undefined, { list: true, type: 'enum' })).toBe(MultiSelectWidget)
    expect(defaultRegistry.resolve(undefined, { type: 'string' })).toBe(TextWidget)
  })

  it('falls back to the property-type default when the explicit name is unknown', () => {
    // Pin the warn-then-default path on a non-string property so the
    // fallback is observably not just the universal text widget.
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    expect(defaultRegistry.resolve('does-not-exist', { type: 'boolean' })).toBe(CheckboxWidget)
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('unknown widget "does-not-exist"'))
    warnSpy.mockRestore()
  })

  it('warns when something double-registers a name into defaultRegistry', () => {
    // Mirrors what would happen if a plugin (or an accidental double
    // import) tried to clobber a production widget.
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    defaultRegistry.register('text', { component: makeStub('PluginText') })
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('re-registering widget "text"'))
    // Restore the production widget so subsequent tests see the canonical state.
    defaultRegistry.register('text', { component: TextWidget })
    warnSpy.mockRestore()
  })
})
