import { defineComponent, h } from 'vue'
import { usePageHeaderOutlet } from './usePageHeader'

/**
 * Test host that renders a view together with its hoisted page header.
 *
 * In the running app the header band is painted by RlAppShell, two levels
 * above the routed view. A unit test that mounts the view alone therefore
 * renders no header at all, and an assertion about the search box or the
 * create button finds nothing — not because the view is broken, but because
 * the test is missing the half of the app that paints it.
 *
 * Wrapping the view in this host restores that half, so a test keeps
 * asserting what the user would actually see. It deliberately renders the
 * header's slots directly rather than through RlPageHeader: the assertions
 * are about the view's own controls, and going through the library component
 * would make them depend on its internal markup.
 */
export function withPageHeader(view: unknown) {
  return defineComponent({
    name: 'PageHeaderTestHost',
    props: { listId: { type: String, required: false, default: undefined } },
    setup(props, { attrs }) {
      const outlet = usePageHeaderOutlet()
      return () => [
        outlet.content.value
          ? h('div', { class: 'page-header-outlet' }, [
              h('h1', outlet.content.value.title),
              ...(outlet.content.value.actions?.() ?? []),
              ...(outlet.content.value.tools?.() ?? []),
            ])
          : null,
        h(view as never, { ...attrs, ...(props.listId ? { listId: props.listId } : {}) }),
      ]
    },
  })
}
