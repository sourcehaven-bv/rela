import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlTextSelectionComment from './RlTextSelectionComment.vue'
import type { NewComment } from './types'

const prose = `Atlas replaces the three spreadsheets the team keeps in parallel today. The pilot covered one department; the rollout covers all of them, which is why the reporting requirements changed between the two.

Select any run of text in this paragraph to see the affordance appear beneath it.`

const meta: Meta<typeof RlTextSelectionComment> = {
  title: 'Comment/Text selection comment',
  component: RlTextSelectionComment,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlTextSelectionComment>

/**
 * Select five characters or more inside the passage. The component reports the
 * quote and the text either side of it; placing that in a document is the
 * caller's job, since only the caller has the source.
 */
export const Playground: Story = {
  render: () => ({
    components: { RlTextSelectionComment },
    setup() {
      const container = ref<HTMLElement | null>(null)
      const posted = ref<string[]>([])

      function add({ body, anchor }: NewComment) {
        posted.value = [...posted.value, `"${anchor.quote}" - ${body}`]
      }

      return { container, posted, add, prose }
    },
    template: `
      <div>
        <div ref="container" style="position:relative; max-width:560px; font-size:14px; line-height:1.65; white-space:pre-line">
          {{ prose }}
          <RlTextSelectionComment :container="container" @add="add" />
        </div>
        <ul v-if="posted.length" style="margin:24px 0 0; padding:0; list-style:none; font-size:13px">
          <li v-for="(entry, i) in posted" :key="i" style="margin-top:6px">{{ entry }}</li>
        </ul>
      </div>
    `,
  }),
}

/**
 * A selection that cannot be anchored, one spanning two table cells for
 * instance. The caller sets the reason, and the composer is never offered:
 * nobody should write a comment that was never going to save.
 */
export const Blocked: Story = {
  render: () => ({
    components: { RlTextSelectionComment },
    setup() {
      const container = ref<HTMLElement | null>(null)
      return { container, prose }
    },
    template: `
      <div ref="container" style="position:relative; max-width:560px; font-size:14px; line-height:1.65; white-space:pre-line">
        {{ prose }}
        <RlTextSelectionComment
          :container="container"
          blocked-reason="This selection crosses two cells"
        />
      </div>
    `,
  }),
}
