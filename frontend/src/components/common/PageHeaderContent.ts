import { defineComponent, watchEffect, type Slot } from 'vue'
import { usePageHeader } from '@/composables/usePageHeader'

/**
 * Declares this screen's page header, which the app shell paints above both
 * the content and the detail panel.
 *
 * Renders nothing where it sits. A view writes its header as ordinary markup
 * in its own template:
 *
 *   <PageHeaderContent :title="listConfig.title">
 *     <template #actions>…</template>
 *     <template #tools><SearchBox ref="searchBoxRef" … /></template>
 *   </PageHeaderContent>
 *
 * and the shell renders those slots two levels up. Because the slots are
 * declared in the view, they render in the VIEW's scope: template refs
 * (`searchBoxRef`, `filterMenuRef`) and handlers resolve exactly as they did
 * when the header was inline, so a view's keyboard shortcuts keep working
 * across the move. See usePageHeader for why the header is hoisted at all.
 *
 * A render function rather than an SFC because it has no template of its own
 * to write, and an SFC with an empty root is not valid Vue.
 */
export default defineComponent({
  name: 'PageHeaderContent',
  props: {
    title: { type: String, required: true },
  },
  setup(props, { slots }) {
    const header = usePageHeader()

    // watchEffect rather than a one-shot call: the title is reactive (a
    // list's config arrives after its first render), and re-publishing is how
    // the band picks that up. usePageHeader clears on scope dispose, so
    // unmounting this component takes the header down with it.
    watchEffect(() => {
      header.show({
        title: props.title,
        actions: slots.actions as Slot | undefined,
        tools: slots.tools as Slot | undefined,
      })
    })

    return () => null
  },
})
