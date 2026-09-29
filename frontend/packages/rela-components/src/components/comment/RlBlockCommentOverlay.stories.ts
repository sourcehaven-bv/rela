import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { nextTick, onMounted, ref } from 'vue'
import RlBlockCommentOverlay, { type CommentableBlock } from './RlBlockCommentOverlay.vue'
import type { AnchoredComment, NewComment } from './types'

const meta: Meta<typeof RlBlockCommentOverlay> = {
  title: 'Comment/Block comment overlay',
  component: RlBlockCommentOverlay,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlBlockCommentOverlay>

/**
 * A badge over each block that has no text to select. The caller says which
 * elements are commentable and what key identifies them; the overlay measures
 * where they are and re-measures when the content reflows.
 */
export const Playground: Story = {
  render: () => ({
    components: { RlBlockCommentOverlay },
    setup() {
      const container = ref<HTMLElement | null>(null)
      const blocks = ref<CommentableBlock[]>([])
      const comments = ref<AnchoredComment[]>([
        {
          id: 'b1',
          author: 'Tess de Vries',
          timestamp: '19 Sep, 14:02',
          body: 'The arrow between these two steps points the wrong way.',
          anchor: { kind: 'block', ref: 'diagram-1', label: 'diagram' },
          editable: true,
          deletable: true,
        },
      ])
      let next = 300

      onMounted(async () => {
        await nextTick()
        const host = container.value
        if (!host) return
        blocks.value = [...host.querySelectorAll<HTMLElement>('[data-block]')].map((el) => ({
          key: el.dataset.block ?? '',
          kind: el.dataset.kind ?? 'block',
          el,
        }))
      })

      function add({ anchor, body }: NewComment) {
        comments.value = [
          ...comments.value,
          {
            id: `new-${next++}`,
            author: 'You',
            timestamp: 'just now',
            body,
            anchor,
            editable: true,
            deletable: true,
          },
        ]
      }

      function remove(id: string) {
        comments.value = comments.value.filter((c) => c.id !== id)
      }

      function toggleResolved(id: string) {
        comments.value = comments.value.map((c) =>
          c.id === id ? { ...c, resolved: !c.resolved } : c,
        )
      }

      return { container, blocks, comments, add, remove, toggleResolved }
    },
    template: `
      <div ref="container" style="position:relative; max-width:560px">
        <p style="font-size:14px; line-height:1.65">
          The rollout has two stages. The diagram below shows the order they run in.
        </p>

        <div
          data-block="diagram-1"
          data-kind="diagram"
          style="height:120px; display:grid; place-items:center; border:1px solid var(--rl-color-border); border-radius:8px; background:var(--rl-color-bg-sunken); font-size:13px; color:var(--rl-color-text-muted)"
        >
          Pilot -> Rollout
        </div>

        <p style="font-size:14px; line-height:1.65; margin-top:16px">
          And the screen the first stage ends on:
        </p>

        <div
          data-block="image-1"
          data-kind="image"
          style="height:90px; border:1px solid var(--rl-color-border); border-radius:8px; background:var(--rl-color-bg-hover)"
        ></div>

        <RlBlockCommentOverlay
          :container="container"
          :blocks="blocks"
          :comments="comments"
          @add="add"
          @remove="remove"
          @toggle-resolved="toggleResolved"
        />
      </div>
    `,
  }),
}
