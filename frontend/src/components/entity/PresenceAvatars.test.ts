import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PresenceAvatars from './PresenceAvatars.vue'
import type { PresenceViewer } from '@/types'

const viewer = (user: string, tool = 'data-entry'): PresenceViewer => ({
  user,
  tool,
  since: '2026-10-12T10:00:00Z',
})

describe('PresenceAvatars', () => {
  it('renders nothing when nobody else is viewing', () => {
    const w = mount(PresenceAvatars, { props: { viewers: [] } })
    expect(w.find('[data-testid="presence-avatars"]').exists()).toBe(false)
  })

  it('titles each avatar with user and tool', () => {
    const w = mount(PresenceAvatars, {
      props: { viewers: [viewer('alice@example.com'), viewer('bot', '')] },
    })
    const titles = w.findAll('.presence-avatar').map((a) => a.attributes('title'))
    expect(titles).toEqual(['alice@example.com (data-entry)', 'bot'])
    expect(w.attributes('aria-label')).toBe('Also viewing: alice@example.com (data-entry), bot')
  })

  it('marks an agent with a badge', () => {
    const w = mount(PresenceAvatars, { props: { viewers: [viewer('a'), viewer('b', 'mcp')] } })
    const avatars = w.findAll('.presence-avatar')
    expect(avatars[0]!.find('.presence-avatar__agent').exists()).toBe(false)
    expect(avatars[1]!.find('.presence-avatar__agent').exists()).toBe(true)
  })

  it('shows at most max avatars and folds the rest and the server overflow into +N', () => {
    const viewers = ['a', 'b', 'c', 'd', 'e', 'f', 'g'].map((u) => viewer(u))
    const w = mount(PresenceAvatars, { props: { viewers, more: 3, max: 5 } })
    expect(w.findAll('.presence-avatar')).toHaveLength(5)
    expect(w.find('.presence-avatars__more').text()).toBe('+5')
    expect(w.attributes('aria-label')).toBe(
      'Also viewing: a (data-entry), b (data-entry), c (data-entry), d (data-entry), ' +
        'e (data-entry), f (data-entry), g (data-entry) and 3 more'
    )
  })

  it('shows 5 avatars by default and +N for the server maximum of 20 plus more', () => {
    const viewers = Array.from({ length: 20 }, (_, i) => viewer(`user${i}`))
    const w = mount(PresenceAvatars, { props: { viewers, more: 3 } })
    expect(w.findAll('.presence-avatar')).toHaveLength(5)
    expect(w.find('.presence-avatars__more').text()).toBe('+18')
  })

  it('announces an agent and tells one person in two tools apart', () => {
    const w = mount(PresenceAvatars, {
      props: { viewers: [viewer('bob'), viewer('bob', 'mcp')] },
    })
    expect(w.attributes('aria-label')).toBe('Also viewing: bob (data-entry), bob (mcp)')
  })

  it('shows +N from more alone', () => {
    const w = mount(PresenceAvatars, { props: { viewers: [viewer('a')], more: 2 } })
    expect(w.findAll('.presence-avatar')).toHaveLength(1)
    expect(w.find('.presence-avatars__more').text()).toBe('+2')
  })

  it('renders markup in a user name as text', () => {
    const w = mount(PresenceAvatars, { props: { viewers: [viewer('<img src=x>')] } })
    expect(w.find('img').exists()).toBe(false)
    expect(w.find('.presence-avatar').attributes('title')).toBe('<img src=x> (data-entry)')
  })
})
