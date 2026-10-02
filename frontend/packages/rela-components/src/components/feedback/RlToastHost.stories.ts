import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, waitFor, within } from 'storybook/test'
import { ref } from 'vue'
import RlToastHost from './RlToastHost.vue'
import RlButton from '../common/RlButton.vue'
import RlText from '../common/RlText.vue'
import { useToasts } from './useToasts'
import type { ToastMessage } from './types'

const meta: Meta<typeof RlToastHost> = {
  title: 'Feedback/Toast host',
  component: RlToastHost,
  parameters: { layout: 'padded' },
  argTypes: {
    placement: {
      control: { type: 'inline-radio' },
      options: ['bottom-right', 'top-right', 'top-center'],
    },
  },
  args: { placement: 'bottom-right' },
}
export default meta
type Story = StoryObj<typeof RlToastHost>

/**
 * The corner where toasts stack. It teleports to `<body>`, so it sits above
 * the app regardless of where it is mounted; pick the placement once at the
 * app root rather than per message.
 *
 * Narrow the viewport and the stack goes full width at the bottom on every
 * placement, clear of the thumb reach at the very edge.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlToastHost, RlButton },
    setup() {
      const { toasts, dismiss, success, info, warning, danger, clear } = useToasts()
      return { args, toasts, dismiss, success, info, warning, danger, clear }
    },
    template: `
      <div style="display:flex; flex-wrap:wrap; gap:8px; align-items:center">
        <RlButton variant="secondary" @click="success('Task created', 'Atlas mobile redesign is on the board.')">Success</RlButton>
        <RlButton variant="secondary" @click="info('Link copied')">Info</RlButton>
        <RlButton variant="secondary" @click="warning('Sync is behind', 'Last updated 2 hours ago.')">Warning</RlButton>
        <RlButton variant="secondary" tone="danger" @click="danger('Could not save', 'Your connection dropped.')">Error</RlButton>
        <RlButton variant="ghost" @click="clear()">Clear</RlButton>

        <RlToastHost v-bind="args" :toasts="toasts" @dismiss="dismiss" />
      </div>
    `,
  }),
}

/**
 * One live region wraps the whole list rather than one per toast, so adding a
 * third message announces only that message instead of re-reading the two
 * already on screen. It is `polite`, because a toast reports something that
 * has already happened and should not cut across what the user is doing.
 */
export const Stacked: Story = {
  render: (args) => ({
    components: { RlToastHost, RlButton, RlText },
    setup() {
      const toasts = ref<ToastMessage[]>([
        { id: '1', title: 'Task created', tone: 'success', duration: 0 },
        { id: '2', title: 'Link copied', tone: 'info', duration: 0 },
        {
          id: '3',
          title: 'Could not save',
          description: 'Your connection dropped.',
          tone: 'danger',
          duration: 0,
        },
      ])
      const dismiss = (id: string) => {
        toasts.value = toasts.value.filter((toast) => toast.id !== id)
      }
      return { args, toasts, dismiss }
    },
    template: `
      <div>
        <RlText size="sm" tone="muted">
          These are pinned with duration 0, so they stay until dismissed.
        </RlText>
        <RlToastHost v-bind="args" :toasts="toasts" @dismiss="dismiss" />
      </div>
    `,
  }),
}

/** An empty host is still in the DOM, so the live region exists before the
 * first message arrives. It must not swallow clicks while it is empty, which
 * is what the pointer-events rule is for: the button underneath still works. */
export const EmptyDoesNotBlockClicks: Story = {
  render: (args) => ({
    components: { RlToastHost, RlButton },
    setup: () => ({ args, clicks: ref(0) }),
    template: `
      <div style="display:flex; flex-direction:column; align-items:flex-end; gap:8px">
        <RlButton variant="secondary" @click="clicks++">Clicked {{ clicks }} times</RlButton>
        <RlToastHost v-bind="args" :toasts="[]" />
      </div>
    `,
  }),
}

/**
 * A toast can carry one action, for the act the message invites: undoing a
 * delete, retrying a failed save.
 *
 * One rather than several, because a toast leaves on a timer and a choice
 * the user has five seconds to weigh is not a choice. Undo is usually the
 * better pattern than a confirmation dialog: one click instead of two, and
 * honest about the fact that people confirm dialogs reflexively.
 */
export const WithUndo: StoryObj = {
  play: async () => {
    /* The host teleports, so the toast is on the body rather than the canvas. */
    const canvas = within(document.body)

    const undo = await canvas.findByRole('button', { name: 'Undo' })

    /*
     * Ordered before the dismiss, so a reader tabbing in reaches the act the
     * message invites before the control that throws the message away.
     */
    const buttons = canvas.getAllByRole('button')
    await expect(buttons.indexOf(undo)).toBeLessThan(
      buttons.indexOf(canvas.getByRole('button', { name: 'Dismiss notification' })),
    )

    await userEvent.click(undo)
    await expect(await canvas.findByTestId('undone')).toBeInTheDocument()

    /* The toast goes: it described a state the action has just changed. */
    await waitFor(() => expect(canvas.queryByRole('button', { name: 'Undo' })).toBeNull())
  },
  render: () => ({
    components: { RlToastHost },
    setup() {
      const undone = ref(false)
      const toasts = ref<ToastMessage[]>([
        {
          id: 'deleted',
          title: 'Task deleted',
          description: 'Pricing page: new iteration',
          tone: 'info',
          duration: 0,
          action: { label: 'Undo', onAction: () => (undone.value = true) },
        },
      ])
      return {
        undone,
        toasts,
        dismiss: (id: string) => (toasts.value = toasts.value.filter((t) => t.id !== id)),
      }
    },
    template: `
      <div style="height:320px">
        <p v-if="undone" data-testid="undone">Restored</p>
        <RlToastHost :toasts="toasts" @dismiss="dismiss" />
      </div>
    `,
  }),
}

/**
 * A `danger` toast is announced assertively, the rest politely.
 *
 * `aria-live` is read from the element whose content changed and cannot be
 * varied per message, so the host keeps two regions and routes by tone. They
 * are `display: contents`, so the split reaches a screen reader and leaves
 * the visible stack as one column.
 */
export const DangerIsAssertive: StoryObj = {
  play: async () => {
    const body = within(document.body)

    const failed = await body.findByText('Save failed')
    const saved = await body.findByText('Task saved')

    const regionOf = (el: HTMLElement) => el.closest('[aria-live]')?.getAttribute('aria-live')

    await expect(regionOf(failed)).toBe('assertive')
    await expect(regionOf(saved)).toBe('polite')

    /*
     * Both live regions exist before any message arrives: a region inserted
     * together with its content is not announced.
     */
    await expect(document.querySelectorAll('.rl-toast-host [aria-live]')).toHaveLength(2)
  },
  render: () => ({
    components: { RlToastHost },
    setup() {
      const toasts = ref<ToastMessage[]>([
        { id: 'ok', title: 'Task saved', tone: 'success', duration: 0 },
        { id: 'bad', title: 'Save failed', tone: 'danger', duration: 0 },
      ])
      return {
        toasts,
        dismiss: (id: string) => (toasts.value = toasts.value.filter((t) => t.id !== id)),
      }
    },
    template: `
      <div style="height:320px">
        <RlToastHost :toasts="toasts" @dismiss="dismiss" />
      </div>
    `,
  }),
}

/** The three shapes an action takes, side by side. */
export const Actions: StoryObj = {
  render: () => ({
    components: { RlToastHost },
    setup() {
      const toasts = ref<ToastMessage[]>([
        {
          id: 'undo',
          title: 'Task deleted',
          description: 'Pricing page: new iteration',
          duration: 0,
          action: { label: 'Undo', onAction: () => {} },
        },
        {
          id: 'retry',
          title: 'Could not save',
          description: 'The connection dropped.',
          tone: 'danger',
          duration: 0,
          action: { label: 'Retry', onAction: () => {} },
        },
        {
          id: 'view',
          title: 'Report ready',
          tone: 'success',
          duration: 0,
          action: { label: 'View', onAction: () => {} },
        },
      ])
      return { toasts, dismiss: () => {} }
    },
    template: `<div style="height:320px"><RlToastHost :toasts="toasts" @dismiss="dismiss" /></div>`,
  }),
}
