import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { computed, ref } from 'vue'
import RlCommentIndicator from './RlCommentIndicator.vue'
import RlFieldLabel from '../common/RlFieldLabel.vue'
import { anchoredComments } from '../../fixtures'
import type { AnchoredComment, NewComment } from './types'

const dueDateComments = anchoredComments.filter((c) => c.anchor.ref === 'dueDate')

const meta: Meta<typeof RlCommentIndicator> = {
  title: 'Comment/Comment indicator',
  component: RlCommentIndicator,
  parameters: { layout: 'centered' },
  args: {
    anchor: { kind: 'property', ref: 'dueDate', label: 'Due date' },
    comments: dueDateComments,
  },
}
export default meta
type Story = StoryObj<typeof RlCommentIndicator>

/** The badge and its popover. Click it to open the thread. */
export const Playground: Story = {}

/**
 * With nothing on the anchor the badge is invisible until the row is hovered
 * or it is focused, so a form of twenty fields is not a form of twenty badges.
 * Tab to it to see that it still reveals without a pointer.
 */
export const Empty: Story = {
  args: { comments: [] },
  render: (args) => ({
    components: { RlCommentIndicator, RlFieldLabel },
    setup: () => ({ args }),
    template: `
      <div class="rl-comment-hover-group" style="display:flex; align-items:center; gap:8px; padding:8px 12px; width:280px; border:1px dashed var(--rl-color-border)">
        <RlFieldLabel>Due date</RlFieldLabel>
        <RlCommentIndicator v-bind="args" />
        <span style="margin-left:auto; font-size:14px">24 Sep</span>
      </div>
    `,
  }),
}

/** Every comment settled: the badge turns to a check rather than disappearing. */
export const AllResolved: Story = {
  args: { comments: dueDateComments.map((c) => ({ ...c, resolved: true })) },
}

/** The property is gone. The comments stay, and the badge says so. */
export const Detached: Story = {
  args: {
    anchor: { kind: 'property', ref: 'legacyOwner', label: 'Legacy owner' },
    comments: anchoredComments.filter((c) => c.detached),
  },
}

/** A row of fields, each with its own anchor. */
export const OnAFieldRow: Story = {
  render: () => ({
    components: { RlCommentIndicator, RlFieldLabel },
    setup() {
      const comments = ref<AnchoredComment[]>([...anchoredComments])
      let next = 200
      const fields = [
        { ref: 'title', label: 'Title', value: 'Atlas mobile redesign' },
        { ref: 'dueDate', label: 'Due date', value: '24 Sep' },
        { ref: 'owner', label: 'Owner', value: 'Jeroen' },
      ]

      const forRef = computed(
        () => (reference: string) => comments.value.filter((c) => c.anchor.ref === reference),
      )

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

      return { fields, forRef, add, remove, toggleResolved }
    },
    template: `
      <div style="display:flex; gap:24px; width:560px">
        <div
          v-for="field in fields"
          :key="field.ref"
          class="rl-comment-hover-group"
          style="flex:1"
        >
          <div style="display:flex; align-items:center; gap:6px">
            <RlFieldLabel>{{ field.label }}</RlFieldLabel>
            <RlCommentIndicator
              :anchor="{ kind: 'property', ref: field.ref, label: field.label }"
              :comments="forRef(field.ref)"
              @add="add"
              @remove="remove"
              @toggle-resolved="toggleResolved"
            />
          </div>
          <p style="margin:4px 0 0; font-size:14px">{{ field.value }}</p>
        </div>
      </div>
    `,
  }),
}
