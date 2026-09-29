import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlModal from './RlModal.vue'
import RlButton from '../../components/common/RlButton.vue'
import RlButtonGroup from '../../components/common/RlButtonGroup.vue'
import RlTextField from '../../components/form/RlTextField.vue'
import RlText from '../../components/common/RlText.vue'

const meta: Meta<typeof RlModal> = {
  title: 'Overlay/Modal',
  component: RlModal,
  parameters: { layout: 'padded' },
  argTypes: {
    size: { control: { type: 'inline-radio' }, options: ['sm', 'md', 'lg'] },
    role: { control: { type: 'inline-radio' }, options: ['dialog', 'alertdialog'] },
    align: { control: { type: 'inline-radio' }, options: ['center', 'top'] },
  },
  args: { title: 'Create a task', size: 'md' },
}
export default meta
type Story = StoryObj<typeof RlModal>

/**
 * Focus moves to the first control on open, Tab is trapped, Escape and the
 * scrim close it, and focus returns to the button that opened it.
 */
export const Playground: Story = {
  render: (args) => ({
    components: { RlModal, RlButton, RlButtonGroup, RlTextField },
    setup: () => ({ args, open: ref(false), name: ref('') }),
    template: `
      <div>
        <RlButton variant="primary" icon="plus" @click="open = true">New task</RlButton>
        <RlModal v-bind="args" :open="open" @close="open = false">
          <RlTextField v-model="name" label="Task name" placeholder="Name this task" />
          <template #actions>
            <RlButtonGroup>
              <RlButton variant="secondary" @click="open = false">Cancel</RlButton>
              <RlButton variant="primary" @click="open = false">Create</RlButton>
            </RlButtonGroup>
          </template>
        </RlModal>
      </div>
    `,
  }),
}

/**
 * A persistent dialog blocks every exit that is not a decision, for a form
 * that is mid-submit and would otherwise be left half done.
 */
export const Persistent: Story = {
  ...Playground,
  args: { title: 'Publishing', persistent: true },
}

/**
 * Two dialogs open at once. The overlay stack means Escape closes only the
 * top one, and the scroll lock is released only when the last one goes.
 */
export const Stacked: Story = {
  render: () => ({
    components: { RlModal, RlButton, RlButtonGroup, RlText },
    setup: () => ({ first: ref(false), second: ref(false) }),
    template: `
      <div>
        <RlButton variant="primary" @click="first = true">Open first</RlButton>

        <RlModal title="First dialog" :open="first" @close="first = false">
          <RlText as="p" size="md">Press Escape and only this one's child closes.</RlText>
          <template #actions>
            <RlButtonGroup>
              <RlButton variant="secondary" @click="first = false">Close</RlButton>
              <RlButton variant="primary" @click="second = true">Open second</RlButton>
            </RlButtonGroup>
          </template>
        </RlModal>

        <RlModal title="Second dialog" size="sm" :open="second" @close="second = false">
          <RlText as="p" size="md">Escape closes this one and leaves the first open.</RlText>
        </RlModal>
      </div>
    `,
  }),
}

/**
 * A dialog the user types into sits on the reading line rather than in the
 * middle, so the list of results grows downwards into empty space.
 */
export const TopAligned: Story = {
  ...Playground,
  args: { title: 'Run a command', align: 'top', size: 'lg' },
}

/**
 * The root is a Teleport, so Vue cannot fall a class or a style through on
 * its own. They are taken by hand and put on the panel, which is what a
 * caller means by them. `panelClass` does the same for a named class.
 */
export const CustomPanel: Story = {
  render: (args) => ({
    components: { RlModal, RlText },
    setup: () => ({ args }),
    template: `
      <RlModal v-bind="args" :open="true" :style="{ maxWidth: '860px' }">
        <RlText as="p" size="md">This panel is 860px wide, not the 800px that lg gives.</RlText>
      </RlModal>
    `,
  }),
  args: { title: 'A width of its own', size: 'lg' },
}

/**
 * A dialog that raises its own confirm has to sit below it. Both teleport to
 * the body, so at one z-index the later one in the DOM wins, which is the
 * confirm only by accident. `layer` and `panelClass` settle it on purpose.
 */
export const Layered: Story = {
  render: () => ({
    components: { RlModal, RlButton, RlButtonGroup, RlText },
    setup: () => ({ form: ref(true), confirm: ref(false) }),
    template: `
      <div>
        <RlButton variant="primary" @click="form = true">Open form</RlButton>

        <RlModal
          title="Edit entity"
          :open="form"
          :layer="900"
          panel-class="demo-form"
          @close="form = false"
        >
          <RlText as="p" size="md">Discarding asks first, and the question sits on top.</RlText>
          <template #actions>
            <RlButtonGroup>
              <RlButton variant="secondary" @click="confirm = true">Discard</RlButton>
              <RlButton variant="primary" @click="form = false">Save</RlButton>
            </RlButtonGroup>
          </template>
        </RlModal>

        <RlModal
          title="Discard your changes?"
          role="alertdialog"
          size="sm"
          :open="confirm"
          :layer="1000"
          @close="confirm = false"
        >
          <RlText as="p" size="md">The edits you made will not be kept.</RlText>
          <template #actions>
            <RlButtonGroup>
              <RlButton variant="secondary" @click="confirm = false">Keep editing</RlButton>
              <RlButton variant="primary" tone="danger" @click="confirm = false; form = false">Discard</RlButton>
            </RlButtonGroup>
          </template>
        </RlModal>
      </div>
    `,
  }),
}

/** Shown open so the layout is visible without interaction. */
export const Open: Story = {
  render: (args) => ({
    components: { RlModal, RlText, RlButton, RlButtonGroup },
    setup: () => ({ args }),
    template: `
      <RlModal v-bind="args" :open="true">
        <RlText as="p" size="md">The body scrolls; the title and actions stay put.</RlText>
        <template #actions>
          <RlButtonGroup>
            <RlButton variant="secondary">Cancel</RlButton>
            <RlButton variant="primary">Save</RlButton>
          </RlButtonGroup>
        </template>
      </RlModal>
    `,
  }),
}
