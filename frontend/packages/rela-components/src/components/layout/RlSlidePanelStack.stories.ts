import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { expect, userEvent, waitFor, within } from 'storybook/test'
import { computed, ref } from 'vue'
import RlSlidePanelStack, { type SlidePanel } from './RlSlidePanelStack.vue'
import RlSlidePanel from './RlSlidePanel.vue'
import RlPanelListRow from './RlPanelListRow.vue'
import RlSectionHeading from './RlSectionHeading.vue'

const meta: Meta<typeof RlSlidePanelStack> = {
  title: 'Layout/Slide panel stack',
  component: RlSlidePanelStack,
  parameters: { layout: 'fullscreen' },
}
export default meta

/** The rows of the first panel, grouped by when they are due. */
const groups = [
  {
    title: 'Today',
    rows: [
      { id: 't1', label: 'Review the Q4 initiative brief', meta: '07:00' },
      { id: 't2', label: 'Katten eten geven bij de buren', meta: '09:00' },
      { id: 't3', label: 'Invoices: fixed-fee projects', meta: '12:00' },
      { id: 't4', label: 'Pricing page: new iteration', meta: '13:00' },
    ],
  },
  {
    title: 'Tomorrow',
    rows: [
      { id: 't5', label: 'Docs: update screenshot', meta: '08:00' },
      { id: 't6', label: 'Time: search results ordering', meta: '17:00' },
      { id: 't7', label: 'Contact churned customers', meta: '18:00' },
    ],
  },
  {
    title: 'Next 7 days',
    rows: [
      { id: 't8', label: 'Migrate projects to templates', meta: 'Mon' },
      { id: 't9', label: 'QA regression before release', meta: 'Mon' },
      { id: 't10', label: 'Team feedback sessions', meta: 'Tue' },
      { id: 't11', label: 'Company tender ready', meta: 'Wed' },
    ],
  },
]

const allRows = groups.flatMap((group) => group.rows)

/*
 * The shape a consuming app has: the nav opens the list, a row in the list
 * opens the detail, and closing one reveals what is under it. The stack is
 * an array here, which is what makes a third panel one more entry rather
 * than a new component.
 */
function stackSetup() {
  const openList = ref(false)
  const openTaskId = ref<string | null>(null)
  const done = ref<Record<string, boolean>>({})

  const openTask = computed(() => allRows.find((row) => row.id === openTaskId.value))

  const panels = computed<SlidePanel[]>(() => {
    const result: SlidePanel[] = []
    if (openList.value) result.push({ id: 'my-tasks', title: 'My tasks', size: 'md' })
    if (openTask.value) {
      result.push({ id: 'task', title: openTask.value.label, size: 'lg' })
    }
    return result
  })

  /* Closing a panel closes what it opened, so the stack cannot outlive its own root. */
  function onClose(id: string) {
    if (id === 'my-tasks') {
      openList.value = false
      openTaskId.value = null
    } else {
      openTaskId.value = null
    }
  }

  return { openList, openTaskId, openTask, done, panels, onClose, groups }
}

/*
 * The stack positions itself against its containing block, so the wrapper it
 * goes in is what decides how far the panels reach. Here that wrapper starts
 * after the nav, so the panels slide out of the nav rather than over it,
 * which is the arrangement the nav trigger implies.
 */
const template = `
  <div style="height:100vh; display:flex; background:var(--rl-color-bg)">
    <nav style="width:220px; flex:none; padding:16px; border-right:1px solid var(--rl-color-border)">
      <button
        type="button"
        :aria-pressed="openList"
        style="width:100%; text-align:left; padding:8px; border:none; border-radius:6px;
               background:transparent; font:inherit; color:inherit; cursor:pointer"
        :style="openList ? 'background:var(--rl-color-bg-selected)' : ''"
        @click="openList = !openList"
      >My tasks</button>
    </nav>

    <div style="position:relative; flex:1; min-width:0">
      <main style="height:100%; padding:24px; overflow:auto; box-sizing:border-box">
        <h1 style="margin:0 0 8px; font-size:20px">Board</h1>
        <p style="color:var(--rl-color-text-subtle)">
          The page stays where it is and stays clickable. The panels sit over it.
        </p>
      </main>

      <RlSlidePanelStack :panels="panels" @close="onClose">
        <template #panel="{ panel }">
          <template v-if="panel.id === 'my-tasks'">
            <div v-for="group in groups" :key="group.title">
              <RlSectionHeading :title="group.title" :count="group.rows.length" />
              <RlPanelListRow
                v-for="row in group.rows"
                :key="row.id"
                :label="row.label"
                :meta="row.meta"
                checkable
                :checked="done[row.id] ?? false"
                :selected="openTaskId === row.id"
                @update:checked="done[row.id] = $event"
                @select="openTaskId = row.id"
              />
            </div>
          </template>

          <div v-else style="padding:20px">
            <p style="margin:0 0 12px; color:var(--rl-color-text-subtle)">
              Due {{ openTask?.meta }}
            </p>
            <p style="margin:0">
              The third panel would be one more entry in the array.
            </p>
          </div>
        </template>
      </RlSlidePanelStack>
    </div>
  </div>
`

/**
 * A click in the nav slides the list out over the page; a click on a row in
 * the list slides the detail out beside it. Closing either reveals what is
 * underneath, which is the page itself once the last one goes.
 */
export const Default: StoryObj = {
  render: () => ({
    components: { RlSlidePanelStack, RlPanelListRow, RlSectionHeading },
    setup: stackSetup,
    template,
  }),
}

/**
 * Drives the whole path, because the thing worth protecting here is the
 * order: opening two panels, closing the inner one and finding the outer one
 * still open is exactly what a stack must do and what a single panel that
 * merely swapped contents would fail.
 */
export const OpensAndCloses: StoryObj = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    await userEvent.click(canvas.getByRole('button', { name: 'My tasks' }))
    const list = await canvas.findByRole('complementary', { name: 'My tasks' })

    /* A row opens the second panel beside the first, which stays open. */
    await userEvent.click(within(list).getByRole('button', { name: /Q4 initiative brief/ }))
    await expect(
      canvas.getByRole('complementary', { name: /Q4 initiative brief/ }),
    ).toBeInTheDocument()
    await expect(canvas.getByRole('complementary', { name: 'My tasks' })).toBeInTheDocument()

    /*
     * Escape closes one level, not the stack.
     *
     * Asserted by naming the panels rather than by counting them: a closing
     * panel stays mounted for the length of its slide, so a count is racing
     * the transition that removes it.
     */
    await userEvent.keyboard('{Escape}')
    await expect(
      await canvas.findByRole('complementary', { name: 'My tasks' }),
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(canvas.queryByRole('complementary', { name: /Q4 initiative brief/ })).toBeNull(),
    )

    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(canvas.queryByRole('complementary')).toBeNull())
  },
  render: () => ({
    components: { RlSlidePanelStack, RlPanelListRow, RlSectionHeading },
    setup: stackSetup,
    template,
  }),
}

/**
 * Ticking a row off is not the same action as opening it, so the checkbox
 * keeps its own click even though the rest of the row opens the panel.
 */
export const CheckDoesNotOpen: StoryObj = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: 'My tasks' }))

    const list = await canvas.findByRole('complementary', { name: 'My tasks' })
    const check = within(list).getAllByRole('checkbox')[0]

    await userEvent.click(check)
    await expect(check).toBeChecked()
    await expect(canvas.getByRole('complementary', { name: 'My tasks' })).toBeInTheDocument()
    await expect(canvas.getAllByRole('complementary')).toHaveLength(1)
  },
  render: () => ({
    components: { RlSlidePanelStack, RlPanelListRow, RlSectionHeading },
    setup: stackSetup,
    template,
  }),
}

/** One panel on its own, which is what the stack repeats. */
export const Panel: StoryObj = {
  parameters: { layout: 'fullscreen' },
  render: () => ({
    components: { RlSlidePanel, RlPanelListRow, RlSectionHeading },
    setup: () => ({ group: groups[0] }),
    template: `
      <div style="position:relative; height:100vh; background:var(--rl-color-bg)">
        <RlSlidePanel title="My tasks">
          <RlSectionHeading :title="group.title" :count="group.rows.length" />
          <RlPanelListRow
            v-for="row in group.rows"
            :key="row.id"
            :label="row.label"
            :meta="row.meta"
            checkable
          />
        </RlSlidePanel>
      </div>
    `,
  }),
}

/**
 * With `manageFocus`, focus moves into each panel as it opens and returns to
 * whatever opened the stack once the last panel closes.
 *
 * Off by default, because these panels are read alongside the page and a
 * stack that took the keyboard would take it from work still in progress.
 * This story covers the opt-in path, including the return: a panel that
 * opened from a control has to hand the keyboard back to it.
 */
export const FocusManaged: StoryObj = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const trigger = canvas.getByRole('button', { name: 'My tasks' })

    await userEvent.click(trigger)
    const list = await canvas.findByRole('complementary', { name: 'My tasks' })
    await waitFor(() => expect(list).toHaveFocus())

    /* A second panel takes the keyboard from the first. */
    await userEvent.click(within(list).getByRole('button', { name: /Q4 initiative brief/ }))
    const detail = await canvas.findByRole('complementary', { name: /Q4 initiative brief/ })
    await waitFor(() => expect(detail).toHaveFocus())

    /* Closing the inner one hands it back to the panel beneath, not the page. */
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(list).toHaveFocus())

    /* Closing the last hands it back to the control that opened the stack. */
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(trigger).toHaveFocus())
  },
  render: () => ({
    components: { RlSlidePanelStack, RlPanelListRow, RlSectionHeading },
    setup: stackSetup,
    template: template.replace('<RlSlidePanelStack :panels="panels"', '<RlSlidePanelStack manage-focus :panels="panels"'),
  }),
}

/**
 * The header's own controls come from `#panel-actions`, called once per
 * panel so one slot can give each panel different controls. Expand is the
 * case this exists for: a panel that has a full page behind it offers a way
 * to go there.
 */
export const PanelActions: StoryObj = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await userEvent.click(canvas.getByRole('button', { name: 'My tasks' }))

    const list = await canvas.findByRole('complementary', { name: 'My tasks' })
    /*
     * Asserted inside the panel rather than on the page: the control has to
     * land in this panel's own header, and a query across the canvas would
     * pass even if it rendered somewhere else entirely.
     */
    await expect(within(list).getByRole('button', { name: 'Expand my-tasks' })).toBeInTheDocument()
    await expect(within(list).getByRole('button', { name: 'Close panel' })).toBeInTheDocument()
  },
  render: () => ({
    components: { RlSlidePanelStack, RlPanelListRow, RlSectionHeading },
    setup: stackSetup,
    template: template.replace(
      '<template #panel="{ panel }">',
      `<template #panel-actions="{ panel }">
         <button type="button">Expand {{ panel.id }}</button>
       </template>
       <template #panel="{ panel }">`,
    ),
  }),
}
