import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlDetailPanel from './RlDetailPanel.vue'
import RlTaskDetail from './RlTaskDetail.vue'
import RlMenu from '../overlay/RlMenu.vue'
import RlMenuItem from '../overlay/RlMenuItem.vue'
import RlMenuSeparator from '../overlay/RlMenuSeparator.vue'
import RlIconButton from '../common/RlIconButton.vue'
import { detailDescription, detailFields } from '../../fixtures'

const meta: Meta<typeof RlDetailPanel> = {
  title: 'Task/Detail panel',
  component: RlDetailPanel,
  parameters: { layout: 'fullscreen' },
  argTypes: {
    variant: { control: { type: 'inline-radio' }, options: ['panel', 'page'] },
    showNavigation: { control: 'boolean' },
    showExpand: { control: 'boolean' },
    showLink: { control: 'boolean' },
    showAttach: { control: 'boolean' },
  },
  args: { variant: 'panel', showNavigation: true, showExpand: true, showAttach: true },
  render: (args) => ({
    components: { RlDetailPanel, RlTaskDetail },
    setup: () => ({ args, fields: detailFields, description: detailDescription }),
    template: `
      <div style="height:560px; display:flex">
        <RlDetailPanel v-bind="args">
          <RlTaskDetail
            title="Create awesome UX for Atlas Projects"
            :fields="fields"
            :description="description"
          />
        </RlDetailPanel>
      </div>
    `,
  }),
}
export default meta
type Story = StoryObj<typeof RlDetailPanel>

/** Beside a list, with its own toolbar. */
export const Panel: Story = {}

/** Full-width variant used by the standalone detail page. */
export const Page: Story = { args: { variant: 'page', showNavigation: false } }

/** A host's own menu in the "⋯" button's place, through the `menu` slot. */
export const WithMenu: Story = {
  render: (args) => ({
    components: { RlDetailPanel, RlTaskDetail, RlMenu, RlMenuItem, RlMenuSeparator, RlIconButton },
    setup: () => ({ args, fields: detailFields, description: detailDescription }),
    template: `
      <div style="height:560px; display:flex">
        <RlDetailPanel v-bind="args">
          <template #menu>
            <RlMenu align="end">
              <template #trigger="{ toggle, attrs }">
                <RlIconButton icon="ellipsis" label="More actions" v-bind="attrs" @click="toggle" />
              </template>
              <RlMenuItem icon="edit" shortcut="E">Edit</RlMenuItem>
              <RlMenuItem icon="history">History</RlMenuItem>
              <RlMenuSeparator />
              <RlMenuItem icon="delete" tone="danger">Delete</RlMenuItem>
            </RlMenu>
          </template>
          <RlTaskDetail
            title="Create awesome UX for Atlas Projects"
            :fields="fields"
            :description="description"
          />
        </RlDetailPanel>
      </div>
    `,
  }),
}
