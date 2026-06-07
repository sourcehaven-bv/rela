import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import InaccessibleField from './InaccessibleField.vue'

describe('InaccessibleField', () => {
  it('renders the lock affordance with the generic tooltip when no reason', () => {
    const w = mount(InaccessibleField)
    const span = w.find('span.property-inaccessible')
    expect(span.exists()).toBe(true)
    expect(span.attributes('title')).toBe('inaccessible')
    expect(span.text()).toContain('inaccessible')
  })

  it('renders the git-crypt-specific tooltip', () => {
    const w = mount(InaccessibleField, { props: { reason: 'git-crypt' } })
    expect(w.find('span.property-inaccessible').attributes('title')).toBe(
      'git-crypt encrypted (run `git-crypt unlock` to read)'
    )
  })

  it('renders a reason-formatted tooltip for unknown reasons', () => {
    const w = mount(InaccessibleField, { props: { reason: 'permission-denied' } })
    expect(w.find('span.property-inaccessible').attributes('title')).toBe(
      'inaccessible (permission-denied)'
    )
  })
})
