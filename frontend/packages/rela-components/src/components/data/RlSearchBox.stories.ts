import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { computed, ref } from 'vue'
import RlSearchBox from './RlSearchBox.vue'
import RlText from '../../components/common/RlText.vue'

const meta: Meta<typeof RlSearchBox> = {
  title: 'Data/Search box',
  component: RlSearchBox,
  parameters: { layout: 'padded' },
  argTypes: { size: { control: { type: 'inline-radio' }, options: ['sm', 'md'] } },
  args: { placeholder: 'Search tasks', debounce: 250, size: 'md' },
  render: (args) => ({
    components: { RlSearchBox },
    setup: () => ({ args, value: ref('') }),
    template: '<div style="max-width:320px"><RlSearchBox v-bind="args" v-model="value" /></div>',
  }),
}
export default meta
type Story = StoryObj<typeof RlSearchBox>

/**
 * Debounced so every search in the product waits the same amount. Clearing
 * and Enter skip the wait, because both are deliberate acts.
 */
export const Playground: Story = {}

/** Filtering a real list, with the count announced when it changes. */
export const Filtering: Story = {
  render: () => ({
    components: { RlSearchBox, RlText },
    setup() {
      const tasks = [
        'Atlas mobile redesign',
        'Payment provider migration',
        'Onboarding survey',
        'Design token audit',
        'Mobile navigation spike',
      ]
      const query = ref('')
      const matches = computed(() =>
        tasks.filter((task) => task.toLowerCase().includes(query.value.toLowerCase())),
      )
      return { query, matches }
    },
    template: `
      <div style="max-width:360px">
        <RlSearchBox
          v-model="query"
          placeholder="Search tasks"
          :result-text="matches.length + ' results'"
        />
        <ul style="margin:12px 0 0; padding:0; list-style:none; display:flex; flex-direction:column; gap:6px">
          <li v-for="task in matches" :key="task" style="font-size:14px">{{ task }}</li>
        </ul>
        <RlText v-if="!matches.length" size="sm" tone="muted">No tasks match.</RlText>
      </div>
    `,
  }),
}
