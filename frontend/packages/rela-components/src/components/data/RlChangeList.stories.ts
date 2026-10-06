import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, within } from 'storybook/test'
import RlChangeList from './RlChangeList.vue'
import RlChangeItem from './RlChangeItem.vue'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlButton from '../common/RlButton.vue'

const meta: Meta<typeof RlChangeList> = {
  title: 'Data/Change list',
  component: RlChangeList,
  subcomponents: { RlChangeItem },
  parameters: { layout: 'padded' },
}
export default meta
type Story = StoryObj<typeof RlChangeList>

/**
 * Changes to configuration, grouped by the thing they belong to. Each row
 * names the change in the reader's terms; nothing here shows the file it ends
 * up in.
 */
export const ConfigurationDraft: Story = {
  render: () => ({
    components: { RlChangeList, RlChangeItem, RlStatusDot },
    template: `
      <div style="max-width:560px; display:flex; flex-direction:column; gap:24px">
        <RlChangeList title="Ticket" :count="2">
          <RlChangeItem kind="added" label="Field Due" detail="Date" as="button" />
          <RlChangeItem kind="changed" label="Field Effort" before="Text" after="Choice list" as="button" />
        </RlChangeList>
        <RlChangeList title="Effort values" :count="2">
          <RlChangeItem kind="added" label="Option XXL" as="button">
            <template #after><span style="display:inline-flex; align-items:center; gap:6px"><RlStatusDot color="red" />Red</span></template>
          </RlChangeItem>
          <RlChangeItem kind="removed" label="Option Unknown" as="button" />
        </RlChangeList>
      </div>
    `,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    // The kind and the from/to words are spoken, not only drawn.
    await expect(canvas.getByRole('button', { name: /Changed\s+Field Effort\s+from Text\s+to Choice list/ })).toBeInTheDocument()
    await expect(canvas.getByRole('heading', { name: 'Ticket' })).toBeInTheDocument()
  },
}

/**
 * The same rows, comparing two versions of one entity. This is the shape the
 * history view shows, which is why the component is not specific to
 * configuration.
 */
export const EntityHistory: Story = {
  render: () => ({
    components: { RlChangeList, RlChangeItem, RlStatusDot },
    template: `
      <div style="max-width:560px">
        <RlChangeList title="Version 7 → current">
          <RlChangeItem kind="changed" label="Status">
            <template #before><span style="display:inline-flex; align-items:center; gap:6px"><RlStatusDot color="blue" />Ready</span></template>
            <template #after><span style="display:inline-flex; align-items:center; gap:6px"><RlStatusDot color="amber" />In progress</span></template>
          </RlChangeItem>
          <RlChangeItem kind="added" label="Effort" after="M" />
          <RlChangeItem kind="removed" label="Blocked reason" before="Waiting on design" />
          <RlChangeItem kind="changed" label="Description" detail="3 paragraphs edited" />
        </RlChangeList>
      </div>
    `,
  }),
}

/** A bulk edit before it runs: one change, many targets, an action beside the title. */
export const BulkEditPreview: Story = {
  render: () => ({
    components: { RlChangeList, RlChangeItem, RlButton },
    template: `
      <div style="max-width:560px">
        <RlChangeList title="12 tickets">
          <template #actions><RlButton size="sm" variant="ghost">Show tickets</RlButton></template>
          <RlChangeItem kind="changed" label="Priority" before="Medium" after="High" />
          <RlChangeItem kind="added" label="Tag" after="needs-design" />
        </RlChangeList>
      </div>
    `,
  }),
}
