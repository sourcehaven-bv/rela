import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import PageLayout from './PageLayout.vue'

describe('PageLayout', () => {
  it('renders the required topbar slot', () => {
    const w = mount(PageLayout, {
      slots: {
        topbar: () => h('h1', 'My Page'),
      },
    })
    expect(w.find('.page-layout__topbar-main').text()).toBe('My Page')
  })

  it('renders the actions slot only when content is provided', () => {
    const without = mount(PageLayout, {
      slots: { topbar: () => h('h1', 'X') },
    })
    expect(without.find('.page-layout__topbar-actions').exists()).toBe(false)

    const withActions = mount(PageLayout, {
      slots: {
        topbar: () => h('h1', 'X'),
        actions: () => h('button', 'Refresh'),
      },
    })
    expect(withActions.find('.page-layout__topbar-actions').exists()).toBe(true)
    expect(withActions.find('.page-layout__topbar-actions').text()).toBe('Refresh')
  })

  it('renders scope-nav slot only when provided', () => {
    const without = mount(PageLayout, {
      slots: { topbar: () => h('h1', 'X') },
    })
    expect(without.find('.page-layout__scope-nav').exists()).toBe(false)

    const withScope = mount(PageLayout, {
      slots: {
        topbar: () => h('h1', 'X'),
        'scope-nav': () => h('a', '← Back'),
      },
    })
    expect(withScope.find('.page-layout__scope-nav').exists()).toBe(true)
  })

  it('renders default content slot', () => {
    const w = mount(PageLayout, {
      slots: {
        topbar: () => h('h1', 'X'),
        default: () => h('p', 'page body'),
      },
    })
    expect(w.find('.page-layout__content').text()).toBe('page body')
  })

  it('renders actionbar only when slot is provided', () => {
    const without = mount(PageLayout, {
      slots: { topbar: () => h('h1', 'X') },
    })
    expect(without.find('.page-layout__actionbar').exists()).toBe(false)

    const withBar = mount(PageLayout, {
      slots: {
        topbar: () => h('h1', 'X'),
        actionbar: () => h('button', 'Save'),
      },
    })
    expect(withBar.find('.page-layout__actionbar').exists()).toBe(true)
    expect(withBar.find('.page-layout__actionbar').text()).toBe('Save')
  })

  it('applies --fixed modifier class when actionbarFixed prop is true', () => {
    const sticky = mount(PageLayout, {
      props: { actionbarFixed: false },
      slots: {
        topbar: () => h('h1', 'X'),
        actionbar: () => h('button', 'Save'),
      },
    })
    expect(sticky.find('.page-layout__actionbar').classes()).not.toContain(
      'page-layout__actionbar--fixed',
    )

    const fixed = mount(PageLayout, {
      props: { actionbarFixed: true },
      slots: {
        topbar: () => h('h1', 'X'),
        actionbar: () => h('button', 'Save'),
      },
    })
    expect(fixed.find('.page-layout__actionbar').classes()).toContain(
      'page-layout__actionbar--fixed',
    )
  })
})
