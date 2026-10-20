import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlChannelAuthor from './RlChannelAuthor.vue'

const meta: Meta<typeof RlChannelAuthor> = {
  title: 'Channel/Channel author',
  component: RlChannelAuthor,
  parameters: { layout: 'padded' },
  args: { author: { name: 'Marieke de Vries' } },
}
export default meta
type Story = StoryObj<typeof RlChannelAuthor>

/** A person: initials and name, no badge. */
export const Person: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Marieke de Vries')).toBeVisible()
    await expect(canvas.queryByText('system')).toBeNull()
    await expect(canvas.queryByText('agent')).toBeNull()
  },
}

/** An agent keeps the person avatar and gets a badge saying what it is. */
export const Agent: Story = {
  args: { author: { name: 'Triage agent', kind: 'agent' } },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('agent')).toBeVisible()
  },
}

/**
 * A message the server posted itself, such as a trigger's, gets a gear and a
 * system badge so it never reads as a colleague's.
 */
export const System: Story = {
  args: { author: { name: 'rela', kind: 'system' } },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('system')).toBeVisible()
  },
}

/** The compact form used on an ephemeral line: no avatar. */
export const Compact: Story = {
  args: { compact: true },
}
