import { defineComponent, h } from 'vue'

/**
 * An RlPopover that is always open and renders in place.
 *
 * The real one teleports its panel to `<body>` and mounts it only while open,
 * so a test of the controls INSIDE a panel (FilterBar's inputs) would have to
 * open it and then query the document. Tests about the controls swap this in
 * with `vi.mock('rela-components/components/overlay/RlPopover.vue', ...)` and
 * keep asserting through the wrapper. The open and close behaviour is the
 * library's own, and FilterBar.menu.test.ts covers it with the real one.
 */
export default defineComponent({
  name: 'RlPopover',
  props: { title: { type: String, default: '' }, align: { type: String, default: 'end' } },
  setup(_props, { slots, expose }) {
    expose({ open: () => {}, close: () => {}, toggle: () => {} })
    return () =>
      h('div', { class: 'rl-popover' }, [
        slots.trigger?.({ open: true, toggle: () => {}, attrs: {} }),
        slots.default?.({ close: () => {} }),
      ])
  },
})
