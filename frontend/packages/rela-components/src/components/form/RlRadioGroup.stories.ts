import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import { ref } from 'vue'
import RlRadioGroup from './RlRadioGroup.vue'

const meta: Meta<typeof RlRadioGroup> = {
  title: 'Form/Radio group',
  component: RlRadioGroup,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlRadioGroup>

const plans = [
  { value: 'solo', label: 'Solo', description: 'One editor. Unlimited read-only viewers.' },
  { value: 'team', label: 'Team', description: 'Up to twenty editors, with shared templates.' },
  { value: 'org', label: 'Organisation', description: 'SSO, audit log and a shared retention policy.' },
  { value: 'legacy', label: 'Legacy', description: 'No longer offered to new workspaces.', disabled: true },
]

/**
 * The shape this exists for: three to six options that need explaining. A
 * segmented control has no room for the descriptions and a select hides them
 * until the list is open, which is exactly when the user is choosing.
 */
export const Cards: Story = {
  render: () => ({
    components: { RlRadioGroup },
    setup: () => ({ value: ref('team'), plans }),
    template: `
      <div style="max-width:520px">
        <RlRadioGroup v-model="value" label="Plan" :options="plans" variant="card" />
      </div>
    `,
  }),
}

/** Short labels, no descriptions: the boxes would be louder than the question. */
export const Plain: Story = {
  args: {
    label: 'Visibility',
    modelValue: 'workspace',
    options: [
      { value: 'private', label: 'Only me' },
      { value: 'workspace', label: 'Everyone in the workspace' },
      { value: 'public', label: 'Anyone with the link' },
    ],
  },
}

/** Two or three short labels can share a row. Described options cannot. */
export const Inline: Story = {
  args: {
    label: 'Sort direction',
    modelValue: 'asc',
    inline: true,
    options: [
      { value: 'asc', label: 'Ascending' },
      { value: 'desc', label: 'Descending' },
    ],
  },
}

/** The error belongs to the choice, so it is announced once, not per option. */
export const WithError: Story = {
  args: {
    label: 'Plan',
    options: plans,
    error: 'Choose a plan to continue.',
    required: true,
  },
}

/**
 * Native radios in a native group, which is the whole reason not to rebuild
 * this from divs: one Tab stop, arrows between the options, and a disabled
 * option that the arrows skip.
 */
export const KeyboardAndSelection: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    const team = canvas.getByRole('radio', { name: /Team/ })
    await expect(team).toBeChecked()

    /*
     * Asserted on the group rather than the options: the description and the
     * error are about the choice, so they hang off the radiogroup and a
     * reader hears them once.
     */
    await expect(canvas.getByRole('radiogroup')).toBeInTheDocument()

    await userEvent.click(canvas.getByRole('radio', { name: /Organisation/ }))
    await expect(canvas.getByRole('radio', { name: /Organisation/ })).toBeChecked()
    await expect(team).not.toBeChecked()

    /* Disabled stays unreachable by click as well as by keyboard. */
    const legacy = canvas.getByRole('radio', { name: /Legacy/ })
    await expect(legacy).toBeDisabled()
  },
  render: () => ({
    components: { RlRadioGroup },
    setup: () => ({ value: ref('team'), plans }),
    template: `
      <div style="max-width:520px">
        <RlRadioGroup v-model="value" label="Plan" :options="plans" variant="card" />
      </div>
    `,
  }),
}
