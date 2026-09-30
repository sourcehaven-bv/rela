import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlTooltip from './RlTooltip.vue'
import RlIconButton from '../../components/common/RlIconButton.vue'

const meta: Meta<typeof RlTooltip> = {
  title: 'Overlay/Tooltip',
  component: RlTooltip,
  parameters: { layout: 'centered' },
  argTypes: {
    placement: { control: { type: 'inline-radio' }, options: ['top', 'bottom', 'left', 'right'] },
  },
  args: { text: 'Copy link to this task', placement: 'top' },
}
export default meta
type Story = StoryObj<typeof RlTooltip>

/**
 * Appears on focus as well as hover, so a keyboard user gets it too
 * (WCAG 1.4.13). It supplements a control that already has its own name; it
 * is never the only place a meaning lives.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlTooltip, RlIconButton },
    setup: () => ({ args }),
    template: `
      <RlTooltip v-bind="args">
        <template #default="{ describedBy }">
          <RlIconButton icon="link" label="Copy link" :aria-describedby="describedBy" />
        </template>
      </RlTooltip>
    `,
  }),
}

/** A toolbar where every icon explains itself on hover or focus. */
export const Toolbar: Story = {
  render: () => ({
    components: { RlTooltip, RlIconButton },
    template: `
      <div style="display:flex; gap:4px; padding:40px 0">
        <RlTooltip text="Star this task">
          <template #default="{ describedBy }">
            <RlIconButton icon="star" label="Star" :aria-describedby="describedBy" />
          </template>
        </RlTooltip>
        <RlTooltip text="Copy link to this task">
          <template #default="{ describedBy }">
            <RlIconButton icon="link" label="Copy link" :aria-describedby="describedBy" />
          </template>
        </RlTooltip>
        <RlTooltip text="Attach a file" placement="bottom">
          <template #default="{ describedBy }">
            <RlIconButton icon="paperclip" label="Attach" :aria-describedby="describedBy" />
          </template>
        </RlTooltip>
      </div>
    `,
  }),
}
