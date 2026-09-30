import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlHeading from './RlHeading.vue'

const meta: Meta<typeof RlHeading> = {
  title: 'Typography/Heading',
  component: RlHeading,
  parameters: { layout: 'padded' },
  argTypes: {
    level: { control: { type: 'select' }, options: [1, 2, 3, 4, 5, 6] },
    size: { control: { type: 'select' }, options: [undefined, 'sm', 'md', 'lg', 'xl', '2xl'] },
    weight: { control: { type: 'select' }, options: ['normal', 'medium', 'semibold'] },
    tone: { control: { type: 'select' }, options: ['default', 'muted'] },
    truncate: { control: 'boolean' },
  },
  args: { level: 2, weight: 'semibold', tone: 'default', truncate: false },
  render: (args) => ({
    components: { RlHeading },
    setup: () => ({ args }),
    template: '<RlHeading v-bind="args">Create awesome UX for Atlas Projects</RlHeading>',
  }),
}
export default meta
type Story = StoryObj<typeof RlHeading>

export const Playground: Story = {}

/** Every visual size. The level is fixed so only the size changes. */
export const Sizes: Story = {
  render: () => ({
    components: { RlHeading },
    setup: () => ({ sizes: ['2xl', 'xl', 'lg', 'md', 'sm'] as const }),
    template: `
      <div style="display:flex; flex-direction:column; gap:12px">
        <RlHeading v-for="s in sizes" :key="s" :level="2" :size="s">
          Heading {{ s }}
        </RlHeading>
      </div>
    `,
  }),
}

/**
 * The size prop is independent of the level, so a visually small heading can
 * still be an h2 where the document outline needs one.
 */
export const LevelIndependentOfSize: Story = {
  render: () => ({
    components: { RlHeading },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px">
        <RlHeading :level="1" size="md">h1 rendered at md</RlHeading>
        <RlHeading :level="4" size="xl">h4 rendered at xl</RlHeading>
      </div>
    `,
  }),
}

export const Tones: Story = {
  render: () => ({
    components: { RlHeading },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px">
        <RlHeading :level="2" size="md" weight="normal">Default tone</RlHeading>
        <RlHeading :level="2" size="md" weight="normal" tone="muted">Muted tone</RlHeading>
      </div>
    `,
  }),
}

export const Truncated: Story = {
  render: () => ({
    components: { RlHeading },
    template: `
      <div style="width:240px; border:1px dashed var(--rl-color-border); padding:8px">
        <RlHeading :level="2" size="lg" truncate>
          A title far too long to fit inside this narrow container
        </RlHeading>
      </div>
    `,
  }),
}
