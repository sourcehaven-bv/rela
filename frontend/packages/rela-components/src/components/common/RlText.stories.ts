import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlText from './RlText.vue'

const meta: Meta<typeof RlText> = {
  title: 'Typography/Text',
  component: RlText,
  parameters: { layout: 'padded' },
  argTypes: {
    as: { control: { type: 'select' }, options: ['span', 'p', 'div', 'dd'] },
    size: { control: { type: 'select' }, options: ['xs', 'sm', 'md', 'lg'] },
    tone: { control: { type: 'select' }, options: ['default', 'muted', 'subtle'] },
    weight: { control: { type: 'select' }, options: ['normal', 'medium', 'semibold'] },
    truncate: { control: 'boolean' },
  },
  args: { as: 'p', size: 'md', tone: 'default', weight: 'normal', truncate: false },
  render: (args) => ({
    components: { RlText },
    setup: () => ({ args }),
    template:
      '<RlText v-bind="args">Discuss the new onboarding flow before the next release.</RlText>',
  }),
}
export default meta
type Story = StoryObj<typeof RlText>

export const Playground: Story = {}

export const Sizes: Story = {
  render: () => ({
    components: { RlText },
    setup: () => ({ sizes: ['lg', 'md', 'sm', 'xs'] as const }),
    template: `
      <div style="display:flex; flex-direction:column; gap:8px">
        <RlText v-for="s in sizes" :key="s" as="p" :size="s">Text size {{ s }}</RlText>
      </div>
    `,
  }),
}

/** The three tones carry hierarchy; all pass AA on the page background. */
export const Tones: Story = {
  render: () => ({
    components: { RlText },
    setup: () => ({ tones: ['default', 'muted', 'subtle'] as const }),
    template: `
      <div style="display:flex; flex-direction:column; gap:8px">
        <RlText v-for="t in tones" :key="t" as="p" :tone="t">Tone {{ t }}</RlText>
      </div>
    `,
  }),
}

export const Weights: Story = {
  render: () => ({
    components: { RlText },
    setup: () => ({ weights: ['normal', 'medium', 'semibold'] as const }),
    template: `
      <div style="display:flex; flex-direction:column; gap:8px">
        <RlText v-for="w in weights" :key="w" as="p" :weight="w">Weight {{ w }}</RlText>
      </div>
    `,
  }),
}

export const Truncated: Story = {
  render: () => ({
    components: { RlText },
    template: `
      <div style="width:240px; border:1px dashed var(--rl-color-border); padding:8px">
        <RlText truncate>A sentence long enough that it has to be cut short here.</RlText>
      </div>
    `,
  }),
}
