import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlActivityBar from './RlActivityBar.vue'
import RlButton from '../../components/common/RlButton.vue'
import RlText from '../../components/common/RlText.vue'

const meta: Meta<typeof RlActivityBar> = {
  title: 'Feedback/Activity bar',
  component: RlActivityBar,
  parameters: { layout: 'padded' },
  args: { active: true, delay: 300 },
}
export default meta
type Story = StoryObj<typeof RlActivityBar>

/**
 * The first of the three pending indicators: this one is for navigation
 * between screens, RlButton's loading state is for an explicit action, and
 * RlAutoSaveIndicator is for ambient background saving. Exactly one of them
 * should respond to any single user act.
 *
 * It is indeterminate on purpose. A fake percentage that stalls at 90 tells
 * the user less than an honest "something is happening".
 */
export const Playground: Story = {}

/**
 * Held back by 300ms, so a fast response never flashes a bar. Press the
 * button to simulate a navigation that takes a second.
 */
export const DelayedReveal: Story = {
  render: () => ({
    components: { RlActivityBar, RlButton, RlText },
    setup() {
      const active = ref(false)
      function navigate(duration: number) {
        active.value = true
        setTimeout(() => (active.value = false), duration)
      }
      return { active, navigate }
    },
    template: `
      <div style="display:flex; flex-direction:column; gap:12px; align-items:flex-start">
        <RlActivityBar :active="active" />
        <RlButton variant="secondary" @click="navigate(1200)">Slow navigation (shows a bar)</RlButton>
        <RlButton variant="secondary" @click="navigate(150)">Fast navigation (stays quiet)</RlButton>
        <RlText size="sm" tone="muted">The bar is pinned to the top of the viewport.</RlText>
      </div>
    `,
  }),
}
