import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import RlAppShell from 'rela-components/components/layout/RlAppShell.vue'

// The read-only banner sits in the shell's banner slot: above the header and
// beside neither pane, so a sticky table header in the content pane cannot
// cover it and an open side panel cannot squeeze it (BUG-WS7QF2K). Above the
// header, so it is at the top on a list page as on an entity page, which has
// no header band (BUG-UX4P9T2).
describe('app shell banner slot', () => {
  function shell(slots: Record<string, string>) {
    return mount(RlAppShell, { props: { panelOpen: true }, slots })
  }

  it('renders the banner before the header and outside the content row', () => {
    const w = shell({
      header: '<div class="h">Header</div>',
      banner: '<div class="b">Read-only</div>',
      default: '<div class="c">Content</div>',
      panel: '<div class="p">Panel</div>',
    })
    const frame = w.get('.rl-app-shell__frame').element
    const children = Array.from(frame.children).map((el) => el.className)
    expect(children).toEqual(['rl-app-shell__banner', 'rl-app-shell__header', 'rl-app-shell__row'])
    expect(w.get('.rl-app-shell__banner').text()).toBe('Read-only')
    expect(w.get('.rl-app-shell__row').find('.b').exists()).toBe(false)
  })

  it('draws no banner box without the slot', () => {
    expect(shell({ default: '<div>Content</div>' }).find('.rl-app-shell__banner').exists()).toBe(
      false
    )
  })
})
