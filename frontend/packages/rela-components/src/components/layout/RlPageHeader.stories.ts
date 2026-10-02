import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlPageHeader from './RlPageHeader.vue'
import RlViewTabs from './RlViewTabs.vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlTag from '../common/RlTag.vue'

const meta: Meta<typeof RlPageHeader> = {
  title: 'Layout/Page header',
  component: RlPageHeader,
  parameters: { layout: 'fullscreen' },
  argTypes: {
    statusColor: { control: { type: 'select' }, options: ['green', 'amber', 'red', 'grey', 'blue'] },
    starred: { control: 'boolean' },
    showStar: { control: 'boolean' },
    showMenu: { control: 'boolean' },
    showNavToggle: { control: 'boolean' },
  },
  args: {
    title: 'Atlas for Sourcehaven',
    statusLabel: 'On track',
    statusColor: 'green',
    starred: true,
  },
}
export default meta
type Story = StoryObj<typeof RlPageHeader>

export const Playground: Story = {}

export const TitleOnly: Story = {
  args: { statusLabel: undefined, showStar: false, showMenu: false },
}

/** With the view tabs and tools that sit on the second row. */
export const WithTabs: Story = {
  render: (args) => ({
    components: { RlPageHeader, RlViewTabs, RlIconButton },
    setup: () => ({
      args,
      current: ref('board'),
      tabs: [
        { id: 'board', label: 'Board', icon: 'kanban' },
        { id: 'table', label: 'Table', icon: 'table' },
      ],
    }),
    template: `
      <RlPageHeader v-bind="args">
        <template #tabs><RlViewTabs :tabs="tabs" v-model="current" /></template>
        <template #tools>
          <RlIconButton icon="filter" label="Filter" />
          <RlIconButton icon="arrow-up-down" label="Sort" />
        </template>
      </RlPageHeader>
    `,
  }),
}

/**
 * Tools with no view tabs, which is a list screen that has only one view.
 *
 * The tools stay on the right, where the same controls sit on a header that
 * does have tabs. They are the same controls doing the same job, so moving
 * them to the other end of the bar because the tabs happen to be absent would
 * make two screens in one product disagree about where Search lives.
 */
export const ToolsWithoutTabs: Story = {
  render: (args) => ({
    components: { RlPageHeader, RlIconButton },
    setup: () => ({ args }),
    template: `
      <RlPageHeader v-bind="args">
        <template #tools>
          <RlIconButton icon="search" label="Search" />
          <RlIconButton icon="filter" label="Filter" />
          <RlIconButton icon="arrow-up-down" label="Sort" />
        </template>
      </RlPageHeader>
    `,
  }),
}

/**
 * A host-drawn status marker in the pill's place, such as a property value
 * rendered by the host's own widget. It sits where `statusLabel` would.
 */
export const CustomStatus: Story = {
  render: (args) => ({
    components: { RlPageHeader, RlTag },
    setup: () => ({ args }),
    template: `
      <RlPageHeader v-bind="args">
        <template #status><RlTag label="At risk" /></template>
      </RlPageHeader>
    `,
  }),
  args: { statusLabel: undefined, showStar: false, showMenu: false },
}

/** The hamburger appears only below 1080px. */
export const Compact: Story = { globals: { viewport: { value: 'phone' } } }
