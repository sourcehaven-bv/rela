import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { ref } from 'vue'
import RlOptionSelect from './RlOptionSelect.vue'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlTag from '../common/RlTag.vue'
import RlAvatar from '../data/RlAvatar.vue'
import { statusOptions } from '../../fixtures'

const meta: Meta<typeof RlOptionSelect> = {
  title: 'Form/Option select',
  component: RlOptionSelect,
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component:
          'A single-choice picker whose options are drawn rather than described. ' +
          'A native `select` can only hold text, so a status loses its colour and ' +
          'a priority loses its badge exactly when the user is choosing between ' +
          'them. Each option renders through a slot, so the open list shows the ' +
          'same thing the closed value shows.',
      },
    },
  },
}
export default meta
type Story = StoryObj<typeof RlOptionSelect>

/** A status keeps its dot in the list. */
export const Status: Story = {
  render: () => ({
    components: { RlOptionSelect, RlStatusDot },
    setup() {
      const value = ref('In progress')
      const colorOf = (label: string) =>
        statusOptions.find((option) => option.value === label)?.status
      return { value, values: statusOptions.map((option) => option.value), colorOf }
    },
    template: `
      <RlOptionSelect v-model="value" :options="values" label="Status">
        <template #option="{ value: option }">
          <RlStatusDot :color="colorOf(option)" />
          <span style="white-space:nowrap">{{ option }}</span>
        </template>
      </RlOptionSelect>
    `,
  }),
}

/** A priority shows the badge itself, not its label. */
export const Badges: Story = {
  render: () => ({
    components: { RlOptionSelect, RlTag },
    setup() {
      const tags = [
        { label: 'Low', color: 'grey' as const },
        { label: 'Medium', color: 'blue' as const },
        { label: 'High', color: 'green' as const },
        { label: 'Urgent', color: 'red' as const },
      ]
      const value = ref('High')
      const colorOf = (label: string) => tags.find((tag) => tag.label === label)?.color
      return { value, values: tags.map((tag) => tag.label), colorOf }
    },
    template: `
      <RlOptionSelect v-model="value" :options="values" label="Priority">
        <template #option="{ value: option }">
          <RlTag :label="option" :color="colorOf(option)" />
        </template>
      </RlOptionSelect>
    `,
  }),
}

/** Anything can go in a row, so a person can be picked by face and name. */
export const People: Story = {
  render: () => ({
    components: { RlOptionSelect, RlAvatar },
    setup: () => ({
      value: ref('Rowdy Bakker'),
      values: ['Rowdy Bakker', 'Hanna de Vries', 'Alex Jansen', 'Jeroen Vloothuis'],
    }),
    template: `
      <RlOptionSelect v-model="value" :options="values" label="Assignee" match-width>
        <template #option="{ value: option }">
          <RlAvatar :name="option" size="xs" decorative />
          <span style="white-space:nowrap">{{ option }}</span>
        </template>
      </RlOptionSelect>
    `,
  }),
}

/** With nothing chosen the trigger shows its placeholder. */
export const Empty: Story = {
  render: () => ({
    components: { RlOptionSelect, RlTag },
    setup: () => ({ value: ref(''), values: ['Low', 'Medium', 'High'] }),
    template: `
      <RlOptionSelect v-model="value" :options="values" label="Priority" placeholder="Set a priority">
        <template #option="{ value: option }">
          <RlTag :label="option" />
        </template>
      </RlOptionSelect>
    `,
  }),
}

/**
 * `variant="inline"` is the form for a value sitting in a record rather than
 * a form: bare until hovered, then the button chrome. The control is the same
 * either way, so a click opens the list directly with nothing swapped in
 * first.
 */
export const Inline: Story = {
  render: () => ({
    components: { RlOptionSelect, RlStatusDot },
    setup() {
      const value = ref('In progress')
      const colorOf = (label: string) =>
        statusOptions.find((option) => option.value === label)?.status
      return { value, values: statusOptions.map((option) => option.value), colorOf }
    },
    template: `
      <dl style="display:flex; align-items:center; gap:16px; margin:0">
        <dt style="width:120px; font-size:14px; color:var(--rl-color-text-muted)">Status</dt>
        <dd style="margin:0">
          <RlOptionSelect v-model="value" :options="values" label="Status" variant="inline">
            <template #option="{ value: option }">
              <RlStatusDot :color="colorOf(option)" />
              <span style="white-space:nowrap">{{ option }}</span>
            </template>
          </RlOptionSelect>
        </dd>
      </dl>
    `,
  }),
}

/**
 * `isOptionDisabled` greys a choice a rule forbids while leaving it in the
 * list. Hiding it would leave the user hunting for something that is not
 * there; this says the choice exists and is not theirs right now. The arrows
 * skip it, and Enter and a click do nothing on it.
 *
 * Here a task can only move forward: from "In progress" the states behind it
 * are closed, the way a transition rule or an ACL would close them.
 */
export const DisabledOptions: Story = {
  render: () => ({
    components: { RlOptionSelect, RlStatusDot },
    setup() {
      const values = statusOptions.map((option) => option.value)
      const value = ref('In progress')
      const colorOf = (label: string) =>
        statusOptions.find((option) => option.value === label)?.status
      // Anything at or before the current state is behind it, so it is shown
      // but closed. The current one stays choosable, since re-picking it is a
      // no-op rather than a move backwards.
      const isOptionDisabled = (option: string) =>
        values.indexOf(option) < values.indexOf(value.value)
      return { value, values, colorOf, isOptionDisabled }
    },
    template: `
      <RlOptionSelect
        v-model="value"
        :options="values"
        :is-option-disabled="isOptionDisabled"
        label="Status"
      >
        <template #option="{ value: option }">
          <RlStatusDot :color="colorOf(option)" />
          <span style="white-space:nowrap">{{ option }}</span>
        </template>
      </RlOptionSelect>
    `,
  }),
}

/**
 * An empty string is a fine option value, for a property that can be set back
 * to having no value. The slot draws it, so it reads as "None" rather than as
 * a blank row.
 *
 * One thing to know: the closed trigger treats an empty model value as
 * nothing chosen and shows the `placeholder` instead of running the slot. Set
 * the placeholder to the same words as the empty option and the two agree.
 */
export const EmptyOptionValue: Story = {
  render: () => ({
    components: { RlOptionSelect, RlTag },
    setup: () => ({ value: ref('Medium'), values: ['', 'Low', 'Medium', 'High'] }),
    template: `
      <RlOptionSelect v-model="value" :options="values" label="Priority" placeholder="None">
        <template #option="{ value: option }">
          <RlTag v-if="option" :label="option" />
          <span v-else style="color:var(--rl-color-text-subtle)">None</span>
        </template>
      </RlOptionSelect>
    `,
  }),
}
