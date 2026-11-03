// TKT-3IBLIP: the conversion host sits beside the inline editor. These tests
// pin that it does not take the `v-else` of the editable/read pair, which
// rendered the read view twice on an editable body without offers.
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import EntityBody from './EntityBody.vue'
import type { TaskItemConversionOffer } from '@/types'

const offer: TaskItemConversionOffer = {
  name: 'subtask',
  label: 'Convert to task',
  to: 'task',
  item: 'link',
  allowed: true,
}

function mountBody(props: { editable: boolean } & Record<string, unknown>) {
  return mount(EntityBody, {
    props: { content: '- [ ] Write the migration', ...props },
    slots: { read: '<div data-testid="read">read view</div>' },
    global: {
      plugins: [createPinia()],
      stubs: { MarkdownEditor: true, TaskItemConversions: true },
    },
  })
}

describe('EntityBody', () => {
  it.each([
    ['read-only', { editable: false }],
    ['editable without offers', { editable: true, source: { id: 'TKT-1' } }],
    [
      'editable with offers',
      { editable: true, source: { id: 'TKT-1' }, taskItemConversions: [offer] },
    ],
  ])('renders the read view once when %s', (_name, props) => {
    const wrapper = mountBody(props)
    expect(wrapper.findAll('[data-testid="read"]')).toHaveLength(1)
  })

  it('mounts the conversion host only for an editable body with offers', () => {
    expect(
      mountBody({ editable: true, source: { id: 'TKT-1' } })
        .findComponent({ name: 'TaskItemConversions' })
        .exists()
    ).toBe(false)
    expect(
      mountBody({ editable: true, source: { id: 'TKT-1' }, taskItemConversions: [offer] })
        .findComponent({ name: 'TaskItemConversions' })
        .exists()
    ).toBe(true)
  })
})
