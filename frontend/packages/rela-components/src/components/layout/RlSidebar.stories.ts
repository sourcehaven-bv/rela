import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSidebar from './RlSidebar.vue'
import { navGroups, meetingNavGroups } from '../../fixtures'
import RlSidebarNavItem from './RlSidebarNavItem.vue'
import RlWorkspaceSwitcher from './RlWorkspaceSwitcher.vue'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlText from '../common/RlText.vue'
import RlIconButton from '../common/RlIconButton.vue'
import type { NavItem } from '../../types'

const meta: Meta<typeof RlSidebar> = {
  title: 'Sidebar/Sidebar',
  component: RlSidebar,
  render: (args) => ({
    components: { RlSidebar },
    setup: () => ({ args }),
    template: `<div style="height:100vh; display:flex"><RlSidebar v-bind="args" /></div>`,
  }),
}
export default meta

type Story = StoryObj<typeof RlSidebar>

export const Default: Story = {
  args: { workspaceName: 'Atlas Projects', groups: navGroups, activeId: 'atlas' },
}

/**
 * The rail. The sidebar has no collapsed state of its own, so the app
 * collapses it by rebinding `--rl-sidebar-width`, and every part that
 * carries text notices the width through a container query.
 *
 * Labels are hidden visually rather than removed, so each row keeps its
 * accessible name, and the group headings become rules.
 */
export const Rail: Story = {
  args: { workspaceName: 'Atlas Projects', groups: navGroups, activeId: 'atlas' },
  render: (args) => ({
    components: { RlSidebar },
    setup: () => ({ args }),
    template: `
      <div style="height:100vh; display:flex; --rl-sidebar-width: 60px">
        <RlSidebar v-bind="args" />
      </div>
    `,
  }),
}

export const WithMeetings: Story = {
  args: { workspaceName: 'Atlas Projects', groups: meetingNavGroups, activeId: 'mt-2' },
}

/**
 * The three open seams, together: a replaced switcher, pinned entries above
 * the navigation, and a footer carrying more than one link.
 *
 * The pinned entries sit outside the filter, so typing in it never hides
 * them, and they carry no group heading, because they are not a category.
 */
export const OpenSeams: Story = {
  render: () => ({
    components: {
      RlSidebar, RlSidebarNavItem, RlWorkspaceSwitcher, RlStatusDot, RlText, RlIconButton,
    },
    setup: () => ({
      groups: navGroups,
      pinned: [
        { id: 'search', label: 'Search', icon: 'search' },
        { id: 'analysis', label: 'Analysis', icon: 'chart-no-axes-gantt' },
      ] as NavItem[],
    }),
    template: `
      <div style="height:100vh; display:flex">
        <!-- Filter forced on, to show pinned entries surviving it. -->
        <RlSidebar
          :groups="groups"
          workspace-name="Atlas Projects"
          active-id="atlas"
          :filter-threshold="0"
        >
          <template #switcher>
            <RlWorkspaceSwitcher name="Sourcehaven / Atlas" />
          </template>

          <template #pinned>
            <RlSidebarNavItem v-for="item in pinned" :key="item.id" :item="item" />
          </template>

          <template #footer>
            <div style="display:flex; align-items:center; gap:8px; padding:4px 8px">
              <RlStatusDot color="amber" />
              <RlText size="sm" tone="muted">main · 3 uncommitted</RlText>
            </div>
            <div style="display:flex; gap:4px; padding:0 4px">
              <RlIconButton icon="settings" label="Settings" />
              <RlIconButton icon="moon" label="Toggle dark mode" />
            </div>
          </template>
        </RlSidebar>
      </div>
    `,
  }),
}
