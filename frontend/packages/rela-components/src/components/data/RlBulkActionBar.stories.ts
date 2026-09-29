import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, waitFor, within } from 'storybook/test'
import { computed, ref } from 'vue'
import RlBulkActionBar from './RlBulkActionBar.vue'
import RlButton from '../common/RlButton.vue'
import RlTable from '../../fixtures/RlTaskTable.vue'
import { tableSections, tableColumns } from '../../fixtures'

const meta: Meta<typeof RlBulkActionBar> = {
  title: 'Data/Bulk action bar',
  component: RlBulkActionBar,
  parameters: { layout: 'fullscreen' },
}
export default meta

/*
 * Wired to a real table, because the bar's whole job is to be the other half
 * of `selectedIds`: shown on its own it is a pill with a number in it.
 */
function setup() {
  const selected = ref(new Set<string>())

  const sections = computed(() => tableSections)
  const count = computed(() => selected.value.size)

  function toggle(item: { id: string }) {
    const next = new Set(selected.value)
    if (next.has(item.id)) next.delete(item.id)
    else next.add(item.id)
    selected.value = next
  }

  function toggleAll(section: { items: { id: string }[] }, checked: boolean) {
    const next = new Set(selected.value)
    for (const item of section.items) {
      if (checked) next.add(item.id)
      else next.delete(item.id)
    }
    selected.value = next
  }

  return {
    sections, selected, count, toggle, toggleAll,
    columns: tableColumns,
    clear: () => (selected.value = new Set()),
  }
}

const template = `
  <div style="position:relative; height:100vh; overflow:auto; background:var(--rl-color-bg)">
    <RlTable
      :sections="sections"
      :columns="columns"
      :selected-ids="selected"
      @toggle="toggle"
      @toggle-all="toggleAll"
    />

    <RlBulkActionBar :count="count" label="task" @clear="clear">
      <RlButton size="sm">Move</RlButton>
      <RlButton size="sm">Assign</RlButton>
      <RlButton size="sm" tone="danger">Delete</RlButton>
    </RlBulkActionBar>
  </div>
`

/**
 * The bar is the half of `selectedIds` that acts. Tick a row and it rises;
 * clear it and it goes.
 *
 * It floats rather than pushing the list down, because a bar that displaced
 * the rows would move the next row a user is reaching for.
 */
export const Default: StoryObj = {
  render: () => ({ components: { RlBulkActionBar, RlButton, RlTable }, setup, template }),
}

/**
 * Drives the path worth protecting: the bar is absent at zero, counts what
 * is ticked, says "1 task" rather than "1 tasks", and clearing puts it away
 * without touching the rows.
 */
export const CountsAndClears: StoryObj = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    /* Nothing selected, nothing to act on. */
    await expect(canvas.queryByText(/selected/)).toBeNull()

    /*
     * Picked by position rather than by name. The fixture repeats a title
     * across sections, so a name matches several boxes, and `RlCheckbox`
     * names itself with a real `<label>` rather than `aria-label`, so
     * reading the attribute finds nothing either. Position is the one thing
     * that identifies a single row here.
     */
    const rowBoxes = canvas
      .getAllByRole('checkbox')
      .filter((box) => !/^Select all /.test((box as HTMLInputElement).labels?.[0]?.textContent ?? ''))

    await userEvent.click(rowBoxes[0])
    await expect(await canvas.findByText('1 task selected')).toBeInTheDocument()

    await userEvent.click(rowBoxes[1])
    await expect(await canvas.findByText('2 tasks selected')).toBeInTheDocument()

    await userEvent.click(canvas.getByRole('button', { name: 'Clear selection' }))
    await waitFor(() => expect(canvas.queryByText(/selected/)).toBeNull())

    /* The rows are still there: clearing a selection is not a delete. */
    await expect(canvas.getAllByRole('row').length).toBeGreaterThan(1)
  },
  render: () => ({ components: { RlBulkActionBar, RlButton, RlTable }, setup, template }),
}
