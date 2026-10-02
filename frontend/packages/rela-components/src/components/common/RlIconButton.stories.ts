import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlIconButton from './RlIconButton.vue'
import { iconNames } from './icons'

const meta: Meta<typeof RlIconButton> = {
  title: 'Common/Icon button',
  component: RlIconButton,
  parameters: { layout: 'padded' },
  argTypes: {
    icon: { control: { type: 'select' }, options: iconNames },
    size: { control: { type: 'range', min: 12, max: 32, step: 1 } },
    tone: { control: { type: 'inline-radio' }, options: ['default', 'danger'] },
    active: { control: 'boolean' },
    pressed: { control: 'boolean' },
  },
  args: { icon: 'star', label: 'Star', size: 18, active: false, tone: 'default' },
}
export default meta
type Story = StoryObj<typeof RlIconButton>

export const Playground: Story = {}

/**
 * `label` is never shown, but it is the button's accessible name, so it is
 * required rather than optional.
 */
export const Toolbar: Story = {
  render: () => ({
    components: { RlIconButton },
    template: `
      <div style="display:flex; gap:4px; align-items:center">
        <RlIconButton icon="star" label="Star" />
        <RlIconButton icon="link" label="Copy link" />
        <RlIconButton icon="paperclip" label="Attach" />
        <RlIconButton icon="maximize-2" label="Expand" />
        <RlIconButton icon="ellipsis" label="More options" />
        <RlIconButton icon="trash-2" label="Delete task" tone="danger" />
      </div>
    `,
  }),
}

/** `pressed` sets aria-pressed for toggles such as the star. */
export const Pressed: Story = {
  args: { icon: 'star', label: 'Star', active: true, pressed: true },
}

/** The destructive tone only colours on hover and focus, not at rest. */
export const Danger: Story = {
  args: { icon: 'trash-2', label: 'Delete task', tone: 'danger' },
}
