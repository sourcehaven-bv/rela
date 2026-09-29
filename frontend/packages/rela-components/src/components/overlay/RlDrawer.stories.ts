import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlDrawer from './RlDrawer.vue'
import RlButton from '../../components/common/RlButton.vue'
import RlButtonGroup from '../../components/common/RlButtonGroup.vue'
import RlTextField from '../../components/form/RlTextField.vue'
import RlTextarea from '../../components/form/RlTextarea.vue'
import RlSelect from '../../components/form/RlSelect.vue'

const meta: Meta<typeof RlDrawer> = {
  title: 'Overlay/Drawer',
  component: RlDrawer,
  parameters: { layout: 'padded' },
  argTypes: {
    side: { control: { type: 'inline-radio' }, options: ['left', 'right'] },
    size: { control: { type: 'inline-radio' }, options: ['sm', 'md', 'lg'] },
  },
  args: { title: 'New task', side: 'right', size: 'md', modal: true },
}
export default meta
type Story = StoryObj<typeof RlDrawer>

/**
 * For a form that should not take the user away from the list behind it.
 * Becomes full-screen below 768px, where there is no room for a side panel.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlDrawer, RlButton, RlButtonGroup, RlTextField, RlTextarea, RlSelect },
    setup: () => ({
      args,
      open: ref(false),
      name: ref(''),
      notes: ref(''),
      status: ref('todo'),
      statuses: [
        { value: 'todo', label: 'To do' },
        { value: 'doing', label: 'In progress' },
        { value: 'done', label: 'Done' },
      ],
    }),
    template: `
      <div>
        <RlButton variant="primary" icon="plus" @click="open = true">New task</RlButton>

        <RlDrawer v-bind="args" :open="open" @close="open = false">
          <div style="display:flex; flex-direction:column; gap:16px">
            <RlTextField v-model="name" label="Task name" placeholder="Name this task" />
            <RlSelect v-model="status" label="Status" :options="statuses" />
            <RlTextarea v-model="notes" label="Notes" :rows="4" auto-grow />
          </div>

          <template #actions>
            <RlButtonGroup>
              <RlButton variant="secondary" @click="open = false">Cancel</RlButton>
              <RlButton variant="primary" @click="open = false">Create task</RlButton>
            </RlButtonGroup>
          </template>
        </RlDrawer>
      </div>
    `,
  }),
}

/**
 * Non-modal: no scrim and no focus trap, so the page behind stays usable.
 * For an inspector the user reads while still working, never for a form.
 */
export const NonModal: Story = {
  ...Playground,
  args: { title: 'Details', modal: false, size: 'sm' },
}

export const FromLeft: Story = { ...Playground, args: { title: 'Filters', side: 'left', size: 'sm' } }
