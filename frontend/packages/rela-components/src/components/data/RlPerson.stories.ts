import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlPerson from './RlPerson.vue'
import type { Person } from './types'

const meta: Meta<typeof RlPerson> = {
  title: 'Data/Person',
  component: RlPerson,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlPerson>

const ada: Person = { id: '1', name: 'Ada Lovelace', secondary: 'Engineering' }

/**
 * The avatar-and-name pair, in one shape. Every place that mentions a person
 * had been building its own, which is how one of them ended up with a name
 * and no avatar and another with an avatar and no accessible name.
 */
export const Default: Story = {
  render: () => ({
    components: { RlPerson },
    setup: () => ({ ada }),
    template: `
      <div style="display:flex; flex-direction:column; gap:12px; align-items:flex-start">
        <RlPerson :person="ada" />
        <RlPerson :person="ada" secondary="ada@example.com" />
        <RlPerson :person="ada" size="lg" />
        <RlPerson />
      </div>
    `,
  }),
}

/**
 * Unassigned is a state, not an absence. A row with nothing in it reads as a
 * rendering fault; a dimmed word says the field was looked at and is empty.
 */
export const Unassigned: Story = { args: {} }

/**
 * Without the name, the person moves onto the wrapper as an image label, so
 * they are still announced once. With it, the avatar is decorative and the
 * written name does the announcing.
 */
export const AnnouncedOnce: Story = {
  render: () => ({
    components: { RlPerson },
    setup: () => ({ ada }),
    template: `
      <div style="display:flex; gap:16px">
        <RlPerson :person="ada" />
        <RlPerson :person="ada" name-hidden />
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    // The hidden-name form names itself; the written one is named by its text.
    const marker = canvas.getByRole('img', { name: 'Ada Lovelace' })
    await expect(marker).toBeInTheDocument()

    await expect(canvas.getAllByText('Ada Lovelace')).toHaveLength(1)
  },
}
