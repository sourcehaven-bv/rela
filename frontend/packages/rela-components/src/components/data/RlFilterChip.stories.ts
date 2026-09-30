import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlFilterChip from './RlFilterChip.vue'
import RlButton from '../../components/common/RlButton.vue'

const meta: Meta<typeof RlFilterChip> = {
  title: 'Data/Filter chip',
  component: RlFilterChip,
  parameters: { layout: 'padded' },
  args: { label: 'Status', value: 'In progress', removable: true },
}
export default meta
type Story = StoryObj<typeof RlFilterChip>

/**
 * The remove button names the filter it removes, since "Remove" on its own
 * means nothing in a row of six chips.
 */
export const Playground: Story = {}

/** A row of active filters with a way to clear them all. */
export const FilterBar: Story = {
  render: () => ({
    components: { RlFilterChip, RlButton },
    setup() {
      const filters = ref([
        { id: 'status', label: 'Status', value: 'In progress', icon: 'done' as const },
        { id: 'assignee', label: 'Assignee', value: 'Hanry Fonda', icon: 'user' as const },
        { id: 'label', label: 'Label', value: 'Design', icon: 'filter' as const },
      ])
      return { filters }
    },
    template: `
      <div style="display:flex; flex-wrap:wrap; align-items:center; gap:8px">
        <RlFilterChip
          v-for="filter in filters"
          :key="filter.id"
          :label="filter.label"
          :value="filter.value"
          :icon="filter.icon"
          @remove="filters = filters.filter((entry) => entry.id !== filter.id)"
        />
        <RlButton v-if="filters.length" size="sm" variant="ghost" @click="filters = []">
          Clear all
        </RlButton>
      </div>
    `,
  }),
}

/** Without a value, for a filter that is simply on or off. */
export const LabelOnly: Story = { args: { value: undefined, label: 'Only my tasks' } }
