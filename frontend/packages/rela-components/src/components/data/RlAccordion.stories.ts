import type { Meta, StoryObj } from '@storybook/vue3-vite'
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
