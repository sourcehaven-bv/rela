import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlThemeToggle from './RlThemeToggle.vue'

/**
 * Picks the colour theme. Three choices rather than a switch, because the
 * stylesheet defines three states: `system` is the absence of an override,
 * so it follows `prefers-color-scheme` until the user picks a side.
 *
 * It writes `.light` or `.dark` to the document root, which is what
 * `dark.css` selects on. Storing the choice is the app's job; bind
 * `v-model` to wherever the preference lives.
 */
const meta: Meta<typeof RlThemeToggle> = {
  title: 'Layout/Theme toggle',
  component: RlThemeToggle,
}

export default meta
type Story = StoryObj<typeof RlThemeToggle>

/** Uncontrolled: no `v-model`, so the control keeps the choice itself. */
export const Playground: Story = {}

/** Icons only, for a tight footer or a collapsed rail. */
export const IconOnly: Story = { args: { iconOnly: true } }

/** Already forced to dark, whatever the system is set to. */
export const Dark: Story = { args: { modelValue: 'dark' } }
