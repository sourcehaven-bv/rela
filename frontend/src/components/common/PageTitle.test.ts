import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import PageTitle from './PageTitle.vue'

describe('PageTitle', () => {
  it('renders the title prop in the h1', () => {
    const w = mount(PageTitle, { props: { title: 'Dashboard' } })
    expect(w.find('h1').text()).toBe('Dashboard')
  })

  it('default slot overrides the title prop', () => {
    const w = mount(PageTitle, {
      props: { title: 'Ignored' },
      slots: { default: () => h('span', 'Custom') },
    })
    expect(w.find('h1').text()).toBe('Custom')
  })

  it('renders subtitle when provided', () => {
    const w = mount(PageTitle, { props: { title: 'X', subtitle: 'detail' } })
    expect(w.find('.page-title__subtitle').exists()).toBe(true)
    expect(w.find('.page-title__subtitle').text()).toBe('detail')
  })

  it('omits subtitle node when prop is absent', () => {
    const w = mount(PageTitle, { props: { title: 'X' } })
    expect(w.find('.page-title__subtitle').exists()).toBe(false)
  })
})
