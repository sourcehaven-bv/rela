import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlBusyIndicator from './RlBusyIndicator.vue'

/** An ISO time `ms` milliseconds ago. */
const ago = (ms: number) => new Date(Date.now() - ms).toISOString()
const HOUR = 3_600_000

const meta: Meta<typeof RlBusyIndicator> = {
  title: 'Feedback/Busy indicator',
  component: RlBusyIndicator,
  parameters: { layout: 'padded' },
  args: { entries: [{ typeLabel: 'Agent run', label: 'Planning', since: ago(3 * HOUR + 60_000) }] },
}
export default meta
type Story = StoryObj<typeof RlBusyIndicator>

/** The visible text a sighted reader sees. */
function visible(canvasElement: HTMLElement): string {
  return canvasElement.querySelector('.rl-busy__text')!.textContent!.trim()
}

/** The text a screen reader hears: everything not hidden from it. */
function spoken(canvasElement: HTMLElement): string {
  return canvasElement.querySelector('.rl-visually-hidden')!.textContent!.trim()
}

/**
 * One busy entry: the state and the time in it. The screen reader hears
 * the type, the state and the time, once.
 */
export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(visible(canvasElement)).toBe('Planning · 3h')
    await expect(spoken(canvasElement)).toBe('Agent run: Planning, for 3 hours')
    const root = canvasElement.querySelector('.rl-busy')!
    await expect(root).not.toHaveAttribute('role')
    await expect(root).not.toHaveAttribute('aria-live')
    for (const part of root.querySelectorAll('.rl-busy__dots, .rl-busy__icon, .rl-busy__text')) {
      await expect(part).toHaveAttribute('aria-hidden', 'true')
    }
  },
}

/**
 * A long state label is cut off with an ellipsis in a narrow card. The
 * time in the state and the "+N" stay whole, because the time is the sign
 * that a run has stalled.
 */
export const LongLabel: Story = {
  args: {
    entries: [
      {
        typeLabel: 'Agent run',
        label: 'Implementing the plan and running every test suite',
        since: ago(20 * 60_000),
      },
      { typeLabel: 'Agent run', label: 'Planning', since: ago(60_000) },
    ],
  },
  render: (args) => ({
    components: { RlBusyIndicator },
    setup: () => ({ args }),
    template: '<div style="width:160px"><RlBusyIndicator v-bind="args" /></div>',
  }),
  play: async ({ canvasElement }) => {
    const label = canvasElement.querySelector<HTMLElement>('.rl-busy__label')!
    await expect(label.scrollWidth).toBeGreaterThan(label.clientWidth)
    const tail = canvasElement.querySelector<HTMLElement>('.rl-busy__tail')!
    await expect(tail.textContent).toBe(' · 20m +1')
    await expect(tail.scrollWidth).toBeLessThanOrEqual(tail.clientWidth)
    const box = canvasElement.querySelector('.rl-busy')!.getBoundingClientRect()
    await expect(tail.getBoundingClientRect().right).toBeLessThanOrEqual(box.right + 0.5)
    await expect(spoken(canvasElement)).toBe(
      'Agent run: Implementing the plan and running every test suite, for 20 minutes; ' +
        'Agent run: Planning, for 1 minute',
    )
  },
}

/** Several busy entries: the first shows with "+N", and the name lists all. */
export const SeveralEntries: Story = {
  args: {
    entries: [
      { typeLabel: 'Agent run', label: 'Planning', since: ago(30_000) },
      { typeLabel: 'Agent run', label: 'Implementing', since: ago(2 * 24 * HOUR) },
      { typeLabel: 'Review', label: 'Reviewing' },
    ],
  },
  play: async ({ canvasElement }) => {
    await expect(visible(canvasElement)).toBe('Planning · <1m +2')
    await expect(spoken(canvasElement)).toBe(
      'Agent run: Planning, for less than a minute; Agent run: Implementing, for 2 days; Review: Reviewing',
    )
  },
}

/**
 * With a reduced-motion preference the dots are hidden and a static icon
 * takes their place. Turn the preference on in the browser's rendering
 * tools to see it; the check below follows whichever preference is set.
 */
export const ReducedMotion: Story = {
  play: async ({ canvasElement }) => {
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const dots = canvasElement.querySelector('.rl-busy__dots')!
    const icon = canvasElement.querySelector('.rl-busy__icon')!
    await expect(dots).toHaveAttribute('aria-hidden', 'true')
    await expect(icon).toHaveAttribute('aria-hidden', 'true')
    if (reduce) {
      await expect(dots).not.toBeVisible()
      await expect(icon).toBeVisible()
    } else {
      await expect(dots).toBeVisible()
      await expect(icon).not.toBeVisible()
    }
    await expect(within(canvasElement).queryByRole('status')).toBeNull()
  },
}

/** No entries render nothing. */
export const Empty: Story = {
  args: { entries: [] },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector('.rl-busy')).toBeNull()
  },
}
