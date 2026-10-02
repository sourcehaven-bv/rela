import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSkeleton from './RlSkeleton.vue'

const meta: Meta<typeof RlSkeleton> = {
  title: 'Feedback/Skeleton',
  component: RlSkeleton,
  parameters: { layout: 'padded' },
  argTypes: {
    variant: { control: { type: 'inline-radio' }, options: ['text', 'block', 'circle'] },
  },
  args: { variant: 'text', lines: 1 },
  render: (args) => ({
    components: { RlSkeleton },
    setup: () => ({ args }),
    template: '<div style="max-width:360px"><RlSkeleton v-bind="args" /></div>',
  }),
}
export default meta
type Story = StoryObj<typeof RlSkeleton>

/**
 * Preferred over a spinner where the shape of what is coming is known: it
 * reserves the space, so the page does not jump when the data lands.
 */
export const Playground: Story = {}

/** The last line is short, as a real paragraph would be. */
export const Paragraph: Story = { args: { lines: 3 } }

/** A loading list row, matching the shape of the real one. */
export const ListRows: Story = {
  render: () => ({
    components: { RlSkeleton },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px; max-width:420px" aria-busy="true">
        <div v-for="row in 3" :key="row" style="display:flex; align-items:center; gap:12px">
          <RlSkeleton variant="circle" />
          <div style="flex:1">
            <RlSkeleton variant="text" width="70%" />
          </div>
        </div>
      </div>
    `,
  }),
}
