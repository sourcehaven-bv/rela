import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import { ref } from 'vue'
import RlAccordion from './RlAccordion.vue'
import RlProgressBar from '../feedback/RlProgressBar.vue'
import RlText from '../../components/common/RlText.vue'

const meta: Meta<typeof RlAccordion> = {
  title: 'Data/Accordion',
  component: RlAccordion,
  parameters: { layout: 'padded' },
  argTypes: { level: { control: { type: 'inline-radio' }, options: [2, 3, 4] } },
  args: { title: 'Atlas mobile redesign', count: 8, open: true, level: 3 },
  render: (args) => ({
    components: { RlAccordion, RlText },
    setup: () => ({ args }),
    template: `
      <RlAccordion v-bind="args">
        <RlText as="p" size="sm" tone="muted">The tasks in this epic go here.</RlText>
      </RlAccordion>
    `,
  }),
}
export default meta
type Story = StoryObj<typeof RlAccordion>

/**
 * Built on `details`/`summary`, so it opens without JavaScript, is keyboard
 * operable for free, and stays findable by the browser's in-page search even
 * while collapsed.
 */
export const Playground: Story = {}

/**
 * Several sections, each with a rollup bar in the summary row. The heading
 * level is a prop because the right one depends on where it is used.
 */
export const EpicList: Story = {
  render: () => ({
    components: { RlAccordion, RlProgressBar, RlText },
    setup: () => ({
      epics: [
        {
          title: 'Atlas mobile redesign',
          count: 8,
          meta: '5 of 8 done',
          segments: [
            { label: 'done', value: 5, tone: 'success' as const },
            { label: 'in progress', value: 2, tone: 'accent' as const },
            { label: 'to do', value: 1, tone: 'neutral' as const },
          ],
        },
        {
          title: 'Payment provider migration',
          count: 12,
          meta: '3 of 12 done',
          segments: [
            { label: 'done', value: 3, tone: 'success' as const },
            { label: 'blocked', value: 2, tone: 'danger' as const },
            { label: 'to do', value: 7, tone: 'neutral' as const },
          ],
        },
      ],
    }),
    template: `
      <div style="max-width:560px">
        <RlAccordion
          v-for="(epic, index) in epics"
          :key="epic.title"
          :title="epic.title"
          :count="epic.count"
          :meta="epic.meta"
          :open="index === 0"
        >
          <template #summaryExtra>
            <RlProgressBar
              :segments="epic.segments"
              :aria-label="epic.title + ' progress'"
              size="sm"
              style="width:100px"
            />
          </template>
          <RlText as="p" size="sm" tone="muted">The tasks in this epic go here.</RlText>
        </RlAccordion>
      </div>
    `,
  }),
}

export const Collapsed: Story = { args: { open: false } }

/**
 * One item of a list, with a menu for the item. The menu sits beside the
 * summary, so opening it does not fold the section. `divider` is off because
 * the list around it separates its items.
 */
export const WithActions: Story = {
  render: () => ({
    components: { RlAccordion, RlText },
    setup: () => ({ pressed: ref(0), open: ref(false) }),
    template: `
      <div style="max-width:560px">
        <RlAccordion title="Hero" meta="Model your domain" :divider="false" :open="open" @toggle="open = $event">
          <RlText as="p" size="sm" tone="muted">The block's fields go here.</RlText>
          <template #actions>
            <button type="button" class="rl-control rl-control--sm" @click="pressed++">Options</button>
          </template>
        </RlAccordion>
        <RlText as="p" size="sm" tone="muted">Pressed {{ pressed }} times; {{ open ? 'open' : 'closed' }}.</RlText>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: 'Options' }))
    await expect(canvas.getByText('Pressed 1 times; closed.')).toBeInTheDocument()
  },
}

/**
 * A summary longer than the row truncates and gives way to the title, which
 * keeps its full text.
 */
export const LongMeta: Story = {
  args: {
    title: 'Text',
    count: undefined,
    open: false,
    meta: 'rela turns a YAML schema into typed entities, relations and a web app, and keeps every change in the audit log so nothing is lost.',
  },
  render: (args) => ({
    components: { RlAccordion },
    setup: () => ({ args }),
    template: `<div style="width:420px"><RlAccordion v-bind="args">Body</RlAccordion></div>`,
  }),
  play: async ({ canvasElement }) => {
    const title = canvasElement.querySelector<HTMLElement>('.rl-accordion__title')!
    const meta = canvasElement.querySelector<HTMLElement>('.rl-accordion__meta')!
    await expect(title.scrollWidth).toBeLessThanOrEqual(title.clientWidth)
    await expect(meta.getBoundingClientRect().left).toBeGreaterThanOrEqual(title.getBoundingClientRect().right)
  },
}
