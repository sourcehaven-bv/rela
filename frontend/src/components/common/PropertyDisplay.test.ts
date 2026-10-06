import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import PropertyDisplay from './PropertyDisplay.vue'
import { useSchemaStore } from '@/stores/schema'
import type { PropertyDef } from '@/types'

// A view section hands PropertyDisplay the property name plus its type name
// (`propType`). Enum labels resolve per property, so the widget must get the
// property name; the type name matches no property def and showed raw values.
describe('PropertyDisplay enum labels', () => {
  // Shaped like the server's v1 PropertyDef: a custom-typed property carries
  // the type's name and values. The cast is needed because the TS union does
  // not admit custom type names.
  const kindDef = {
    type: 'task_kind',
    values: ['internal_duty', 'external_obligation'],
  } as unknown as PropertyDef
  const areasDef = {
    type: 'work_area',
    list: true,
    values: ['software_development'],
  } as unknown as PropertyDef

  beforeEach(() => {
    setActivePinia(createPinia())
    const schemaStore = useSchemaStore()
    schemaStore.entityTypes = new Map([
      ['task', { label: 'Task', properties: { kind: kindDef, areas: areasDef } }],
    ]) as never
    schemaStore.customTypes = new Map([
      [
        'task_kind',
        {
          values: ['internal_duty', 'external_obligation'],
          labels: { external_obligation: 'External obligation' },
        },
      ],
      [
        'work_area',
        {
          values: ['software_development'],
          labels: { software_development: 'Software development' },
        },
      ],
    ]) as never
    schemaStore.styles = { task_kind: { external_obligation: 'badge-red' } }
  })

  function render(withDefs: boolean) {
    return mount(PropertyDisplay, {
      props: {
        entityType: 'task',
        properties: [
          {
            name: 'kind',
            label: 'Kind',
            value: 'external_obligation',
            propType: 'task_kind',
            propertyDef: withDefs ? kindDef : undefined,
          },
          {
            name: 'areas',
            label: 'Areas',
            value: ['software_development'],
            propType: 'work_area',
            propertyDef: withDefs ? areasDef : undefined,
          },
        ],
      },
    })
  }

  it.each([
    ['with schema defs', true],
    ['via the routing hint', false],
  ])('shows labels when the property name differs from its type (%s)', (_, withDefs) => {
    const badges = render(withDefs).findAll('.badge')
    expect(badges.map((b) => b.text())).toEqual(['External obligation', 'Software development'])
    // The colour still resolves through the property's type.
    expect(badges[0].classes()).toContain('badge--red')
  })

  it('shows labels for an inline enum (wire propType "enum")', () => {
    const levelDef: PropertyDef = { type: 'enum', values: ['p1'], labels: { p1: 'Urgent' } }
    const schemaStore = useSchemaStore()
    schemaStore.entityTypes = new Map([
      ['task', { label: 'Task', properties: { level: levelDef } }],
    ]) as never
    const wrapper = mount(PropertyDisplay, {
      props: {
        entityType: 'task',
        properties: [{ name: 'level', label: 'Level', value: 'p1', propType: 'enum' }],
      },
    })
    expect(wrapper.find('.badge').text()).toBe('Urgent')
  })
})
