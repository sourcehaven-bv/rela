import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlBanner from './RlBanner.vue'
import RlButton from '../../components/common/RlButton.vue'

const meta: Meta<typeof RlBanner> = {
  title: 'Feedback/Banner',
  component: RlBanner,
  parameters: { layout: 'padded' },
  argTypes: {
    tone: { control: { type: 'inline-radio' }, options: ['info', 'success', 'warning', 'danger'] },
  },
  args: { tone: 'info', title: 'Read-only project', dismissible: false },
  render: (args) => ({
    components: { RlBanner },
    setup: () => ({ args }),
    template: `
      <RlBanner v-bind="args">
        You can view this project but not change it. Ask an owner for edit access.
      </RlBanner>
    `,
  }),
}
export default meta
type Story = StoryObj<typeof RlBanner>

/** Stays until the condition behind it changes, unlike a toast. */
export const Playground: Story = {}

/**
 * Each tone pairs a colour with an icon and its own wording, so the meaning
 * never rests on hue alone.
 */
export const Tones: Story = {
  render: () => ({
    components: { RlBanner },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px">
        <RlBanner tone="info" title="Read-only project">Ask an owner for edit access.</RlBanner>
        <RlBanner tone="success" title="Import finished">248 tasks were added.</RlBanner>
        <RlBanner tone="warning" title="Sync is behind">Last updated 2 hours ago.</RlBanner>
        <RlBanner tone="danger" title="Could not save">Your connection dropped. Retrying.</RlBanner>
      </div>
    `,
  }),
}

/** An action belongs in the banner when the fix is one click away. */
export const WithAction: Story = {
  render: () => ({
    components: { RlBanner, RlButton },
    template: `
      <RlBanner tone="warning" title="Sync is behind">
        Last updated 2 hours ago.
        <template #actions>
          <RlButton size="sm" variant="secondary">Sync now</RlButton>
        </template>
      </RlBanner>
    `,
  }),
}

export const Dismissible: Story = { args: { dismissible: true, tone: 'success', title: 'Import finished' } }
