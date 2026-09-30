import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlDisclosure from './RlDisclosure.vue'

const meta: Meta<typeof RlDisclosure> = {
  title: 'Common/Disclosure',
  component: RlDisclosure,
  parameters: { layout: 'padded' },
  argTypes: { open: { control: 'boolean' } },
  args: { open: true, label: 'In progress' },
}
export default meta
type Story = StoryObj<typeof RlDisclosure>

export const Playground: Story = {}

export const BothStates: Story = {
  render: () => ({
    components: { RlDisclosure },
    template: `
      <div style="display:flex; gap:16px; align-items:center">
        <RlDisclosure :open="true" label="In progress" />
        <RlDisclosure :open="false" label="Backlog" />
      </div>
    `,
  }),
}

/** Wired to state so the caret animates on click. */
export const Interactive: Story = {
  render: () => ({
    components: { RlDisclosure },
    setup: () => ({ open: ref(true) }),
    template: `
      <div>
        <RlDisclosure :open="open" label="Toggle me" @toggle="open = !open" />
        <p v-if="open" style="margin-top:12px">Panel contents</p>
      </div>
    `,
  }),
}
