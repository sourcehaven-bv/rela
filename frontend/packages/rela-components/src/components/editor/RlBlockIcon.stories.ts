import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlBlockIcon from './RlBlockIcon.vue'
import { ICON_PARTS } from './editorIcons'

const meta: Meta<typeof RlBlockIcon> = {
  title: 'Editor/Block icon',
  component: RlBlockIcon,
  parameters: { layout: 'padded' },
  args: { name: 'strong' },
  argTypes: { name: { control: 'select', options: Object.keys(ICON_PARTS) } },
}
export default meta
type Story = StoryObj<typeof RlBlockIcon>

/**
 * The editor toolbar's own glyph set, separate from `RlIcon`'s app-chrome
 * set. A command's `id` is its icon name, so the toolbar needs no mapping.
 */
export const Playground: Story = {}

/**
 * Every glyph, read straight from `ICON_PARTS`, so a new one shows up here
 * without an edit. An unknown name renders an empty `<svg>` rather than
 * throwing, which is why a toolbar button with no glyph means a missing
 * entry rather than a broken command.
 */
export const Gallery: Story = {
  render: () => ({
    components: { RlBlockIcon },
    setup: () => ({ names: Object.keys(ICON_PARTS) }),
    template: `
      <div style="display:grid; grid-template-columns:repeat(auto-fill,minmax(120px,1fr)); gap:16px">
        <div
          v-for="name in names"
          :key="name"
          style="display:flex; flex-direction:column; align-items:center; gap:6px; color:var(--rl-color-text-muted)"
        >
          <RlBlockIcon :name="name" />
          <code style="font-size:var(--rl-font-size-xs); color:var(--rl-color-text-subtle)">{{ name }}</code>
        </div>
      </div>
    `,
  }),
}

/** The glyphs take their colour from the surrounding text. */
export const InheritsColour: Story = {
  render: () => ({
    components: { RlBlockIcon },
    setup: () => ({
      tones: ['var(--rl-color-text)', 'var(--rl-color-text-muted)', 'var(--rl-color-accent)'],
    }),
    template: `
      <div style="display:flex; gap:20px">
        <span v-for="tone in tones" :key="tone" :style="{ color: tone }">
          <RlBlockIcon name="blockquote" />
        </span>
      </div>
    `,
  }),
}
