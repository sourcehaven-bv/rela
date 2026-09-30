import { ref } from 'vue'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlButton from './RlButton.vue'
import RlButtonGroup from './RlButtonGroup.vue'

const meta: Meta<typeof RlButton> = {
  title: 'Common/Button',
  component: RlButton,
  parameters: { layout: 'padded' },
  argTypes: {
    variant: {
      control: { type: 'inline-radio' },
      options: ['primary', 'secondary', 'subtle', 'ghost'],
    },
    tone: { control: { type: 'inline-radio' }, options: ['default', 'danger'] },
    size: { control: { type: 'inline-radio' }, options: ['sm', 'md', 'lg'] },
    icon: {
      control: { type: 'select' },
      options: [undefined, 'plus', 'check', 'trash', 'close', 'link'],
    },
    loading: { control: 'boolean' },
    disabled: { control: 'boolean' },
    block: { control: 'boolean' },
  },
  args: {
    variant: 'primary',
    tone: 'default',
    size: 'md',
    loading: false,
    disabled: false,
    block: false,
  },
  render: (args) => ({
    components: { RlButton },
    setup: () => ({ args }),
    template: '<RlButton v-bind="args">Create</RlButton>',
  }),
}
export default meta
type Story = StoryObj<typeof RlButton>

export const Playground: Story = {}

const row = 'display:flex; gap:12px; align-items:center; flex-wrap:wrap'

export const Variants: Story = {
  render: () => ({
    components: { RlButton },
    template: `
      <div style="${row}">
        <RlButton variant="primary">Primary</RlButton>
        <RlButton variant="secondary">Secondary</RlButton>
        <RlButton variant="subtle">Subtle</RlButton>
        <RlButton variant="ghost">Ghost</RlButton>
        <RlButton variant="primary" disabled>Disabled</RlButton>
      </div>
    `,
  }),
}

/** Every variant also has a destructive tone, for delete and remove actions. */
export const Destructive: Story = {
  render: () => ({
    components: { RlButton },
    template: `
      <div style="${row}">
        <RlButton variant="primary" tone="danger" icon="trash-2">Delete task</RlButton>
        <RlButton variant="secondary" tone="danger">Delete</RlButton>
        <RlButton variant="subtle" tone="danger">Delete</RlButton>
        <RlButton variant="ghost" tone="danger" icon="trash-2">Remove</RlButton>
      </div>
    `,
  }),
}

export const Sizes: Story = {
  render: () => ({
    components: { RlButton },
    template: `
      <div style="${row}">
        <RlButton variant="primary" size="sm">Small</RlButton>
        <RlButton variant="primary" size="md">Medium</RlButton>
        <RlButton variant="primary" size="lg">Large</RlButton>
      </div>
    `,
  }),
}

export const WithIcons: Story = {
  render: () => ({
    components: { RlButton },
    template: `
      <div style="${row}">
        <RlButton variant="primary" icon="plus">New task</RlButton>
        <RlButton variant="secondary" icon="link">Copy link</RlButton>
        <RlButton variant="ghost" icon="done">Mark done</RlButton>
      </div>
    `,
  }),
}

/** A loading button stays disabled and announces itself with `aria-busy`. */
export const Loading: Story = {
  render: () => ({
    components: { RlButton },
    template: `
      <div style="${row}">
        <RlButton variant="primary" loading loading-label="Saving">Save</RlButton>
        <RlButton variant="secondary" loading>Save</RlButton>
        <RlButton variant="primary" tone="danger" loading loading-label="Deleting">Delete</RlButton>
      </div>
    `,
  }),
}

/** The usual pairings. The confirming action is last in the DOM. */
export const ActionPairs: Story = {
  name: 'Action pairs',
  render: () => ({
    components: { RlButton, RlButtonGroup },
    template: `
      <div style="display:flex; flex-direction:column; gap:20px; max-width:420px">
        <RlButtonGroup>
          <RlButton variant="ghost">Cancel</RlButton>
          <RlButton variant="primary">Create task</RlButton>
        </RlButtonGroup>

        <RlButtonGroup>
          <RlButton variant="ghost">Cancel</RlButton>
          <RlButton variant="primary" tone="danger">Delete task</RlButton>
        </RlButtonGroup>

        <RlButtonGroup align="between">
          <RlButton variant="ghost" tone="danger" icon="trash-2">Delete</RlButton>
          <RlButton variant="primary">Save changes</RlButton>
        </RlButtonGroup>
      </div>
    `,
  }),
}

/**
 * The explicit-action pending indicator. Click Save and watch the width: both
 * labels are laid out in one cell, so the longer "Saving" does not widen the
 * button under the cursor.
 *
 * The swap is also held back half a second, so an action that finishes quickly
 * shows nothing at all. Use "Save (fast)" to see that: the request resolves
 * before the delay elapses, so the label never changes.
 */
export const Pending: Story = {
  render: () => ({
    components: { RlButton },
    setup() {
      const slow = ref(false)
      const fast = ref(false)
      // The template unwraps refs, so the handler names which one to set
      // rather than being handed the ref itself.
      const run = (which: 'slow' | 'fast', ms: number) => {
        const flag = which === 'slow' ? slow : fast
        flag.value = true
        setTimeout(() => (flag.value = false), ms)
      }
      return { slow, fast, run }
    },
    template: `
      <div style="display:flex; gap:12px; align-items:center">
        <RlButton
          variant="primary"
          pending-label="Saving"
          :loading="slow"
          @click="run('slow', 2000)"
        >Save</RlButton>
        <RlButton
          variant="secondary"
          pending-label="Saving"
          :loading="fast"
          @click="run('fast', 120)"
        >Save (fast)</RlButton>
      </div>
    `,
  }),
}

/**
 * A navigation action is a link, not a button.
 *
 * `as` swaps the element while keeping the variant, size and tone. Rendering
 * a real anchor is what gives the user the middle click, the modifier click
 * and the status-bar preview; a button with a programmatic push has none of
 * them. Pass a router link component the same way.
 */
export const AsLink: Story = {
  render: (args) => ({
    components: { RlButton },
    setup: () => ({ args }),
    template: `
      <div style="display:flex; gap:12px; align-items:center">
        <RlButton v-bind="args" as="a" href="#edit" variant="secondary">Edit</RlButton>
        <RlButton v-bind="args" as="a" href="#history" variant="ghost" icon="clock">History</RlButton>
        <RlButton v-bind="args" variant="secondary">A real button, for comparison</RlButton>
      </div>
    `,
  }),
}
