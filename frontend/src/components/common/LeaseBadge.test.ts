import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LeaseBadge from './LeaseBadge.vue'
import type { EntityLease } from '@/types'

// The holder writes the status, and anyone who can read the entity sees it.
// It is text: markup in it must show as the characters typed, in the badge
// and in its title, and never become an element (TKT-IMBF7U).

const lease = (over: Partial<EntityLease> = {}): EntityLease => ({
  user: 'bob@example.com',
  tool: 'mcp',
  since: '2026-10-12T10:00:00Z',
  ...over,
})

describe('LeaseBadge', () => {
  it('renders nothing without a lease', () => {
    expect(mount(LeaseBadge).html()).toBe('<!--v-if-->')
  })

  it.each([
    ['the label', lease({ label: 'triage' }), 'Claimed by bob@example.com (triage)'],
    ['the tool when there is no label', lease(), 'Claimed by bob@example.com (mcp)'],
    ['only the user when there is neither', lease({ tool: '' }), 'Claimed by bob@example.com'],
  ])('names %s', (_name, l, text) => {
    expect(mount(LeaseBadge, { props: { lease: l } }).text()).toBe(text)
  })

  it('puts the status in the title as text', () => {
    const status = '<img src=x onerror="alert(1)"><b>bold</b>'
    const w = mount(LeaseBadge, { props: { lease: lease({ label: 'a', status }) } })
    expect(w.attributes('title')).toBe(`Claimed by bob@example.com (a)\n${status}`)
    expect(w.find('img').exists()).toBe(false)
    expect(w.find('b').exists()).toBe(false)
  })

  it('renders markup in the user and label literally', () => {
    const w = mount(LeaseBadge, {
      props: { lease: lease({ user: '<i>eve</i>', label: '<script>x</script>' }) },
    })
    expect(w.text()).toBe('Claimed by <i>eve</i> (<script>x</script>)')
    expect(w.find('i').exists()).toBe(false)
    expect(w.find('script').exists()).toBe(false)
  })

  it('uses the badge text as the title when there is no status', () => {
    const w = mount(LeaseBadge, { props: { lease: lease({ label: 'a' }) } })
    expect(w.attributes('title')).toBe('Claimed by bob@example.com (a)')
  })
})
