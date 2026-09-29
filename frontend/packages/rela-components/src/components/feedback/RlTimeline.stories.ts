import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlTimeline from './RlTimeline.vue'
import RlTag from '../common/RlTag.vue'
import type { TimelineEntry, TimelineGroup } from './timeline'

const entries: TimelineEntry[] = [
  {
    id: 'e1',
    actor: 'Hanna',
    summary: 'moved this to In review',
    timestamp: '2 hours ago',
    datetime: '2026-09-27T08:12:00Z',
    icon: 'arrow-right',
    tone: 'blue',
    current: true,
  },
  {
    id: 'e2',
    actor: 'Rowdy',
    summary: 'linked the control ISO-27001.A.9',
    timestamp: '4 hours ago',
    datetime: '2026-09-27T06:40:00Z',
    icon: 'link',
  },
  {
    id: 'e3',
    actor: 'Hanna',
    summary: 'changed the owner from Rowdy to Ada',
    timestamp: 'Yesterday 16:20',
    datetime: '2026-09-26T14:20:00Z',
    icon: 'user-add',
  },
  {
    id: 'e4',
    summary: 'The nightly validation passed',
    timestamp: 'Yesterday 03:00',
    datetime: '2026-09-26T01:00:00Z',
    icon: 'check',
    tone: 'green',
  },
]

const meta: Meta<typeof RlTimeline> = {
  title: 'Feedback/Timeline',
  component: RlTimeline,
  parameters: { layout: 'padded' },
  args: { entries },
  decorators: [() => ({ template: '<div style="max-width:560px"><story /></div>' })],
}
export default meta
type Story = StoryObj<typeof RlTimeline>

export const Playground: Story = {}

/**
 * The markup, which is the part worth asserting.
 *
 * An ordered list, because the sequence IS the content: a screen reader should
 * say "2 of 4" as it moves through the events. A stack of divs would read as
 * four unrelated paragraphs.
 */
export const Semantics: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    const list = canvas.getByRole('list')
    await expect(list.tagName).toBe('OL')
    await expect(canvas.getAllByRole('listitem')).toHaveLength(4)

    /* The newest entry is marked as the current one. */
    const items = canvas.getAllByRole('listitem')
    await expect(items[0]).toHaveAttribute('aria-current', 'true')
    await expect(items[1]).not.toHaveAttribute('aria-current')

    /*
     * A real `<time datetime>` where the caller passed the instant, so the date
     * behind "2 hours ago" is recoverable rather than lost in a formatted
     * string.
     */
    const when = canvasElement.querySelector('time')
    await expect(when).toHaveAttribute('datetime', '2026-09-27T08:12:00Z')
  },
}

/** Avatars, for a feed where several people act. Decorative: the name is beside them. */
export const WithAvatars: Story = {
  args: { avatars: true },
}

/**
 * Grouped under day headings, which the app buckets. The timeline does not do it
 * itself: deciding which day an instant falls in needs a display timezone, which
 * is the same boundary `calendarGrid.ts` draws.
 */
export const Grouped: Story = {
  args: {
    groups: [
      { id: 'today', label: 'Today', entries: entries.slice(0, 2) },
      { id: 'yesterday', label: '26 September', entries: entries.slice(2) },
    ] as TimelineGroup[],
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    /* One list per group, each with its own heading. */
    await expect(canvas.getAllByRole('list')).toHaveLength(2)
    await expect(canvas.getByRole('heading', { name: 'Today' })).toBeInTheDocument()
    await expect(canvas.getByRole('heading', { name: '26 September' })).toBeInTheDocument()
  },
}

/** A longer body under the summary: a comment quoted into the trail, or a reason. */
export const WithDetail: Story = {
  args: {
    entries: [
      {
        id: 'd1',
        actor: 'Ada',
        summary: 'rejected the change',
        timestamp: '10 minutes ago',
        icon: 'x',
        tone: 'red',
        detail:
          'The control still references the 2022 policy. Please repoint it before this goes back for review.',
      },
      ...entries.slice(1, 3),
    ] as TimelineEntry[],
  },
}

/**
 * The `entry` slot, for content that needs markup rather than a sentence: a link
 * to the thing that changed, or the new value as a tag.
 *
 * This is why `summary` is a plain string and not a set of parts. The word order
 * of "who did what to which thing" differs by language, so the library either
 * takes a finished sentence or hands the whole line over.
 */
export const RichEntries: Story = {
  render: () => ({
    components: { RlTimeline, RlTag },
    setup: () => ({ entries: entries.slice(0, 3) }),
    template: `
      <RlTimeline :entries="entries">
        <template #entry="{ entry }">
          <span>{{ entry.summary.split(' ')[0] }}</span>
          <RlTag v-if="entry.id === 'e1'" color="blue" label="In review" />
          <a v-else-if="entry.id === 'e2'" href="#iso-a9">ISO-27001.A.9</a>
          <span v-else>{{ entry.summary.split(' ').slice(1).join(' ') }}</span>
        </template>
      </RlTimeline>
    `,
  }),
}

/** Nothing has happened yet. Left unset the timeline renders nothing at all. */
export const Empty: Story = {
  args: { entries: [], emptyLabel: 'No activity yet.' },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('No activity yet.')).toBeInTheDocument()
    await expect(canvas.queryByRole('list')).toBeNull()
  },
}
