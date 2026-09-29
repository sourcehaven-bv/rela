import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlDetailField from './RlDetailField.vue'
import { ref } from 'vue'
import type { DetailField } from '../../types'
import RlTag from '../common/RlTag.vue'
import RlIcon from '../common/RlIcon.vue'
import RlCommentIndicator from '../comment/RlCommentIndicator.vue'
import { detailFields, statusOptions } from '../../fixtures'

const meta: Meta<typeof RlDetailField> = {
  title: 'Task/Detail field',
  component: RlDetailField,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlDetailField>

const inList = () => ({ template: '<dl style="margin:0"><story /></dl>' })

export const Status: Story = {
  decorators: [inList],
  args: { field: { id: 's', label: 'Status', type: 'status', value: 'In progress', status: 'green' } },
}

export const Text: Story = {
  decorators: [inList],
  args: { field: { id: 'a', label: 'Assignee', type: 'text', value: 'Jeroen' } },
}

export const Tags: Story = {
  decorators: [inList],
  args: {
    field: {
      id: 't',
      label: 'Labels',
      type: 'tags',
      tags: [
        { id: '1', label: 'Design', color: 'blue' },
        { id: '2', label: 'Urgent', color: 'red' },
      ],
    },
  },
}

/** Every field type from the fixtures, as the detail view stacks them. */
export const AllTypes: Story = {
  render: () => ({
    components: { RlDetailField },
    setup: () => ({ fields: detailFields }),
    template: `
      <dl style="display:flex; flex-direction:column; gap:12px; margin:0">
        <RlDetailField v-for="f in fields" :key="f.id" :field="f" />
      </dl>
    `,
  }),
}

/**
 * With `editable`, each value is click-to-edit, and each type gets the
 * control that fits it: a status and a priority pick from a drawn list that
 * keeps the dot and the badge, a date gets the date picker, and text gets a
 * text box. Escape drops the edit in every case.
 */
export const Editable: Story = {
  render: () => ({
    components: { RlDetailField },
    setup() {
      const fields = ref<DetailField[]>(detailFields.map((field) => ({ ...field })))
      function update(next: DetailField) {
        const index = fields.value.findIndex((field) => field.id === next.id)
        if (index !== -1) fields.value[index] = next
      }
      const priorityOptions = [
        { id: 'low', label: 'Low', color: 'grey' as const },
        { id: 'medium', label: 'Medium', color: 'blue' as const },
        { id: 'high', label: 'High', color: 'green' as const },
        { id: 'urgent', label: 'Urgent', color: 'red' as const },
      ]
      return { fields, statusOptions, priorityOptions, update }
    },
    template: `
      <dl style="display:flex; flex-direction:column; gap:12px; margin:0; max-width:420px">
        <RlDetailField
          v-for="f in fields"
          :key="f.id"
          :field="f"
          editable
          :options="statusOptions"
          :tag-options="priorityOptions"
          @update:field="update"
        />
      </dl>
    `,
  }),
}

/**
 * The `value` slot hands the whole value column to the caller, keeping the
 * row: the label column, the row height, the wrapping and what happens at a
 * narrow width. For an app with its own registry of property widgets, where
 * the built-in types cover a fraction of what a field can be.
 *
 * Here the two rows below the built-in one are the caller's own: a set of
 * tags with a count, and a rule read from a recurrence.
 */
export const CustomValue: Story = {
  render: () => ({
    components: { RlDetailField, RlTag, RlIcon },
    setup: () => ({
      status: { id: 's', label: 'Status', type: 'status' as const, value: 'In progress', status: 'green' as const },
      audiences: { id: 'a', label: 'Audience', type: 'text' as const },
      repeat: { id: 'r', label: 'Repeats', type: 'text' as const },
    }),
    template: `
      <dl style="display:flex; flex-direction:column; gap:12px; margin:0; max-width:420px">
        <RlDetailField :field="status" />

        <RlDetailField :field="audiences">
          <template #value>
            <RlTag label="Members" color="blue" />
            <RlTag label="Board" color="green" />
            <span style="color:var(--rl-color-text-subtle)">+2</span>
          </template>
        </RlDetailField>

        <RlDetailField :field="repeat">
          <template #value>
            <RlIcon name="refresh" :size="14" />
            <span>Every second Tuesday, until 31 Dec</span>
          </template>
        </RlDetailField>
      </dl>
    `,
  }),
}

/**
 * `label-trailing` renders after the label text, on the label's own line. It
 * is where a per-field comment marker goes: the label is a fixed point at any
 * width, while the end of the value moves with the value's length and reads
 * as belonging to whatever comes next.
 */
export const LabelTrailing: Story = {
  render: () => ({
    components: { RlDetailField, RlCommentIndicator },
    setup() {
      const anchor = { kind: 'property' as const, ref: 'owner', label: 'Owner' }
      return {
        owner: { id: 'o', label: 'Owner', type: 'text' as const, value: 'Hanna de Vries' },
        due: { id: 'd', label: 'Due date', type: 'date' as const, value: '22 sep' },
        anchor,
        comments: [
          {
            id: 'c1',
            author: 'Rowdy van Looy',
            timestamp: '22 Sep, 11:02',
            body: 'Should this be the team rather than one person?',
            anchor,
          },
        ],
      }
    },
    template: `
      <dl style="display:flex; flex-direction:column; gap:12px; margin:0; max-width:420px">
        <RlDetailField :field="owner">
          <template #label-trailing>
            <RlCommentIndicator :anchor="anchor" :comments="comments" />
          </template>
        </RlDetailField>

        <!-- No comments yet: the marker stays quiet until the row is hovered
             or the button is focused. -->
        <RlDetailField :field="due">
          <template #label-trailing>
            <RlCommentIndicator :anchor="{ kind: 'property', ref: 'due', label: 'Due date' }" />
          </template>
        </RlDetailField>
      </dl>
    `,
  }),
}

/**
 * `stacked` puts the value under its label across the full row. For content
 * too long to read in the value column, where the alternative is text
 * wrapping four times in a narrow column beside a one-word label.
 */
export const Stacked: Story = {
  render: () => ({
    components: { RlDetailField },
    setup: () => ({
      short: { id: 's', label: 'Owner', type: 'text' as const, value: 'Hanna de Vries' },
      long: {
        id: 'l',
        label: 'Summary',
        type: 'text' as const,
        value:
          'The board asked for a single view that shows which initiatives are ' +
          'at risk this quarter, who owns each one, and what the last thing ' +
          'that happened to it was, without opening four screens to find out.',
      },
    }),
    template: `
      <dl style="display:flex; flex-direction:column; gap:12px; margin:0; max-width:420px">
        <RlDetailField :field="short" />
        <RlDetailField :field="long" stacked editable />
      </dl>
    `,
  }),
}
