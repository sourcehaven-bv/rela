import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlNavIcon from './RlNavIcon.vue'

const meta: Meta<typeof RlNavIcon> = {
  title: 'Layout/NavIcon',
  component: RlNavIcon,
  parameters: { layout: 'padded' },
  args: { name: 'kanban', reserve: true, collapsed: false, size: 16 },
}
export default meta
type Story = StoryObj<typeof RlNavIcon>

export const Playground: Story = {}

const row = `display:flex; align-items:center; gap:12px; font-size:14px`

/**
 * Why the reserved box exists. Without it the unlabelled row's text slides
 * left and the list reads as ragged; with it every label starts at the same x.
 */
export const Alignment: Story = {
  render: () => ({
    components: { RlNavIcon },
    template: `
      <div style="display:flex; gap:48px">
        <div>
          <p style="font-size:12px; color:var(--rl-color-text-muted)">reserve</p>
          <div style="${row}"><RlNavIcon name="kanban" reserve /><span>Board</span></div>
          <div style="${row}"><RlNavIcon reserve /><span>No icon</span></div>
          <div style="${row}"><RlNavIcon name="table" reserve /><span>Table</span></div>
        </div>
        <div>
          <p style="font-size:12px; color:var(--rl-color-text-muted)">without</p>
          <div style="${row}"><RlNavIcon name="kanban" /><span>Board</span></div>
          <div style="${row}"><RlNavIcon /><span>No icon</span></div>
          <div style="${row}"><RlNavIcon name="table" /><span>Table</span></div>
        </div>
      </div>
    `,
  }),
}

/**
 * Collapsed, a reserved slot draws its `fallback` instead of empty space. With
 * the labels hidden, an empty row would be invisible but still clickable.
 */
export const Collapsed: Story = {
  render: () => ({
    components: { RlNavIcon },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px">
        <RlNavIcon name="kanban" reserve collapsed />
        <RlNavIcon reserve collapsed fallback="file" />
      </div>
    `,
  }),
}
