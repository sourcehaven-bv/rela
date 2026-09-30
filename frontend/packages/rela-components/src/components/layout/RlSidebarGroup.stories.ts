import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlSidebarGroup from './RlSidebarGroup.vue'
import { crowdedNavGroups } from '../../fixtures/bulk'
import { navGroups } from '../../fixtures'

const long = crowdedNavGroups[1]
const short = navGroups[1]

const meta: Meta<typeof RlSidebarGroup> = {
  title: 'Sidebar/Group',
  component: RlSidebarGroup,
  parameters: { layout: 'padded' },
  decorators: [
    () => ({
      template:
        '<div style="width:260px; background:var(--rl-color-bg-sunken); padding:12px"><story /></div>',
    }),
  ],
}
export default meta
type Story = StoryObj<typeof RlSidebarGroup>

/**
 * Five items: a list you read top to bottom. Subheadings here would spend
 * vertical space organising something nobody was struggling with, so the
 * group stays flat even though it could split.
 */
export const Short: Story = {
  args: { group: short, activeId: 'atlas' },
}

/**
 * Thirty items: the same component, the same props, past the threshold. The
 * subheadings come from the group's own `subgroups`, and "Other" collects
 * what carries no `groupId` rather than dropping it.
 */
export const Long: Story = {
  args: { group: long, activeId: 'seat-count' },
}

/**
 * A long group with nothing to split by stays flat. Grouping needs both a
 * reason and a way: past the threshold is the reason, `subgroups` is the way.
 */
export const LongWithoutSubgroups: Story = {
  args: { group: { ...long, subgroups: undefined }, activeId: 'seat-count' },
}

/** The threshold is a prop, so a caller can keep a long group flat. */
export const ThresholdRaised: Story = {
  args: { group: long, activeId: 'seat-count', groupThreshold: Infinity },
}
