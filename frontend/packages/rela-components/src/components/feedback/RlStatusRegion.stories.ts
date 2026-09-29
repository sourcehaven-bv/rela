import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlStatusRegion from './RlStatusRegion.vue'
import RlButton from '../../components/common/RlButton.vue'

const meta: Meta<typeof RlStatusRegion> = {
  title: 'Feedback/Status region',
  component: RlStatusRegion,
  parameters: { layout: 'padded' },
  argTypes: {
    tone: { control: { type: 'inline-radio' }, options: ['pending', 'error', 'info'] },
    size: { control: { type: 'inline-radio' }, options: ['sm', 'md'] },
  },
  args: { tone: 'pending', size: 'md' },
}
export default meta
type Story = StoryObj<typeof RlStatusRegion>

/**
 * A pane that has not loaded yet. The spinner comes with the `pending` tone,
 * because a wait with no motion reads as a screen that is simply empty.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlStatusRegion },
    setup: () => ({ args }),
    template: `
      <div style="height:280px; display:flex; border:1px solid var(--rl-color-border); border-radius:8px">
        <RlStatusRegion v-bind="args">Loading tasks…</RlStatusRegion>
      </div>
    `,
  }),
}

/**
 * The pane failed. The way out is an action, so the message says what went
 * wrong and the action offers the retry rather than the message describing one.
 */
export const Failed: Story = {
  render: () => ({
    components: { RlStatusRegion, RlButton },
    template: `
      <div style="height:280px; display:flex; border:1px solid var(--rl-color-border); border-radius:8px">
        <RlStatusRegion tone="error">
          Could not load tasks. The workspace may be unreachable.
          <template #actions>
            <RlButton variant="secondary" icon="refresh">Try again</RlButton>
          </template>
        </RlStatusRegion>
      </div>
    `,
  }),
}

/**
 * Neither loading nor broken: the pane is working exactly as asked and has
 * nothing to show because nothing was selected. Announced to nobody, since it
 * describes the pane rather than reporting an event.
 */
export const Unavailable: Story = {
  render: () => ({
    components: { RlStatusRegion },
    template: `
      <div style="height:280px; display:flex; border:1px solid var(--rl-color-border); border-radius:8px">
        <RlStatusRegion tone="info">Select a relation to see its history.</RlStatusRegion>
      </div>
    `,
  }),
}

/**
 * The small size, for a side panel rather than a whole view. Same component:
 * a narrow pane needs the same three states as a wide one.
 */
export const InSidePanel: Story = {
  render: () => ({
    components: { RlStatusRegion },
    template: `
      <div style="width:280px; height:200px; display:flex; border:1px solid var(--rl-color-border); border-radius:8px">
        <RlStatusRegion size="sm">Loading documents…</RlStatusRegion>
      </div>
    `,
  }),
}

/**
 * Naming an icon replaces the tone's own mark, which also turns the spinner
 * off: for a wait better described by what it is waiting on than by motion.
 */
export const NamedIcon: Story = {
  render: () => ({
    components: { RlStatusRegion },
    template: `
      <div style="height:280px; display:flex; border:1px solid var(--rl-color-border); border-radius:8px">
        <RlStatusRegion tone="pending" icon="clock">
          Analysing this document. This usually takes a minute.
        </RlStatusRegion>
      </div>
    `,
  }),
}
