import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, waitFor } from 'storybook/test'
import { ref } from 'vue'
import RlMultiSelect from './RlMultiSelect.vue'
import RlTag from '../common/RlTag.vue'

const options = [
  { value: 'design', label: 'Design' },
  { value: 'frontend', label: 'Frontend' },
  { value: 'backend', label: 'Backend' },
  { value: 'research', label: 'Research' },
  { value: 'infra', label: 'Infrastructure' },
]

const meta: Meta<typeof RlMultiSelect> = {
  title: 'Form/Multi select',
  component: RlMultiSelect,
  parameters: { layout: 'padded' },
  args: { label: 'Labels', options },
  render: (args) => ({
    components: { RlMultiSelect },
    setup: () => ({ args, value: ref(args.modelValue ?? []) }),
    // Room below, so the open panel is not clipped by the story frame.
    template:
      '<div style="max-width:320px; padding-bottom:260px"><RlMultiSelect v-bind="args" v-model="value" /></div>',
  }),
}
export default meta
type Story = StoryObj<typeof RlMultiSelect>

/** Click the field to open the list. Tabbing past the last option closes it. */
export const Playground: Story = {}

/** Chosen values stay visible as chips, so the field reports its own state. */
export const WithSelection: Story = { args: { modelValue: ['design', 'frontend'] } }

/**
 * The `option` slot draws each choice, in the panel and in the trigger both,
 * so a tag keeps its colour while it is being chosen and after it has been.
 * Without it the label is drawn as a plain chip, which is what the stories
 * above show.
 */
export const DrawnOptions: Story = {
  render: () => ({
    components: { RlMultiSelect, RlTag },
    setup() {
      const tags = [
        { value: 'design', label: 'Design', color: 'blue' as const },
        { value: 'urgent', label: 'Urgent', color: 'red' as const },
        { value: 'research', label: 'Research', color: 'green' as const },
        { value: 'infra', label: 'Infrastructure', color: 'grey' as const },
      ]
      const value = ref(['design', 'urgent'])
      const colorOf = (v: string) => tags.find((tag) => tag.value === v)?.color
      return { tags, value, colorOf }
    },
    template: `
      <div style="max-width:320px; padding-bottom:260px">
        <RlMultiSelect v-model="value" label="Labels" :options="tags">
          <template #option="{ option }">
            <RlTag :label="option.label" :color="colorOf(option.value)" />
          </template>
        </RlMultiSelect>
      </div>
    `,
  }),
}

/**
 * `variant="inline"` is the form for a value sitting in a record rather than
 * a form: the values alone until hovered, then the button chrome, and one
 * click to open. It drops the field shell with it, since the row an inline
 * value sits in has already said what the field is.
 *
 * It matches `RlOptionSelect`'s inline variant, so a single-value and a
 * multi-value property read the same way in the same column.
 */
export const Inline: Story = {
  render: () => ({
    components: { RlMultiSelect, RlTag },
    setup() {
      const tags = [
        { value: 'design', label: 'Design', color: 'blue' as const },
        { value: 'urgent', label: 'Urgent', color: 'red' as const },
        { value: 'research', label: 'Research', color: 'green' as const },
      ]
      const value = ref(['design'])
      const colorOf = (v: string) => tags.find((tag) => tag.value === v)?.color
      return { tags, value, colorOf }
    },
    template: `
      <dl style="display:flex; align-items:center; gap:16px; margin:0; padding-bottom:260px">
        <dt style="width:120px; font-size:14px; color:var(--rl-color-text-muted)">Labels</dt>
        <dd style="margin:0">
          <RlMultiSelect v-model="value" label="Labels" :options="tags" variant="inline">
            <template #option="{ option }">
              <RlTag :label="option.label" :color="colorOf(option.value)" />
            </template>
          </RlMultiSelect>
        </dd>
      </dl>
    `,
  }),
}

/**
 * `isOptionDisabled` closes a choice by a rule that is not a property of the
 * choice itself, such as a permission. It layers over the option's own
 * `disabled`, which stays for a list that already knows.
 *
 * A closed value that is already chosen keeps showing and cannot be unticked:
 * a rule that forbids picking something forbids unpicking it too. Here
 * "Design" is both chosen and closed.
 */
export const DisabledOptions: Story = {
  render: () => ({
    components: { RlMultiSelect, RlTag },
    setup() {
      const tags = [
        { value: 'design', label: 'Design', color: 'blue' as const },
        { value: 'urgent', label: 'Urgent', color: 'red' as const },
        { value: 'research', label: 'Research', color: 'green' as const },
        { value: 'infra', label: 'Infrastructure', color: 'grey' as const, disabled: true },
      ]
      const value = ref(['design'])
      const locked = ['design', 'urgent']
      const colorOf = (v: string) => tags.find((tag) => tag.value === v)?.color
      return { tags, value, colorOf, isOptionDisabled: (v: string) => locked.includes(v) }
    },
    template: `
      <div style="max-width:320px; padding-bottom:260px">
        <RlMultiSelect
          v-model="value"
          label="Labels"
          :options="tags"
          :is-option-disabled="isOptionDisabled"
        >
          <template #option="{ option }">
            <RlTag :label="option.label" :color="colorOf(option.value)" />
          </template>
        </RlMultiSelect>
      </div>
    `,
  }),
}

/**
 * `update:modelValue` fires on every tick, which is the default and right for
 * a form. `close` fires once when the panel closes, however it closed, for a
 * caller that would rather save once per visit than once per checkbox.
 *
 * The counts below show the difference.
 */
export const SaveOnClose: Story = {
  render: () => ({
    components: { RlMultiSelect },
    setup() {
      const value = ref<string[]>(['design'])
      const toggles = ref(0)
      const saves = ref(0)
      return { options, value, toggles, saves }
    },
    template: `
      <div style="max-width:320px; padding-bottom:260px">
        <RlMultiSelect
          v-model="value"
          label="Labels"
          :options="options"
          @update:model-value="toggles++"
          @close="saves++"
        />
        <p style="margin-top:12px; font-size:14px; color:var(--rl-color-text-muted)">
          {{ toggles }} toggles, {{ saves }} saves
        </p>
      </div>
    `,
  }),
}

/**
 * Inline with nothing chosen yet. The trigger is at its widest here, since
 * the placeholder is longer than most values, which is what makes it the
 * case to check: the chevron has to sit inside the background rather than
 * beside it.
 */
export const InlineEmpty: Story = {
  render: () => ({
    components: { RlMultiSelect, RlTag },
    setup() {
      const tags = [
        { value: 'design', label: 'Design', color: 'blue' as const },
        { value: 'urgent', label: 'Urgent', color: 'red' as const },
        { value: 'research', label: 'Research', color: 'green' as const },
      ]
      const value = ref<string[]>([])
      const colorOf = (v: string) => tags.find((tag) => tag.value === v)?.color
      return { tags, value, colorOf }
    },
    template: `
      <dl style="display:flex; align-items:center; gap:16px; margin:0; padding-bottom:260px">
        <dt style="width:120px; font-size:14px; color:var(--rl-color-text-muted)">Labels</dt>
        <dd style="margin:0">
          <RlMultiSelect v-model="value" label="Labels" :options="tags" variant="inline">
            <template #option="{ option }">
              <RlTag :label="option.label" :color="colorOf(option.value)" />
            </template>
          </RlMultiSelect>
        </dd>
      </dl>
    `,
  }),
}

/**
 * A trigger narrower than the options it opens, which is the shape a filter
 * bar has: the field reads "All" and the options read "Visual regression".
 *
 * The panel takes the trigger's width as a floor and grows to fit its
 * content, so the options stay on one line each. Matching the trigger
 * exactly would squeeze every option to fit a word the user did not choose.
 */
export const NarrowTrigger: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(canvasElement.querySelector('.rl-multi__trigger')!)

    const panel = await waitFor(() => {
      const found = document.querySelector('.rl-panel--options')
      if (!found) throw new Error('panel did not open')
      return found
    })

    const trigger = canvasElement.querySelector('.rl-multi__trigger')!
    const triggerWidth = trigger.getBoundingClientRect().width
    const panelWidth = panel.getBoundingClientRect().width

    /* At least the field, and wider than it, because the options need it. */
    await expect(panelWidth).toBeGreaterThanOrEqual(triggerWidth)
    await expect(panelWidth).toBeGreaterThan(triggerWidth)

    /*
     * Asserted on the rows rather than on the panel's scrollWidth: an option
     * row shrinks to the panel instead of overflowing it, so the panel
     * reports no overflow while the labels wrap. Row height is what actually
     * shows the squeeze.
     */
    const rows = [...panel.querySelectorAll('.rl-multi__option')]
    await expect(rows.length).toBe(3)
    const tallest = Math.max(...rows.map((row) => row.getBoundingClientRect().height))
    const shortest = Math.min(...rows.map((row) => row.getBoundingClientRect().height))
    await expect(tallest).toBe(shortest)
  },
  render: () => ({
    components: { RlMultiSelect },
    setup: () => ({
      value: ref([]),
      opts: [
        { value: 'arch', label: 'Architecture' },
        { value: 'feat', label: 'Feature request' },
        { value: 'visual', label: 'Visual regression' },
      ],
    }),
    template: `<div style="width:130px; padding-bottom:260px">
      <RlMultiSelect v-model="value" label="All" :options="opts" />
    </div>`,
  }),
}
