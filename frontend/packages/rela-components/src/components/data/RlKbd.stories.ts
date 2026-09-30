import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlKbd from './RlKbd.vue'

const meta: Meta<typeof RlKbd> = {
  title: 'Data/Keyboard key',
  component: RlKbd,
  parameters: { layout: 'padded' },
  argTypes: {
    separator: { control: { type: 'inline-radio' }, options: ['+', 'then', 'or'] },
    size: { control: { type: 'inline-radio' }, options: ['sm', 'md'] },
  },
  args: { keys: 'Ctrl+K', separator: '+', size: 'md' },
}
export default meta
type Story = StoryObj<typeof RlKbd>

/**
 * The separator carries meaning: `+` is held together, `then` is pressed in
 * sequence. It is spoken as well as drawn, so "G then D" is announced as
 * three tokens rather than the unpronounceable "GthenD".
 */
export const Playground: Story = {}

/** A shortcut reference, the form this is built for. */
export const ShortcutList: Story = {
  render: () => ({
    components: { RlKbd },
    setup: () => ({
      shortcuts: [
        { action: 'Open the command palette', keys: 'Ctrl+K', separator: '+' as const },
        { action: 'Focus search', keys: '/', separator: '+' as const },
        { action: 'Go to dashboard', keys: 'G then D', separator: 'then' as const },
        { action: 'Next task', keys: 'J or Down', separator: 'or' as const },
        { action: 'Save', keys: 'Cmd+Enter', separator: '+' as const },
      ],
    }),
    template: `
      <dl style="display:grid; grid-template-columns:1fr auto; gap:10px 24px; margin:0; max-width:420px">
        <template v-for="shortcut in shortcuts" :key="shortcut.action">
          <dt style="font-size:14px">{{ shortcut.action }}</dt>
          <dd style="margin:0">
            <RlKbd :keys="shortcut.keys" :separator="shortcut.separator" />
          </dd>
        </template>
      </dl>
    `,
  }),
}
