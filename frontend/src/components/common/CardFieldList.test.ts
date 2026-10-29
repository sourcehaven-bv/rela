import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useSchemaStore } from '@/stores/schema'
import { defineComponent, h } from 'vue'
import CardFieldList from './CardFieldList.vue'
import { cardFieldLabel, cardFieldLabelShown } from '@/types/config'

/** A stand-in widget, so the component's widget branch can be exercised
 * without pulling the real registry into a unit test. */
const StubWidget = defineComponent({
  props: {
    modelValue: { type: null, default: undefined },
    mode: { type: String, default: 'display' },
    propertyName: { type: String, default: '' },
    entityType: { type: String, default: '' },
  },
  setup: (props) => () => h('em', { 'data-type': props.entityType }, String(props.modelValue)),
})

describe('cardFieldLabel', () => {
  it('derives the label from the field rather than making an author restate it', () => {
    expect(cardFieldLabel({ property: 'assignee' })).toBe('assignee')
    expect(cardFieldLabel({ relation: 'belongs-to' })).toBe('belongs-to')
  })

  it('prefers a relation type’s authored label over its raw id', () => {
    const lookup = (rel: string) => (rel === 'belongs-to' ? 'belongs to' : undefined)
    // A raw relation id reads like a field name on a chip; the metamodel
    // already carries a human label one lookup away.
    expect(cardFieldLabel({ relation: 'belongs-to' }, lookup)).toBe('belongs to')
    // An unlabelled relation still falls back to its id rather than blank.
    expect(cardFieldLabel({ relation: 'mentions' }, lookup)).toBe('mentions')
  })

  it('does not invent a label for a property', () => {
    // Labels are authored, never derived (DEC-6C1NAA): an operator who wants
    // "Assignee" writes it, rather than the UI title-casing an identifier.
    expect(cardFieldLabel({ property: 'assignee' }, () => 'ignored')).toBe('assignee')
  })

  it('prefers an explicit override', () => {
    expect(cardFieldLabel({ property: 'assignee', label: 'Owner' })).toBe('Owner')
  })
})

describe('cardFieldLabelShown', () => {
  // Unset must mean shown: a plain bool's zero value would silently turn every
  // label off the moment the key appeared anywhere.
  it('shows the label unless explicitly disabled', () => {
    expect(cardFieldLabelShown({ property: 'assignee' })).toBe(true)
    expect(cardFieldLabelShown({ property: 'assignee', show_label: true })).toBe(true)
    expect(cardFieldLabelShown({ property: 'priority', show_label: false })).toBe(false)
  })

  // A busy indicator names its channel type to a screen reader already.
  it('hides a busy field label unless asked for', () => {
    expect(cardFieldLabelShown({ busy: 'run' })).toBe(false)
    expect(cardFieldLabelShown({ busy: 'run', show_label: true })).toBe(true)
  })
})

describe('CardFieldList', () => {
  beforeEach(() => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const schema = useSchemaStore()
    schema.relationTypes.set('belongs-to', { label: 'belongs to' } as never)
  })

  it('renders one line per field, label then value', () => {
    const wrapper = mount(CardFieldList, {
      props: {
        fields: [
          { field: { property: 'assignee' }, text: 'Alex' },
          { field: { property: 'reviewer' }, text: 'Sam' },
        ],
      },
    })

    const lines = wrapper.findAll('.card-field')
    expect(lines).toHaveLength(2)
    // Two person fields are indistinguishable without their labels — the case
    // the label default exists to serve.
    expect(lines[0].text()).toContain('assignee')
    expect(lines[0].text()).toContain('Alex')
    expect(lines[1].text()).toContain('reviewer')
    expect(lines[1].text()).toContain('Sam')
  })

  it('omits the label when show_label is false', () => {
    const wrapper = mount(CardFieldList, {
      props: {
        fields: [{ field: { property: 'priority', show_label: false }, text: 'High' }],
      },
    })

    expect(wrapper.find('.field-label').exists()).toBe(false)
    expect(wrapper.text()).toBe('High')
  })

  it('uses an explicit label over the derived one', () => {
    const wrapper = mount(CardFieldList, {
      props: { fields: [{ field: { property: 'assignee', label: 'Owner' }, text: 'Alex' }] },
    })

    expect(wrapper.text()).toContain('Owner')
    expect(wrapper.text()).not.toContain('assignee')
  })

  it('renders through a widget when one is supplied, else plain text', () => {
    const wrapper = mount(CardFieldList, {
      props: {
        fields: [
          { field: { property: 'status' }, component: StubWidget, modelValue: 'todo', text: 'Todo' },
          { field: { relation: 'belongs-to' }, text: 'Apollo' },
        ],
      },
    })

    // The widget renders the model value; the relation falls back to text.
    expect(wrapper.find('em').text()).toBe('todo')
    expect(wrapper.text()).toContain('Apollo')
  })

  it('renders a relation’s authored label instead of its raw id', () => {
    const wrapper = mount(CardFieldList, {
      props: { fields: [{ field: { relation: 'belongs-to' }, text: 'Apollo' }] },
    })

    expect(wrapper.text()).toContain('belongs to')
    expect(wrapper.text()).not.toContain('belongs-to')
  })

  it('renders each related value with the widget and type it came with', () => {
    const wrapper = mount(CardFieldList, {
      props: {
        entityType: 'deal',
        fields: [
          {
            field: { relation: 'belongs-to', property: 'segment' },
            text: 'hospital, Gov',
            values: [
              { key: 'a', component: StubWidget, propertyName: 'segment', entityType: 'org', modelValue: 'hospital', text: 'hospital' },
              { key: 'b', propertyName: 'segment', entityType: 'lead', modelValue: 'gov', text: 'Gov' },
            ],
          },
        ],
      },
    })

    // Labelled by the property, not the relation.
    expect(wrapper.get('.field-label').text()).toBe('segment:')
    // The widget gets the related type, not the card's, for styles and formats.
    expect(wrapper.get('em').attributes('data-type')).toBe('org')
    expect(wrapper.get('.related-values').text()).toBe('hospitalGov')
  })

  it('joins related values with commas when none has a widget', () => {
    const value = (text: string) => ({ key: text, propertyName: 'name', entityType: 'org', modelValue: text, text })
    const wrapper = mount(CardFieldList, {
      props: {
        fields: [
          {
            field: { relation: 'belongs-to', property: 'name', show_label: false },
            text: 'Acme, Beta',
            values: [value('Acme'), value('Beta')],
          },
        ],
      },
    })
    expect(wrapper.get('.card-field').text()).toBe('Acme, Beta')
  })

  it('renders nothing at all when there are no fields', () => {
    const wrapper = mount(CardFieldList, { props: { fields: [] } })
    expect(wrapper.find('.card-fields').exists()).toBe(false)
  })
})
