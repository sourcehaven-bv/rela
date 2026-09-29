import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, within } from 'storybook/test'
import { ref } from 'vue'
import RlPersonField from './RlPersonField.vue'
import type { Person } from '../data/types'

const meta: Meta<typeof RlPersonField> = {
  title: 'Form/PersonField',
  component: RlPersonField,
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlPersonField>

const people: Person[] = [
  { id: 'ada', name: 'Ada Lovelace', secondary: 'Engineering' },
  { id: 'grace', name: 'Grace Hopper', secondary: 'Engineering' },
  { id: 'jan-a', name: 'Jan de Vries', secondary: 'Design' },
  { id: 'jan-b', name: 'Jan Bakker', secondary: 'Support' },
  { id: 'katherine', name: 'Katherine Johnson', secondary: 'Research' },
  { id: 'alan', name: 'Alan Turing', secondary: 'On leave', disabled: true },
]

/**
 * Type to narrow the list, or arrow through it. The filter is in the trigger
 * rather than in the panel, because the panel keeps focus on the control: a
 * search box that cannot be typed into is worse than none.
 */
export const Default: Story = {
  render: () => ({
    components: { RlPersonField },
    setup: () => ({ people, value: ref('grace') }),
    template: `
      <div style="max-width:320px; display:flex; flex-direction:column; gap:16px">
        <RlPersonField v-model="value" label="Assignee" :people="people" />
        <RlPersonField label="Reviewer" :people="people" hint="Anyone on the team." />
      </div>
    `,
  }),
}

/**
 * In a record rather than a form: the value alone, with the chrome held back
 * until it is wanted. The same shape `RlOptionSelect` and `RlMultiSelect` use.
 */
export const Inline: Story = {
  render: () => ({
    components: { RlPersonField },
    setup: () => ({ people, value: ref('ada') }),
    template: `
      <dl style="display:grid; grid-template-columns:120px 1fr; align-items:center; gap:8px; max-width:360px">
        <dt style="font-size:14px; color:var(--rl-color-text-muted)">Assignee</dt>
        <dd style="margin:0">
          <RlPersonField v-model="value" label="Assignee" :people="people" variant="inline" />
        </dd>
      </dl>
    `,
  }),
}

/**
 * Two people called Jan are told apart by the line under the name, so the
 * filter reads that line as well as the name.
 */
export const FiltersOnSecondary: Story = {
  render: Default.render,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const field = canvas.getByRole('combobox', { name: 'Reviewer' })

    await userEvent.click(field)
    await userEvent.type(field, 'support')

    // Teleported to the body, so the panel is not inside the canvas.
    const list = within(document.body).getAllByRole('listbox', { name: 'Reviewer' })[0]
    const options = within(list).getAllByRole('option')

    await expect(options).toHaveLength(1)
    await expect(options[0]).toHaveTextContent('Jan Bakker')
  },
}

/**
 * Unassigning is a listed choice rather than a small x in the corner: it
 * carries the same weight as picking someone, and a corner button is a
 * reliable way to hide a decision from anyone not using a mouse.
 */
export const ClearsFromTheList: Story = {
  render: () => ({
    components: { RlPersonField },
    setup: () => ({ people, value: ref('grace') }),
    template: `
      <div style="max-width:320px">
        <RlPersonField v-model="value" label="Assignee" :people="people" />
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const field = canvas.getByRole('combobox', { name: 'Assignee' })

    await userEvent.click(field)

    const body = within(document.body)
    await userEvent.click(body.getByRole('option', { name: /Unassigned/ }))

    await expect(field).toHaveAttribute('aria-expanded', 'false')
    await expect(canvas.getByText('Unassigned')).toBeInTheDocument()
  },
}

/**
 * Someone a rule forbids is shown rather than hidden, so the list is the same
 * list for everyone. The arrows skip them and Enter does nothing on them.
 */
export const SkipsDisabled: Story = {
  render: ClearsFromTheList.render,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const field = canvas.getByRole('combobox', { name: 'Assignee' })

    await userEvent.click(field)
    const body = within(document.body)
    await expect(body.getByRole('option', { name: /Alan Turing/ })).toHaveAttribute(
      'aria-disabled',
      'true',
    )

    // From the end of the list, Up must pass over Alan and land on Katherine.
    await userEvent.keyboard('{End}')
    const active = () => field.getAttribute('aria-activedescendant')
    const landed = document.getElementById(active() ?? '')
    await expect(landed).toHaveTextContent('Katherine Johnson')
  },
}
