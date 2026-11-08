// @vitest-environment jsdom
// DOMPurify under happy-dom returns some hostile input unsanitized, so a
// sanitization assertion only means something in jsdom.
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import MarkdownHelp from './MarkdownHelp.vue'

describe('MarkdownHelp', () => {
  it('strips script elements and event handlers', () => {
    const w = mount(MarkdownHelp, {
      props: { text: '<img src="x" onerror="alert(1)"><script>alert(2)</script>ok' },
    })
    expect(w.find('script').exists()).toBe(false)
    expect(w.find('img').attributes('onerror')).toBeUndefined()
    expect(w.text()).toContain('ok')
  })

  it('drops javascript: link targets', () => {
    const w = mount(MarkdownHelp, { props: { text: '[click](javascript:alert(1))' } })
    expect(w.find('a').attributes('href')).toBeUndefined()
  })

  it('renders plain text as one paragraph', () => {
    const w = mount(MarkdownHelp, { props: { text: 'Just text.' } })
    expect(w.findAll('p')).toHaveLength(1)
  })
})
