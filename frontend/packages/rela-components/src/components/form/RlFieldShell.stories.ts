import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlFieldShell from './RlFieldShell.vue'

const meta: Meta<typeof RlFieldShell> = {
  title: 'Form/Field shell',
  component: RlFieldShell,
  parameters: { layout: 'padded' },
  args: { label: 'Custom field', hint: 'The shell owns the label, hint and error.' },
}
export default meta
type Story = StoryObj<typeof RlFieldShell>

/**
 * The wrapper behind every input. Use it directly to give a control the
 * library does not cover the same labelling and error handling as one it does.
 *
 * The scoped slot hands out the id and the ARIA wiring, so a custom control
 * cannot accidentally be left unlabelled. Those slot props are the supported
 * contract: `id`, `describedBy` (error id before hint id, `undefined` when
 * there is neither), `invalid` and `required`.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlFieldShell },
    setup: () => ({ args }),
    template: `
      <RlFieldShell v-bind="args" style="max-width:360px">
        <template #default="control">
          <input
            :id="control.id"
            class="rl-control"
            :aria-describedby="control.describedBy"
            :aria-invalid="control.invalid || undefined"
            placeholder="Any control can go here"
          />
        </template>
      </RlFieldShell>
    `,
  }),
}

/** With an error present, the control is marked invalid and the ring recolours. */
export const WithError: Story = {
  ...Playground,
  args: { error: 'This value is not valid.' },
}
