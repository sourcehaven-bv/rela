import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlCommentsPanel from './RlCommentsPanel.vue'
import { anchoredComments, commentAnchorOptions } from '../../fixtures'
import type { AnchoredComment, NewComment } from './types'

const meta: Meta<typeof RlCommentsPanel> = {
  title: 'Comment/Comments panel',
  component: RlCommentsPanel,
  parameters: { layout: 'padded' },
  args: {
    comments: anchoredComments,
    anchorOptions: commentAnchorOptions,
    defaultExpanded: true,
  },
}
export default meta
type Story = StoryObj<typeof RlCommentsPanel>

/**
 * Every comment on the record, each labelled with what it points at. Unresolved
 * ones sort first, so a settled remark never buries an active one.
 */
export const Playground: Story = {}

/**
 * Collapsed, which is the default wherever comments also appear beside the
 * fields they are about. The summary says how many have nowhere else to
 * appear, so the count is not lost when the panel is folded away.
 */
export const Collapsed: Story = {
  args: { defaultExpanded: false },
}

/** Nothing to show yet. The composer is still offered. */
export const Empty: Story = {
  args: { comments: [] },
}

/** Posting and resolving, wired up so the controls do what they say. */
export const Interactive: Story = {
  render: () => ({
    components: { RlCommentsPanel },
    setup() {
      const comments = ref<AnchoredComment[]>([...anchoredComments])
      let next = 100

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

      function edit(id: string, body: string) {
        comments.value = comments.value.map((c) => (c.id === id ? { ...c, body } : c))
      }

      function toggleResolved(id: string) {
        comments.value = comments.value.map((c) =>
          c.id === id ? { ...c, resolved: !c.resolved } : c,
        )
      }

      function remove(id: string) {
        comments.value = comments.value.filter((c) => c.id !== id)
      }

      return { comments, anchorOptions: commentAnchorOptions, add, edit, toggleResolved, remove }
    },
    template: `
      <RlCommentsPanel
        :comments="comments"
        :anchor-options="anchorOptions"
        default-expanded
        @add="add"
        @edit="edit"
        @toggle-resolved="toggleResolved"
        @remove="remove"
      />
    `,
  }),
}
