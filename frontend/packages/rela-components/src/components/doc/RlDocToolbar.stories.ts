import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlDocToolbar from './RlDocToolbar.vue'

const meta: Meta<typeof RlDocToolbar> = {
  title: 'Doc/Doc toolbar',
  component: RlDocToolbar,
  parameters: { layout: 'padded' },
  argTypes: { starred: { control: 'boolean' } },
  args: { shareLabel: 'Share', starred: false },
}
export default meta
type Story = StoryObj<typeof RlDocToolbar>

export const Playground: Story = {}

export const Starred: Story = { args: { starred: true } }
