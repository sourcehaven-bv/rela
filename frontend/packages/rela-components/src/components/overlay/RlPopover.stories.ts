import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlPopover from './RlPopover.vue'
import RlButton from '../../components/common/RlButton.vue'
import RlIconButton from '../../components/common/RlIconButton.vue'
import RlText from '../../components/common/RlText.vue'
import RlHeading from '../../components/common/RlHeading.vue'

const meta: Meta<typeof RlPopover> = {
  title: 'Overlay/Popover',
  component: RlPopover,
  parameters: { layout: 'padded' },
  argTypes: {
    align: { control: { type: 'inline-radio' }, options: ['start', 'end'] },
    placement: { control: { type: 'inline-radio' }, options: ['bottom', 'top'] },
  },
  args: { title: 'Next action', align: 'end', placement: 'bottom' },
}
export default meta
type Story = StoryObj<typeof RlPopover>

/**
 * Prose plus a control or two, anchored to what opened it. Escape closes and
 * returns focus to the trigger, a click outside closes, and Tab cycles within
 * the panel rather than walking into the page behind it.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlPopover, RlButton, RlText, RlHeading },
    setup: () => ({ args }),
    template: `
      <div style="padding-bottom:240px">
        <RlPopover v-bind="args">
          <template #trigger="{ toggle, attrs }">
            <RlButton variant="secondary" icon="target" v-bind="attrs" @click="toggle">
              Next action
            </RlButton>
          </template>

          <template #default="{ close }">
            <RlHeading :level="2" size="sm">Review the Meridian lease</RlHeading>
            <RlText as="p" size="sm" tone="muted" style="margin:8px 0 12px">
              Due in three days. It is the only thing blocking the renewal.
            </RlText>
            <div style="display:flex; gap:8px">
              <RlButton variant="primary" size="sm" @click="close">Open it</RlButton>
              <RlButton variant="ghost" size="sm" @click="close">Later</RlButton>
            </div>
          </template>
        </RlPopover>
      </div>
    `,
  }),
}

/**
 * The gap this closes. A menu cannot hold this panel: its body has a link, and
 * a menu closes on any click inside, so the panel would be gone before the
 * link navigated. Here the click follows the link and the panel decides for
 * itself when to close.
 */
export const WithLinkInBody: Story = {
  render: () => ({
    components: { RlPopover, RlButton, RlText },
    setup: () => ({ followed: ref('') }),
    template: `
      <div style="padding-bottom:240px">
        <RlPopover title="Next action">
          <template #trigger="{ toggle, attrs }">
            <RlButton variant="secondary" icon="target" v-bind="attrs" @click="toggle">
              1 action waiting
            </RlButton>
          </template>

          <template #default="{ close }">
            <RlText as="p" size="sm" style="margin:0 0 12px">
              The Meridian lease is waiting on
              <a href="#" @click.prevent="followed = 'Ada'; close()">a countersignature</a>
              before it can go out.
            </RlText>
            <RlButton variant="ghost" size="sm" @click="close">Dismiss</RlButton>
          </template>
        </RlPopover>

        <RlText v-if="followed" as="p" size="sm" tone="muted" style="margin-top:12px">
          Followed the link, then closed.
        </RlText>
      </div>
    `,
  }),
}

/**
 * Anchored to a control in a sidebar footer, which is where the flip matters:
 * there is no room below, so the panel opens upward on its own.
 */
export const InSidebarFooter: Story = {
  render: () => ({
    components: { RlPopover, RlIconButton, RlButton, RlText },
    template: `
      <div
        style="
          width:240px; height:320px; display:flex; flex-direction:column;
          justify-content:flex-end; padding:12px; overflow:hidden;
          border:1px solid var(--rl-color-border); border-radius:8px;
          background:var(--rl-color-bg-sunken)
        "
      >
        <RlPopover title="Next action" align="start" placement="top">
          <template #trigger="{ toggle, attrs }">
            <RlIconButton icon="target" label="Next action" v-bind="attrs" @click="toggle" />
          </template>

          <template #default="{ close }">
            <RlText as="p" size="sm" style="margin:0 0 12px">
              Three relations have no owner. Assigning one unblocks this week's review.
            </RlText>
            <RlButton variant="primary" size="sm" @click="close">Assign owners</RlButton>
          </template>
        </RlPopover>
      </div>
    `,
  }),
}
