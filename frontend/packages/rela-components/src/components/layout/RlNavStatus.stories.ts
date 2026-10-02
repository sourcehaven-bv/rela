import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import RlSidebar from './RlSidebar.vue'
import type { NavGroup } from '../../types'

const meta: Meta<typeof RlSidebar> = {
  title: 'Layout/Nav status',
  component: RlSidebar,
  parameters: { layout: 'fullscreen' },
}
export default meta
type Story = StoryObj<typeof RlSidebar>

/* One row per tone, so the five read against each other down one column. */
const groups: NavGroup[] = [
  {
    id: 'main',
    items: [
      { id: 'search', label: 'Search', icon: 'search' },
      {
        id: 'inbox',
        label: 'Inbox',
        icon: 'inbox',
        status: { tone: 'new', label: '12 unread', count: 12 },
      },
      {
        id: 'notes',
        label: 'Release notes',
        icon: 'news',
        status: { tone: 'info', label: 'Updated today' },
      },
      {
        id: 'billing',
        label: 'Billing',
        icon: 'card',
        status: { tone: 'warning', label: 'Card expires this month' },
      },
      {
        id: 'deploys',
        label: 'Deployments',
        icon: 'rocket',
        status: { tone: 'error', label: 'Last deploy failed' },
      },
      {
        id: 'checks',
        label: 'Compliance',
        icon: 'shield-check',
        status: { tone: 'success', label: 'All checks passing' },
      },
    ],
  },
  {
    id: 'initiatives',
    label: 'Initiatives',
    items: [
      { id: 'atlas', label: 'Atlas for Sourcehaven', initial: 'R' },
      {
        id: 'tender',
        label: 'Company tender ready',
        initial: 'R',
        status: { tone: 'warning', label: 'Due in 2 days' },
      },
      {
        id: 'crowded',
        label: 'A label long enough to be truncated by the rail',
        initial: 'J',
        status: { tone: 'new', label: '3 new', count: 3 },
      },
    ],
  },
]

/**
 * The five tones. Each is a glyph and a colour the sidebar picks, so a row
 * flagged `error` looks the same wherever it is: the caller says what is
 * true, not what to draw.
 *
 * `new` is a plain dot, because it means "something is here" and nothing
 * more precise. A glyph would imply a kind of thing it does not know.
 */
export const Tones: Story = {
  args: { workspaceName: 'Atlas Projects', groups, activeId: 'atlas' },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    /*
     * Asserted through the accessible name rather than by counting glyphs: an
     * icon and a colour carry no text, so the label is the only part of the
     * indicator a screen reader can reach, and it is the part worth keeping.
     */
    await expect(canvas.getByRole('button', { name: /Deployments.*Last deploy failed/ })).toBeInTheDocument()
    await expect(canvas.getByRole('button', { name: /Inbox.*12 unread/ })).toBeInTheDocument()

    /* A row without a status says nothing extra. */
    const search = canvas.getByRole('button', { name: /Search/ })
    await expect(search.textContent).not.toMatch(/unread|failed/)
  },
}

/**
 * Collapsed to a rail, every tone becomes one dot on the icon's corner.
 *
 * A count has nowhere to render at 60px, and a glyph beside a clipped label
 * is unreadable, so the indicator keeps only the part that survives losing
 * the words: something here, look. The label is still in the accessible
 * name, so a screen reader loses nothing at all.
 */
export const Rail: Story = {
  args: { workspaceName: 'Atlas Projects', groups, activeId: 'atlas' },
  decorators: [
    () => ({ template: '<div style="--rl-sidebar-width:60px; height:100vh"><story /></div>' }),
  ],
}

/**
 * A collapsed parent stands in for the statuses it is hiding, so a branch
 * with a failure inside does not look calm while it is shut.
 *
 * The most severe tone wins and the counts add up, because the row is
 * answering "is there anything in here" rather than reporting each child.
 * Expanding it hands the job back to the children, which can say precisely
 * which one is wrong.
 */
export const RollsUpWhenCollapsed: Story = {
  args: {
    workspaceName: 'Atlas Projects',
    groups: [
      {
        id: 'main',
        items: [
          {
            id: 'integrations',
            label: 'Integrations',
            icon: 'plug',
            children: [
              { id: 'slack', label: 'Slack', status: { tone: 'success', label: 'Connected' } },
              { id: 'jira', label: 'Jira', status: { tone: 'error', label: 'Token expired' } },
              { id: 'gh', label: 'GitHub', status: { tone: 'new', label: '4 new events', count: 4 } },
            ],
          },
        ],
      },
    ],
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    /*
     * Collapsed, the parent carries the worst of what it hides. Asserted on
     * the accessible name, since that is the part a reader gets: the colour
     * and glyph carry nothing on their own.
     */
    const parent = canvas.getByRole('button', { name: /Integrations/ })
    await expect(parent).toHaveAccessibleName(/3 items need attention/)

    await userEvent.click(parent)

    /* Open, the children speak for themselves and the parent falls silent. */
    await expect(await canvas.findByRole('button', { name: /Jira.*Token expired/ })).toBeInTheDocument()
    await expect(canvas.getByRole('button', { name: /^Integrations$/ })).toBeInTheDocument()
  },
}

/**
 * A summary counts within its own tone, never across tones.
 *
 * Adding an unread count to a failed sync gives a number that means nothing:
 * "7" across "4 new" and "3 errors" is not 7 of anything. Here the winning
 * tone is error and both errors carry counts, so the badge says 3 and the
 * four new events are left out of it rather than folded in.
 */
export const RollUpCountsWithinOneTone: Story = {
  args: {
    workspaceName: 'Atlas Projects',
    groups: [
      {
        id: 'main',
        items: [
          {
            id: 'sync',
            label: 'Sync',
            icon: 'refresh',
            children: [
              { id: 'a', label: 'Mailbox', status: { tone: 'new', label: '4 new', count: 4 } },
              { id: 'b', label: 'Calendar', status: { tone: 'error', label: '2 failed', count: 2 } },
              { id: 'c', label: 'Contacts', status: { tone: 'error', label: '1 failed', count: 1 } },
            ],
          },
        ],
      },
    ],
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const parent = canvas.getByRole('button', { name: /Sync/ })

    await expect(parent).toHaveAccessibleName(/3 items need attention/)

    /*
     * 3, the two error counts, not 7. Asserted on the drawn number because
     * this is the one part of the indicator the label does not already say.
     */
    const badge = parent.querySelector('.rl-nav-status__count')
    await expect(badge).toHaveTextContent('3')
  },
}

/**
 * A mixed tone with no counts shows none rather than a partial sum, which
 * would under-report while reading as exact.
 */
export const RollUpDropsPartialCounts: Story = {
  args: {
    workspaceName: 'Atlas Projects',
    groups: [
      {
        id: 'main',
        items: [
          {
            id: 'checks',
            label: 'Checks',
            icon: 'shield-check',
            children: [
              { id: 'a', label: 'Retention', status: { tone: 'warning', label: 'Review due', count: 2 } },
              { id: 'b', label: 'Access', status: { tone: 'warning', label: 'Needs an owner' } },
            ],
          },
        ],
      },
    ],
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const parent = canvas.getByRole('button', { name: /Checks/ })

    await expect(parent).toHaveAccessibleName(/2 items need attention/)
    await expect(parent.querySelector('.rl-nav-status__count')).toBeNull()
  },
}
