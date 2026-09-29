import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlMenu from './RlMenu.vue'
import RlMenuItem from './RlMenuItem.vue'
import RlMenuSeparator from './RlMenuSeparator.vue'
import RlIconButton from '../../components/common/RlIconButton.vue'
import RlButton from '../../components/common/RlButton.vue'
import RlConfirmDialog from '../../components/common/RlConfirmDialog.vue'
import RlText from '../../components/common/RlText.vue'

const meta: Meta<typeof RlMenu> = {
  title: 'Overlay/Menu',
  component: RlMenu,
  parameters: { layout: 'padded' },
  argTypes: {
    align: { control: { type: 'inline-radio' }, options: ['start', 'end'] },
    placement: { control: { type: 'inline-radio' }, options: ['bottom', 'top'] },
  },
  args: { align: 'end', placement: 'bottom' },
}
export default meta
type Story = StoryObj<typeof RlMenu>

/**
 * The panel behind a "..." button. Arrow keys move between items, Home and End
 * jump to the ends, Escape closes and returns focus to the trigger, and Tab
 * leaves the menu rather than cycling inside it.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlMenu, RlMenuItem, RlMenuSeparator, RlIconButton },
    setup: () => ({ args, last: ref('') }),
    template: `
      <div style="padding-bottom:200px">
        <RlMenu v-bind="args">
          <template #trigger="{ toggle, attrs }">
            <RlIconButton icon="ellipsis" label="More options" v-bind="attrs" @click="toggle" />
          </template>

          <RlMenuItem icon="link" shortcut="C" @click="last = 'Copy link'">Copy link</RlMenuItem>
          <RlMenuItem icon="file" @click="last = 'Duplicate'">Duplicate</RlMenuItem>
          <RlMenuItem icon="star" @click="last = 'Add to starred'">Add to starred</RlMenuItem>
          <RlMenuSeparator />
          <RlMenuItem icon="trash-2" tone="danger" @click="last = 'Delete'">Delete task</RlMenuItem>
        </RlMenu>

        <p v-if="last" style="margin-top:12px; font-size:13px; color:var(--rl-color-text-muted)">Chose: {{ last }}</p>
      </div>
    `,
  }),
}

/** Any control can be the trigger, as long as it spreads the bound attributes. */
export const ButtonTrigger: Story = {
  render: () => ({
    components: { RlMenu, RlMenuItem, RlButton },
    template: `
      <div style="padding-bottom:200px">
        <RlMenu align="start">
          <template #trigger="{ toggle, attrs, open }">
            <RlButton variant="secondary" icon="arrow-up-down" v-bind="attrs" @click="toggle">
              Sort by{{ open ? '' : '' }}
            </RlButton>
          </template>
          <RlMenuItem>Newest first</RlMenuItem>
          <RlMenuItem>Oldest first</RlMenuItem>
          <RlMenuItem>Priority</RlMenuItem>
        </RlMenu>
      </div>
    `,
  }),
}

/**
 * The gap this closes: a Delete in an overflow menu that opens a confirmation
 * rather than acting on one click.
 */
export const WithConfirmation: Story = {
  render: () => ({
    components: { RlMenu, RlMenuItem, RlMenuSeparator, RlIconButton, RlConfirmDialog, RlText },
    setup: () => ({ confirming: ref(false), deleted: ref(false) }),
    template: `
      <div style="padding-bottom:200px">
        <RlMenu>
          <template #trigger="{ toggle, attrs }">
            <RlIconButton icon="ellipsis" label="More options" v-bind="attrs" @click="toggle" />
          </template>
          <RlMenuItem icon="link">Copy link</RlMenuItem>
          <RlMenuSeparator />
          <RlMenuItem icon="trash-2" tone="danger" @click="confirming = true">Delete task</RlMenuItem>
        </RlMenu>

        <RlConfirmDialog
          :open="confirming"
          title="Delete this task?"
          description="Atlas mobile redesign and its 3 subtasks will be removed. This cannot be undone."
          @cancel="confirming = false"
          @confirm="confirming = false; deleted = true"
        />

        <RlText v-if="deleted" as="p" size="sm" tone="muted">Deleted.</RlText>
      </div>
    `,
  }),
}
