import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
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
  setup: (props) => () => h('em', String(props.modelValue)),
})

describe('cardFieldLabel', () => {
  it('derives the label from the field rather than making an author restate it', () => {
    expect(cardFieldLabel({ property: 'assignee' })).toBe('assignee')
    expect(cardFieldLabel({ relation: 'belongs-to' })).toBe('belongs-to')
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
})

describe('CardFieldList', () => {
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

  it('renders nothing at all when there are no fields', () => {
    const wrapper = mount(CardFieldList, { props: { fields: [] } })
    expect(wrapper.find('.card-fields').exists()).toBe(false)
  })
})
