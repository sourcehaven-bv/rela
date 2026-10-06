import { afterEach, describe, expect, it } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'

// Pressing a menu item must not move focus. WebKit (Safari, the desktop app)
// does not focus a link on press, so focus left the menu with no
// relatedTarget, the menu closed on focusout, and the click never landed:
// the space switcher opened but no item switched.
const Host = defineComponent({
  setup() {
    return () =>
      h(RlMenu, null, {
        trigger: ({ toggle, attrs }: { toggle: () => void; attrs: Record<string, unknown> }) =>
          h('button', { ...attrs, 'data-testid': 'trigger', onClick: toggle }, 'Open'),
        default: () => [
          h(RlMenuItem, { href: '/a' }, () => 'A'),
          h(RlMenuItem, { href: '/b' }, () => 'B'),
        ],
      })
  },
})

let wrapper: VueWrapper | undefined

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
})

describe('RlMenu', () => {
  it('cancels mousedown on an item so the press keeps focus and the menu open', async () => {
    wrapper = mount(Host, { attachTo: document.body })
    await wrapper.find('[data-testid="trigger"]').trigger('click')
    await flushPromises()

    const item = Array.from(document.body.querySelectorAll('[role="menu"] a')).find(
      (a) => a.textContent?.trim() === 'B',
    )
    expect(item).toBeDefined()
    const press = new MouseEvent('mousedown', { bubbles: true, cancelable: true })
    item!.dispatchEvent(press)

    expect(press.defaultPrevented).toBe(true)
    expect(document.body.querySelector('[role="menu"]')).not.toBeNull()
  })
})
