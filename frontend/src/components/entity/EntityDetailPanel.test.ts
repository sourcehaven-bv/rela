import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import type { EntityAction } from './entityActions'

let emitActions: ((list: EntityAction[]) => void) | null = null
vi.mock('./EntityDetail.vue', () => ({
  default: defineComponent({
    name: 'EntityDetail',
    emits: ['actions'],
    setup(_, { emit }) {
      emitActions = (list) => emit('actions', list)
      return () => h('div', { class: 'stub-detail' })
    },
  }),
}))

import EntityDetailPanel from './EntityDetailPanel.vue'

// The panel hides the detail's header buttons and draws the same actions in
// its toolbar's "⋯" menu, in place of the library's own inert button.
describe('EntityDetailPanel', () => {
  let w: VueWrapper | null = null
  afterEach(() => {
    w?.unmount()
    document.body.innerHTML = ''
  })

  it('draws the detail actions in its menu and runs them', async () => {
    w = mount(EntityDetailPanel, {
      props: { entityType: 'beleid', entityId: 'POL-1' },
      attachTo: document.body,
    })
    expect(w.find('[data-testid="entity-actions-menu"]').exists()).toBe(false)
    expect(w.find('button[aria-label="More options"]').exists()).toBe(false)

    const run = vi.fn()
    emitActions!([
      { id: 'delete', label: 'Delete', group: 'danger', tone: 'danger', run: vi.fn() },
      { id: 'copy:vaststellen', label: 'Vaststellen', group: 'command', run },
    ])
    await flushPromises()
    await w.get('[data-testid="entity-actions-menu"]').trigger('click')
    const rows = [...document.body.querySelectorAll('[data-action]')]
    expect(rows.map((r) => r.getAttribute('data-action'))).toEqual(['copy:vaststellen', 'delete'])
    ;(rows[0] as HTMLElement).click()
    expect(run).toHaveBeenCalled()
  })
})
