import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlToastHost from './RlToastHost.vue'
import RlToast from './RlToast.vue'
import RlButton from '../../components/common/RlButton.vue'
import { useToasts } from './useToasts'

const meta: Meta<typeof RlToast> = {
  title: 'Feedback/Toast',
  component: RlToast,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlToast>

/**
 * Raise a toast from anywhere with `useToasts()`; the host renders them.
 *
 * The timer pauses while the pointer is over a toast or focus is inside it,
 * so a message never disappears mid-read (WCAG 2.2.1).
 */
export const Playground: Story = {
  render: () => ({
    components: { RlToastHost, RlButton },
    setup() {
      const { toasts, dismiss, success, danger, warning, info } = useToasts()
      return { toasts, dismiss, success, danger, warning, info }
    },
    template: `
      <div style="display:flex; flex-wrap:wrap; gap:8px">
        <RlButton variant="secondary" @click="success('Task created', 'Atlas mobile redesign is on the board.')">
          Success
        </RlButton>
        <RlButton variant="secondary" @click="info('Link copied')">Info</RlButton>
        <RlButton variant="secondary" @click="warning('Sync is behind', 'Last updated 2 hours ago.')">
          Warning
        </RlButton>
        <RlButton variant="secondary" tone="danger" @click="danger('Could not save', 'Your connection dropped.')">
          Error
        </RlButton>

        <RlToastHost :toasts="toasts" @dismiss="dismiss" />
      </div>
    `,
  }),
}

/** One toast on its own, so the anatomy is visible without a timer running. */
export const Single: Story = {
  render: () => ({
    components: { RlToast },
    setup: () => ({
      toast: {
        id: 'demo',
        title: 'Task created',
        description: 'Atlas mobile redesign is on the board.',
        tone: 'success' as const,
        duration: 0,
      },
    }),
    template: '<div style="max-width:380px"><RlToast :toast="toast" /></div>',
  }),
}
