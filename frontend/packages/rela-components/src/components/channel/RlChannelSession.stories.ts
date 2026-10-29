import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import RlChannelSession from './RlChannelSession.vue'
import RlChannelDecisions from './RlChannelDecisions.vue'
import { provideMessages } from '../../composables/useMessages'
import {
  breachDecisions,
  breachSessionAssessed,
  breachSessionCancelled,
  breachSessionDetected,
  breachSessionFailed,
  breachSessionFinal,
  breachSessionInputRequired,
  breachSessionLoading,
  breachSessionStranded,
  breachSessionWaitingOnMe,
} from '../../fixtures/channels'

const meta: Meta<typeof RlChannelSession> = {
  title: 'Channel/Channel session',
  component: RlChannelSession,
  parameters: { layout: 'padded' },
  args: { session: breachSessionAssessed },
}
export default meta
type Story = StoryObj<typeof RlChannelSession>

/** The items of the list, with the text a screen reader hears. */
function steps(canvasElement: HTMLElement): HTMLElement[] {
  return [...canvasElement.querySelectorAll<HTMLElement>('ol > li')]
}

function current(canvasElement: HTMLElement): HTMLElement[] {
  return steps(canvasElement).filter((li) => li.getAttribute('aria-current') === 'step')
}

/** The items marked next, by their visible label. */
function nextLabels(canvasElement: HTMLElement): string[] {
  return steps(canvasElement)
    .filter((li) => li.textContent?.includes(', next'))
    .map((li) => li.querySelector('span:nth-child(2)')!.textContent!.trim())
}

const narrow: Story['render'] = (args) => ({
  components: { RlChannelSession },
  setup: () => ({ args }),
  template: '<div style="width:320px"><RlChannelSession v-bind="args" /></div>',
})

/** A linear run: the current state is marked, the next one has hidden text. */
export const Linear: Story = {
  play: async ({ canvasElement }) => {
    await expect(steps(canvasElement).map((li) => li.querySelector('span:nth-child(2)')!.textContent)).toEqual([
      'Detected',
      'Contained',
      'Risk assessed',
      'AP notified',
      'Dismissed',
    ])
    const cur = current(canvasElement)
    await expect(cur).toHaveLength(1)
    await expect(cur[0]).toHaveTextContent('Risk assessed')
    await expect(nextLabels(canvasElement)).toEqual(['AP notified'])
    await expect(within(canvasElement).queryAllByRole('button')).toEqual([])
    await expect(canvasElement.querySelector('a')).toBeNull()
  },
}

/** A branch: both targets of Detected are next, each marked once. */
export const Branching: Story = {
  args: { session: { ...breachSessionDetected, next: ['contained', 'dismissed', 'contained'] } },
  play: async ({ canvasElement }) => {
    await expect(nextLabels(canvasElement)).toEqual(['Contained', 'Dismissed'])
    const contained = steps(canvasElement)[1]!
    await expect(contained.textContent!.match(/, next/g)).toHaveLength(1)
  },
}

/** A session that never moved reads as its initial state, with nothing extra. */
export const Initial: Story = {
  args: { session: breachSessionDetected },
  play: async ({ canvasElement }) => {
    const cur = current(canvasElement)
    await expect(cur).toHaveLength(1)
    await expect(cur[0]!.textContent!.trim()).toBe('Detected')
  },
}

/** An ended session: a check, the hidden end-state text, "Ended", no next marks. */
export const Final: Story = {
  args: { session: breachSessionFinal },
  play: async ({ canvasElement }) => {
    const cur = current(canvasElement)[0]!
    await expect(cur).toHaveTextContent('AP notified')
    await expect(cur.querySelector('svg')).not.toBeNull()
    await expect(cur).toHaveTextContent(', end state')
    await expect(cur).toHaveTextContent('Ended')
    await expect(nextLabels(canvasElement)).toEqual([])
  },
}

/** A final state whose status is completed reads "Completed" with the check. */
export const Completed: Story = {
  args: { session: { ...breachSessionFinal, status: 'completed' } },
  play: async ({ canvasElement }) => {
    const cur = current(canvasElement)[0]!
    await expect(cur.querySelector('svg')).not.toBeNull()
    await expect(cur).toHaveTextContent('Completed')
    await expect(cur).not.toHaveTextContent('Ended')
  },
}

/** A cancelled session reads "Cancelled", with no check. */
export const Cancelled: Story = {
  args: { session: breachSessionCancelled },
  play: async ({ canvasElement }) => {
    const cur = current(canvasElement)[0]!
    await expect(cur).toHaveTextContent('Dismissed')
    await expect(cur).toHaveTextContent('Cancelled')
    await expect(cur.querySelector('svg')).toBeNull()
    await expect(within(canvasElement).queryByText('Ended')).toBeNull()
  },
}

/** A failed session shows a red "Failed" tag, with no check. */
export const Failed: Story = {
  args: { session: breachSessionFailed },
  play: async ({ canvasElement }) => {
    const cur = current(canvasElement)[0]!
    const tag = cur.querySelector('.rl-tag')
    await expect(tag).toHaveTextContent('Failed')
    await expect(tag).toHaveClass('rl-tag--red')
    await expect(cur.querySelector('svg')).toBeNull()
  },
}

/** An input_required state: an amber tag on the current item, and whom it waits for. */
export const InputRequired: Story = {
  args: { session: breachSessionInputRequired },
  play: async ({ canvasElement }) => {
    const cur = current(canvasElement)[0]!
    const tag = cur.querySelector('.rl-tag')
    await expect(tag).toHaveTextContent('Input required')
    await expect(tag).toHaveClass('rl-tag--amber')
    await expect(within(canvasElement).getByText('Waiting for the incident lead')).toBeVisible()
    await expect(within(canvasElement).getByRole('status')).toHaveTextContent('Waiting for the incident lead')
    await expect(canvasElement).not.toHaveTextContent('Waiting for you')
  },
}

/** A session that waits for the reader says so with a tag. */
export const WaitingOnMe: Story = {
  args: { session: breachSessionWaitingOnMe },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('Waiting for you')).toBeVisible()
    await expect(canvas.getByRole('status')).toHaveTextContent('Waiting for you')
    await expect(canvas.queryByText('Waiting for the incident lead')).toBeNull()
    await expect(current(canvasElement)[0]).toHaveTextContent('Input required')
  },
}

/** A paused session does not say it waits: nothing can answer on a closed channel. */
export const InputRequiredClosed: Story = {
  args: { session: { ...breachSessionWaitingOnMe, paused: 'closed' } },
  play: async ({ canvasElement }) => {
    await expect(canvasElement).not.toHaveTextContent('Input required')
    await expect(canvasElement).not.toHaveTextContent('Waiting for')
    await expect(within(canvasElement).getByRole('status')).toBeEmptyDOMElement()
  },
}

/** A stored state the configuration dropped: no current item, a warning. */
export const Stranded: Story = {
  args: { session: breachSessionStranded },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(current(canvasElement)).toEqual([])
    await expect(
      canvas.getByText('triage is no longer a state of this session. Messages do not move it.'),
    ).toBeVisible()
    await expect(canvasElement.querySelector('.rl-channel-session__stranded')).toHaveTextContent('Stranded')
    await expect(nextLabels(canvasElement)).toEqual([])
  },
}

/** A closed channel pauses the session; the current state stays marked. */
export const Closed: Story = {
  args: { session: { ...breachSessionDetected, paused: 'closed' } },
  play: async ({ canvasElement }) => {
    await expect(current(canvasElement)[0]).toHaveTextContent('Detected')
    await expect(within(canvasElement).getByText('Paused while the channel is closed')).toBeVisible()
    await expect(nextLabels(canvasElement)).toEqual([])
  },
}

/** A session that ended is not paused, even on a closed channel. */
export const FinalClosed: Story = {
  args: { session: { ...breachSessionFinal, paused: 'closed' } },
  play: async ({ canvasElement }) => {
    await expect(canvasElement).toHaveTextContent('Ended')
    await expect(canvasElement).not.toHaveTextContent('Paused')
  },
}

/** Stranded wins over paused. */
export const StrandedClosed: Story = {
  args: { session: { ...breachSessionStranded, paused: 'archived' } },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('.rl-channel-session__stranded')).toHaveTextContent('Stranded')
    await expect(canvasElement).not.toHaveTextContent('Paused')
  },
}

/** In a narrow container the list collapses behind a toggle. */
export const Narrow: Story = {
  render: narrow,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const list = canvasElement.querySelector('ol')!
    await expect(list).not.toBeVisible()
    await expect(canvas.getByText('Session · Risk assessed')).toBeVisible()
    const toggle = canvas.getByRole('button', { name: 'Show all states' })
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await expect(toggle).toHaveAttribute('aria-controls', list.id)
    await userEvent.click(toggle)
    await expect(toggle).toHaveAttribute('aria-expanded', 'true')
    await expect(list).toBeVisible()
    await expect(canvas.getByRole('button', { name: 'Hide the states' })).toBe(toggle)
  },
}

/** The collapsed summary of a stranded session names the stored state. */
export const NarrowStranded: Story = {
  args: { session: breachSessionStranded },
  render: narrow,
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('Stranded · triage')).toBeVisible()
  },
}

/**
 * The app is still loading the states that include the current one: no
 * current item, no next marks, no warning, and a note that it is loading.
 */
export const Loading: Story = {
  args: { session: breachSessionLoading },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(current(canvasElement)).toEqual([])
    await expect(nextLabels(canvasElement)).toEqual([])
    await expect(canvasElement).not.toHaveTextContent('Stranded')
    await expect(canvas.getByText('Loading the states of this session…')).toBeVisible()
  },
}

/** The collapsed summary of a loading session names the stored state. */
export const NarrowLoading: Story = {
  args: { session: breachSessionLoading },
  render: narrow,
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('Session · triaged · Loading')).toBeVisible()
  },
}

/** A locale can reorder the summary and the hidden step text as a whole. */
export const LocalizedTexts: Story = {
  args: { session: { ...breachSessionDetected, next: ['contained'] } },
  render: (args) => ({
    components: { RlChannelSession },
    setup() {
      provideMessages({
        channelSessionSummary: ({ status, label }) => `[${status}] ${label}`,
        channelSessionStep: ({ label, next, final }) => `${label}: ${next ? 'N' : ''}${final ? 'F' : ''}`,
      })
      return { args }
    },
    template: '<div style="width:320px"><RlChannelSession v-bind="args" /></div>',
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getByText('[normal] Detected')).toBeVisible()
    await expect(canvasElement).toHaveTextContent('Contained: N')
    await expect(canvasElement).toHaveTextContent('Dismissed: F')
  },
}

const withDecisions =
  (width?: string): Story['render'] =>
  (args) => ({
    components: { RlChannelSession, RlChannelDecisions },
    setup: () => ({ args, decisions: breachDecisions }),
    template: `
    <div :style="${width ? `'width:${width}'` : "''"}">
      <RlChannelSession v-bind="args">
        <template #decisions><RlChannelDecisions :decisions="decisions" /></template>
      </RlChannelSession>
    </div>`,
  })

/** The decisions sit on their own row under the states. */
export const WithDecisions: Story = {
  args: { session: breachSessionWaitingOnMe },
  render: withDecisions(),
  play: async ({ canvasElement }) => {
    const group = within(canvasElement).getByRole('group', { name: 'Decisions' })
    await expect(within(group).getAllByRole('button')).toHaveLength(3)
    await expect(steps(canvasElement)[1]).toHaveAttribute('aria-current', 'step')
  },
}

/** At 300px the states fold behind the toggle; the decisions stay visible and wrap. */
export const NarrowWithDecisions: Story = {
  args: { session: breachSessionWaitingOnMe },
  render: withDecisions('300px'),
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('ol')).not.toBeVisible()
    const group = within(canvasElement).getByRole('group', { name: 'Decisions' })
    for (const b of within(group).getAllByRole('button')) await expect(b).toBeVisible()
    await expect(group.scrollWidth).toBeLessThanOrEqual(group.clientWidth + 1)
  },
}
