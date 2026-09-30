import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlAutoSaveIndicator from './RlAutoSaveIndicator.vue'
import RlTextarea from '../../components/form/RlTextarea.vue'

const meta: Meta<typeof RlAutoSaveIndicator> = {
  title: 'Feedback/Auto-save indicator',
  component: RlAutoSaveIndicator,
  parameters: { layout: 'padded' },
  argTypes: {
    state: { control: { type: 'inline-radio' }, options: ['idle', 'saving', 'saved', 'error'] },
  },
  args: { state: 'saving' },
}
export default meta
type Story = StoryObj<typeof RlAutoSaveIndicator>

/**
 * The third pending indicator, for a form that saves as the user types.
 * Nothing is shown while idle: a permanent "Saved" is reassurance the user
 * stopped reading long ago.
 */
export const Playground: Story = {}

export const States: Story = {
  render: () => ({
    components: { RlAutoSaveIndicator },
    template: `
      <div style="display:flex; flex-direction:column; gap:8px">
        <RlAutoSaveIndicator state="saving" />
        <RlAutoSaveIndicator state="saved" />
        <RlAutoSaveIndicator state="error" error-text="Offline. Retrying." />
      </div>
    `,
  }),
}

/** In place: type in the field and the state moves from saving to saved. */
export const InAForm: Story = {
  render: () => ({
    components: { RlAutoSaveIndicator, RlTextarea },
    setup() {
      const notes = ref('')
      const state = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
      let timer: ReturnType<typeof setTimeout> | undefined

      function onType() {
        state.value = 'saving'
        clearTimeout(timer)
        timer = setTimeout(() => (state.value = 'saved'), 900)
      }
      return { notes, state, onType }
    },
    template: `
      <div style="max-width:440px">
        <div style="display:flex; justify-content:flex-end; min-height:20px">
          <RlAutoSaveIndicator :state="state" />
        </div>
        <RlTextarea v-model="notes" label="Notes" :rows="4" @update:model-value="onType" />
      </div>
    `,
  }),
}
