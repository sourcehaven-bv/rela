import { computed, ref } from 'vue'
import { expect, userEvent, within } from 'storybook/test'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlTable from './RlTable.vue'
import RlButton from '../common/RlButton.vue'
import type { Section, Task } from '../../types'
import RlCheckbox from '../form/RlCheckbox.vue'
import RlTagList from '../common/RlTagList.vue'
import RlStatusPill from '../common/RlStatusPill.vue'
import type { ColumnVisibility, RowMove, SortClickEvent, TableColumn, TableSort } from './types'
import { storyComponent } from '../storyGeneric'
import { tableSections, tableColumns } from '../../fixtures'

/*
 * The table is generic over its row. `Task` is this library's demo row, and
 * pinning it here is what the stories exercise; a consumer supplies its own.
 *
 * `storyComponent` handles registration, which a generic SFC's signature
 * cannot satisfy on its own; `Meta`/`StoryObj` still type every story's args.
 */
const meta: Meta<typeof RlTable<Task>> = {
  title: 'Table/Table',
  component: storyComponent(RlTable),
  /*
   * The props a reader would want to try against any story, rather than only
   * the ones a particular story is about. `compact` is the reason this list
   * exists: its two layouts differ only below a width threshold, so the way
   * to understand it is to switch it while looking at a narrow table.
   */
  argTypes: {
    compact: { control: { type: 'inline-radio' }, options: ['stack', 'compress'] },
    showHeaderRow: { control: 'boolean' },
    showSectionHeader: { control: 'boolean' },
    nameLabel: { control: 'text' },
    addLabel: { control: 'text' },
    pageSize: { control: { type: 'number', min: 0 } },
  },
}
export default meta

type Story = StoryObj<typeof RlTable<Task>>

export const WithColumns: Story = {
  args: { sections: tableSections, columns: tableColumns },
}

/** Narrow list variant used beside the detail panel: name column only. */
export const NameOnly: Story = {
  args: { sections: tableSections, columns: [] },
  render: (args) => ({
    components: { RlTable: storyComponent(RlTable) },
    setup: () => ({ args }),
    template: `<div style="width:432px; border-right:1px solid var(--rl-color-border)"><RlTable v-bind="args" /></div>`,
  }),
}

/**
 * The first column's header is whatever the view calls it. The default is
 * `Name`, so a table that never sets it is unchanged.
 */
export const CustomNameLabel: Story = {
  args: { sections: tableSections, columns: tableColumns, nameLabel: 'Ticket' },
}

/**
 * Sorting on several columns at once. The header owns the arrow and the
 * position number; the sort itself is the caller's, which is why this story
 * reorders the array rather than the rows.
 *
 * A plain press sorts by one column; shift-press adds a key. Either way a
 * column already in the sort cycles ascending, descending, then out of it —
 * that last step is what lets a key be dropped, and dropping the primary
 * promotes the one behind it.
 */
export const Sortable: Story = {
  render: () => ({
    components: { RlTable: storyComponent(RlTable) },
    setup() {
      const columns = tableColumns.map((column) => ({ ...column, sortable: true }))
      const sort = ref<TableSort[]>([
        { key: 'priority', dir: 'desc' },
        { key: 'dueDate', dir: 'asc' },
      ])

      /*
       * The conventional reading of the two gestures. A plain press sorts by
       * this column alone; a shift press adds it as the next tie-breaker.
       *
       * Either way a column already in the sort cycles ascending, then
       * descending, then out of it. The third step is what lets a key be
       * dropped without discarding the whole sort, and dropping the primary
       * is how the second key gets promoted.
       */
      function onSortClick(column: TableColumn, event: SortClickEvent) {
        const next = event.additive ? [...sort.value] : sort.value.filter(
          (entry) => entry.key === column.key,
        )
        const at = next.findIndex((entry) => entry.key === column.key)

        if (at === -1) sort.value = [...next, { key: column.key, dir: 'asc' }]
        else if (next[at].dir === 'asc')
          sort.value = next.map((entry, i) =>
            i === at ? { ...entry, dir: 'desc' as const } : entry,
          )
        else sort.value = next.filter((_, i) => i !== at)
      }

      return { sections: tableSections, columns, sort, onSortClick }
    },
    template: `
      <RlTable
        :sections="sections"
        :columns="columns"
        :sort="sort"
        @sort-click="onSortClick"
      />
    `,
  }),
}

/**
 * Multi-row selection with a bulk-action bar. Passing `selectedIds` at all is
 * what turns the select column on, so a table without it is the plain list.
 *
 * The bar takes the column headers' place while a section has rows checked,
 * because the actions apply to the selection rather than to the columns.
 */
export const Selection: Story = {
  render: () => ({
    components: { RlTable: storyComponent(RlTable), RlButton },
    setup() {
      const selectedIds = ref(new Set<string>())

      function onToggle(item: Task) {
        const next = new Set(selectedIds.value)
        if (next.has(item.id)) next.delete(item.id)
        else next.add(item.id)
        selectedIds.value = next
      }

      function onToggleAll(section: Section<Task>, checked: boolean) {
        const next = new Set(selectedIds.value)
        for (const item of section.items) {
          if (checked) next.add(item.id)
          else next.delete(item.id)
        }
        selectedIds.value = next
      }

      return { sections: tableSections, columns: tableColumns, selectedIds, onToggle, onToggleAll }
    },
    template: `
      <RlTable
        :sections="sections"
        :columns="columns"
        :selected-ids="selectedIds"
        @toggle="onToggle"
        @toggle-all="onToggleAll"
      >
        <template #bulk="{ count }">
          <strong>{{ count }} selected</strong>
          <RlButton size="sm" variant="secondary">Assign</RlButton>
          <RlButton size="sm" variant="secondary" tone="danger">Delete</RlButton>
        </template>
      </RlTable>
    `,
  }),
}

/**
 * Column visibility is state, not a flag on the column, so one record serves
 * a columns menu, a saved view and a narrow screen at once. A column absent
 * from the record is showing, so `{}` shows everything.
 *
 * `hideable: false` pins a column visible: here the assignee cannot be
 * turned off, so a stale saved view cannot hide it.
 */
export const ColumnVisibilityStory: Story = {
  name: 'Column Visibility',
  render: () => ({
    components: { RlTable: storyComponent(RlTable), RlCheckbox },
    setup() {
      const columns: TableColumn[] = [
        { ...tableColumns[0], hideable: false },
        ...tableColumns.slice(1),
      ]
      const columnVisibility = ref<ColumnVisibility>({ label: false })

      function toggle(key: string, shown: boolean) {
        columnVisibility.value = { ...columnVisibility.value, [key]: shown }
      }

      return { sections: tableSections, columns, columnVisibility, toggle }
    },
    template: `
      <div>
        <div style="display:flex; gap:16px; padding:8px 16px; flex-wrap:wrap">
          <RlCheckbox
            v-for="column in columns"
            :key="column.key"
            :label="column.header + (column.hideable === false ? ' (pinned)' : '')"
            :disabled="column.hideable === false"
            :model-value="columnVisibility[column.key] !== false"
            @update:model-value="toggle(column.key, $event)"
          />
        </div>
        <RlTable
          :sections="sections"
          :columns="columns"
          :column-visibility="columnVisibility"
        />
      </div>
    `,
  }),
}

/** `align: 'end'` trails a cell, so a column of numbers lines up. */
export const AlignedColumns: Story = {
  args: {
    sections: tableSections,
    columns: [
      { key: 'assignee', header: 'Assignee', field: 'assignee', width: 128 },
      { key: 'dueDate', header: 'Due date', field: 'dueDate', width: 128, align: 'end' as const },
    ],
  },
}

/**
 * A row that navigates. The `name` slot replaces the row's primary control,
 * so a list of entities can use real links: middle click and cmd-click open a
 * tab, right click offers copy-link, and the target shows in the status bar.
 * A button gives up all four.
 *
 * The caller writes a plain anchor. The row stretches it across the full row
 * and takes care of truncation and the focus ring, so there is no class to
 * remember and no second overlay to fight with the row's own.
 */
export const RowsAsLinks: Story = {
  render: () => ({
    components: { RlTable: storyComponent(RlTable) },
    setup: () => ({ sections: tableSections, columns: tableColumns }),
    template: `
      <RlTable :sections="sections" :columns="columns">
        <template #name="{ item }">
          <a :href="'#' + item.id">{{ item.title }}</a>
        </template>
      </RlTable>
    `,
  }),
}


/**
 * A flat list: one unnamed section, with the section header suppressed.
 *
 * A heading, a count, a collapse toggle and an "Add to <section>" button all
 * describe a grouping that a flat list does not have. Hiding the header also
 * moves the column header up to the top of the scroll area, which it must:
 * left where it was it would float over the first row and swallow its clicks.
 */
export const FlatList: Story = {
  args: {
    sections: [{ id: 'all', title: '', items: tableSections[0].items }],
    columns: tableColumns,
    showSectionHeader: false,
  },
}

/**
 * A sortable first column. Sorting a list by its title is ordinary, so the
 * name column takes the same control as any other once `name-column` gives it
 * a key to sort by.
 */
export const SortableNameColumn: Story = {
  render: () => ({
    components: { RlTable: storyComponent(RlTable) },
    setup() {
      const nameColumn: TableColumn = { key: 'title', header: 'Task', sortable: true }
      const sort = ref<TableSort[]>([{ key: 'title', dir: 'asc' }])

      function onSortClick(column: TableColumn) {
        const at = sort.value.findIndex((entry) => entry.key === column.key)
        if (at === -1) sort.value = [{ key: column.key, dir: 'asc' }]
        else if (sort.value[at].dir === 'asc')
          sort.value = [{ key: column.key, dir: 'desc' }]
        else sort.value = []
      }

      return { sections: tableSections, columns: tableColumns, nameColumn, sort, onSortClick }
    },
    template: `
      <RlTable
        :sections="sections"
        :columns="columns"
        :name-column="nameColumn"
        :sort="sort"
        @sort-click="onSortClick"
      />
    `,
  }),
}

/**
 * A keyboard cursor, separate from the row whose detail is open.
 *
 * Click into the list, then use `j`/`k` (or the arrow keys) to move the ring
 * and Enter to bring the panel to it. The ring and the filled row come apart
 * as soon as you move: that gap is the whole point of the prop, and it is why
 * a cursor cannot be carried in `selectedId`.
 *
 * Moving the cursor is the caller's job. What `j` does at the end of a
 * section — stop, wrap, or cross into the next one — is a property of the
 * list, not of the table, so the table paints the cursor and announces it and
 * leaves the arithmetic here.
 */
export const KeyboardCursor: Story = {
  render: () => ({
    components: { RlTable: storyComponent(RlTable) },
    setup() {
      const order = tableSections.flatMap((section) => section.items.map((item) => item.id))
      const cursorId = ref(order[0])
      const selectedId = ref(order[0])

      /* Clamped rather than wrapped, so the ends of the list stay findable. */
      function move(step: number) {
        const at = order.indexOf(cursorId.value)
        cursorId.value = order[Math.min(order.length - 1, Math.max(0, at + step))]
      }

      function onKeydown(event: KeyboardEvent) {
        const step = { j: 1, ArrowDown: 1, k: -1, ArrowUp: -1 }[event.key]
        if (step) {
          // Or the arrow keys scroll the list out from under the cursor.
          event.preventDefault()
          move(step)
        } else if (event.key === 'Enter') {
          selectedId.value = cursorId.value
        }
      }

      return { sections: tableSections, columns: tableColumns, cursorId, selectedId, onKeydown }
    },
    /*
     * `tabindex="0"` on the wrapper, not on each row: focus stays put and the
     * cursor row is named through `aria-activedescendant`, so `j` held down
     * does not drag focus through a hundred rows.
     */
    template: `
      <div tabindex="0" style="outline:none" @keydown="onKeydown">
        <RlTable
          :sections="sections"
          :columns="columns"
          :cursor-id="cursorId"
          :selected-id="selectedId"
        />
      </div>
    `,
  }),
}

/**
 * All three row states at once, which is the case the separate props exist
 * for: two rows checked for a bulk action, a third open in the panel, and the
 * cursor resting on a fourth. The fill, the accent edge and the ring have to
 * stay tellable apart when they land on the same row, so the cursor is a ring
 * rather than another background.
 */
export const CursorWithSelection: Story = {
  render: () => ({
    components: { RlTable: storyComponent(RlTable) },
    setup() {
      const ids = tableSections[0].items.map((item) => item.id)
      return {
        sections: tableSections,
        columns: tableColumns,
        selectedIds: new Set([ids[0], ids[1]]),
        selectedId: ids[2],
        cursorId: ids[3],
      }
    },
    template: `
      <RlTable
        :sections="sections"
        :columns="columns"
        :selected-ids="selectedIds"
        :selected-id="selectedId"
        :cursor-id="cursorId"
      />
    `,
  }),
}

/** The cursor and the open row on the same row: the ring reads over the fill. */
export const CursorOnOpenRow: Story = {
  render: () => ({
    components: { RlTable: storyComponent(RlTable) },
    setup() {
      const id = tableSections[0].items[0].id
      return { sections: tableSections, columns: tableColumns, selectedId: id, cursorId: id }
    },
    template: `
      <RlTable
        :sections="sections"
        :columns="columns"
        :selected-id="selectedId"
        :cursor-id="cursorId"
      />
    `,
  }),
}

/**
 * The two narrow layouts side by side, at a width where both have triggered.
 *
 * `stack`, the default, gives every populated cell its own labelled line, so
 * the row keeps all four columns and grows tall. `compress` keeps the row on
 * one line and drops every column that is not `primary`; here that leaves the
 * title and the assignee.
 *
 * The choice is about what the narrow table is. A phone showing nothing else
 * wants `stack`, because the row is the only place the data is. A master list
 * beside an open detail panel wants `compress`: the panel is already showing
 * those fields, so stacking them repeats it at three times the height.
 *
 * Both are keyed to the table's own width, not the window's, which is why
 * these two panes stack without the viewport changing at all.
 */
export const CompactModes: Story = {
  render: () => ({
    components: { RlTable: storyComponent(RlTable) },
    setup() {
      const columns = tableColumns.map((column) => ({
        ...column,
        primary: column.key === 'assignee',
      }))
      return { sections: tableSections, columns }
    },
    template: `
      <div style="display:flex; gap:var(--rl-space-6); align-items:flex-start">
        <div style="width:420px; border:1px solid var(--rl-color-border)">
          <p style="padding:var(--rl-space-2) var(--rl-space-4); margin:0; font-size:var(--rl-font-size-sm)">
            compact="stack"
          </p>
          <RlTable :sections="sections" :columns="columns" compact="stack" />
        </div>
        <div style="width:420px; border:1px solid var(--rl-color-border)">
          <p style="padding:var(--rl-space-2) var(--rl-space-4); margin:0; font-size:var(--rl-font-size-sm)">
            compact="compress"
          </p>
          <RlTable :sections="sections" :columns="columns" compact="compress" />
        </div>
      </div>
    `,
  }),
}

/**
 * `compress` with nothing marked `primary`: the narrow row is the title
 * alone. This is the sensible starting point for a master list, since the
 * caller then adds back only what a glance actually needs.
 */
export const CompressToTitle: Story = {
  args: { sections: tableSections, columns: tableColumns, compact: 'compress' },
  render: (args) => ({
    components: { RlTable: storyComponent(RlTable) },
    setup: () => ({ args }),
    template: `<div style="width:420px; border-right:1px solid var(--rl-color-border)"><RlTable v-bind="args" /></div>`,
  }),
}

/**
 * A playground for the props that change the table's shape.
 *
 * The width slider is the control that makes the rest legible: `compact` does
 * nothing until the table is narrower than 767px, and the table measures
 * itself rather than the window, so this story reaches the narrow layouts by
 * resizing the pane instead of the browser. Drag it past the threshold and
 * switch `compact` to see the two answers against the same rows.
 *
 * Selection is live, so checking a row brings up the bulk bar. That bar
 * replaces the column headers at every width, which is why it is worth
 * watching while the table is narrow: the headers are gone there and the bar
 * is the only place the actions live.
 */
export const Playground: Story = {
  args: {
    sections: tableSections,
    compact: 'stack',
    showHeaderRow: true,
    showSectionHeader: true,
    nameLabel: 'Name',
    addLabel: 'Add',
    pageSize: 0,
  },
  argTypes: {
    /* Columns and sections are the story's own, so the panel does not offer them. */
    sections: { table: { disable: true } },
    columns: { table: { disable: true } },
    selectedIds: { table: { disable: true } },
  },
  render: (args) => ({
    components: {
      RlTable: storyComponent(RlTable),
      RlButton,
      RlCheckbox,
      RlTagList,
      RlStatusPill,
    },
    setup() {
      const width = ref(420)
      const selectable = ref(true)
      const selectedIds = ref(new Set<string>())

      /*
       * Which columns survive `compress`, held here rather than on the
       * fixture: the point of the story is to watch a column come and go, and
       * `primary` is the flag that decides it.
       */
      const primary = ref<Record<string, boolean>>({ assignee: true })

      /*
       * `isEmpty` for the two columns whose value is not a string. The row
       * cannot see inside a cell slot, so without this it treats a task with
       * no tags as filled and a stacked card shows a "Label" with nothing
       * after it.
       */
      const columns = computed(() =>
        tableColumns.map((column) => ({
          ...column,
          primary: primary.value[column.key] === true,
          isEmpty:
            column.key === 'label'
              ? (item: Task) => !item.tags?.length
              : column.key === 'priority'
                ? (item: Task) => !item.priority
                : undefined,
        })),
      )

      function onToggle(item: Task) {
        const next = new Set(selectedIds.value)
        if (next.has(item.id)) next.delete(item.id)
        else next.add(item.id)
        selectedIds.value = next
      }

      function onToggleAll(section: Section<Task>, checked: boolean) {
        const next = new Set(selectedIds.value)
        for (const item of section.items) {
          if (checked) next.add(item.id)
          else next.delete(item.id)
        }
        selectedIds.value = next
      }

      return { args, width, columns, primary, selectable, selectedIds, onToggle, onToggleAll }
    },
    template: `
      <div style="display:flex; flex-direction:column; gap:var(--rl-space-4)">
        <div style="display:flex; gap:var(--rl-space-4); align-items:center; flex-wrap:wrap">
          <label style="display:flex; gap:var(--rl-space-2); align-items:center; font-size:var(--rl-font-size-sm)">
            Table width
            <input type="range" min="280" max="1100" step="10" v-model.number="width" />
            <strong>{{ width }}px</strong>
            <span style="color:var(--rl-color-text-subtle)">
              ({{ width <= 767 ? 'narrow: compact applies' : 'wide: full columns' }})
            </span>
          </label>
          <RlCheckbox label="Selectable" :model-value="selectable" @update:model-value="selectable = $event" />
        </div>

        <div style="display:flex; gap:var(--rl-space-4); align-items:center; flex-wrap:wrap; font-size:var(--rl-font-size-sm)">
          <span style="color:var(--rl-color-text-subtle)">Primary columns (kept by compress):</span>
          <RlCheckbox
            v-for="column in columns"
            :key="column.key"
            :label="column.header"
            :model-value="primary[column.key] === true"
            @update:model-value="primary = { ...primary, [column.key]: $event }"
          />
        </div>

        <div :style="{ width: width + 'px', border: '1px solid var(--rl-color-border)' }">
          <RlTable
            v-bind="args"
            :columns="columns"
            :selected-ids="selectable ? selectedIds : undefined"
            @toggle="onToggle"
            @toggle-all="onToggleAll"
          >
            <!--
              A column's field renders a string or a number and nothing else,
              so a column holding tags or a status needs its own cell slot.
            -->
            <template #cell-label="{ item }">
              <RlTagList v-if="item.tags?.length" :tags="item.tags" />
            </template>
            <template #cell-priority="{ item }">
              <RlStatusPill v-if="item.priority" :label="item.priority.label" :color="item.priority.color" />
            </template>
            <template #bulk="{ count }">
              <strong>{{ count }} selected</strong>
              <RlButton size="sm" variant="secondary">Assign</RlButton>
              <RlButton size="sm" variant="secondary" tone="danger">Delete</RlButton>
            </template>
          </RlTable>
        </div>
      </div>
    `,
  }),
}

/**
 * A list that refuses creation hides both add controls: the one in each
 * section header and the one at the foot of each list are the same
 * affordance in two places, so gating one and leaving the other would just
 * move the dead control.
 *
 * For a permission the user does not hold, or a list with no form to create
 * with. A button that emits an event nothing can act on is worse than no
 * button, because it reads as an offer.
 */
export const NotAddable: Story = {
  args: {
    sections: tableSections,
    columns: tableColumns,
    showAdd: false,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    /*
     * Asserted by name rather than by counting buttons: the table is full of
     * controls, and a count would pass for the wrong reason the moment any
     * other button moved in or out of the section.
     */
    await expect(canvas.queryByRole('button', { name: /^Add$/ })).toBeNull()
    await expect(canvas.queryByRole('button', { name: /^Add to / })).toBeNull()

    /* The rows themselves are untouched: this hides a control, not content. */
    await expect(canvas.getAllByRole('row').length).toBeGreaterThan(1)
  },
}

/** The same table with creation allowed, so the absence above means something. */
export const Addable: Story = {
  args: {
    sections: tableSections,
    columns: tableColumns,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    await expect(canvas.getAllByRole('button', { name: /^Add$/ }).length).toBeGreaterThan(0)
    await expect(canvas.getAllByRole('button', { name: /^Add to / }).length).toBeGreaterThan(0)
  },
}

/**
 * A section with no rows says so. Without `empty-label` it draws its header
 * and then nothing, which reads as a list that failed to load rather than
 * one that is genuinely empty.
 *
 * Sections with rows ignore it, so it is set once rather than conditionally.
 */
export const EmptySection: Story = {
  args: {
    sections: [
      { id: 'todo', title: 'To do', color: 'grey', items: [] },
      tableSections[0],
    ],
    columns: tableColumns,
    emptyLabel: 'Nothing here yet',
    emptyDescription: 'Tasks moved to this column will show up here.',
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    await expect(await canvas.findByText('Nothing here yet')).toBeInTheDocument()

    /*
     * Once, not per section: the section that has rows must not also claim
     * to be empty, which a naive `v-if` on the table rather than the section
     * would produce.
     */
    await expect(canvas.getAllByText('Nothing here yet')).toHaveLength(1)
    await expect(canvas.getAllByRole('row').length).toBeGreaterThan(1)
  },
}

/**
 * A thousand rows in one section, to exercise what the row does when it is
 * scrolled out of view.
 *
 * Rows carry `content-visibility: auto`, so the browser skips layout and paint
 * for the ones off screen and pays that cost on scroll instead. This is not
 * windowing: every row stays in the DOM, so find-in-page, Select All and a
 * screen reader's row count all keep working, which is what a virtual list
 * gives up.
 *
 * The test below is the part worth keeping. `content-visibility` without a
 * matching `contain-intrinsic-size` collapses every skipped row to zero height,
 * which makes the scrollbar wrong and the list jump as it renders — and it
 * still looks correct in a screenshot of the top of the list.
 */
export const LongList: Story = {
  args: {
    sections: [
      {
        id: 'all',
        title: 'Every task',
        color: 'grey',
        items: Array.from({ length: 1000 }, (_, i) => ({
          id: `t${i}`,
          title: `Task ${i + 1}`,
          status: i % 3 === 0 ? 'In progress' : 'To do',
        })),
      },
    ] as Section<Task>[],
    columns: tableColumns,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)

    /* Every row is real, not a window over the data. */
    const rows = canvas.getAllByRole('row')
    await expect(rows.length).toBeGreaterThan(1000)

    const first = canvasElement.querySelector('.rl-table-row') as HTMLElement
    await expect(getComputedStyle(first).contentVisibility).toBe('auto')

    /*
     * The row an eye is on has a real height, and so does one far down the
     * list: a skipped row keeps the intrinsic size it was given rather than
     * collapsing. Checked on a late row because the early ones are on screen
     * and would pass either way.
     */
    const all = [...canvasElement.querySelectorAll('.rl-table-row')] as HTMLElement[]
    await expect(all[0].getBoundingClientRect().height).toBeGreaterThan(0)
    await expect(all[all.length - 1].getBoundingClientRect().height).toBeGreaterThan(0)

    /* And the scroll extent reflects a thousand rows rather than a handful. */
    const scroller = canvasElement.querySelector('.rl-table') as HTMLElement
    await expect(scroller.scrollHeight).toBeGreaterThan(1000 * 20)
  },
}

/**
 * Rows in an order the reader sets by hand. Each row gets a handle: drag it,
 * or focus it and press the up and down arrow keys.
 *
 * The table reports the move as a `RowMove` and changes nothing itself; this
 * story applies it to its own copy of the rows, which is what a consuming app
 * does. A drag names the row it landed against, and a key press names a step.
 */
export const Reorderable: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement)
    const handle = canvas.getAllByRole('button', { name: /^Move / })[0]
    const first = handle.getAttribute('aria-label')

    handle.focus()
    await userEvent.keyboard('{ArrowDown}')
    await expect(canvas.getAllByRole('button', { name: /^Move / })[1]).toHaveAttribute('aria-label', first)
  },
  render: () => ({
    components: { RlTable: storyComponent(RlTable) },
    setup() {
      const items = ref<Task[]>(structuredClone(tableSections[0].items))
      const sections = computed<Section<Task>[]>(() => [{ id: 'all', title: '', items: items.value }])

      function onReorder(move: RowMove) {
        const list = items.value.slice()
        const from = list.findIndex((item) => item.id === move.itemId)
        if (from < 0) return
        const [moved] = list.splice(from, 1)
        let to: number
        if ('step' in move) {
          to = Math.min(Math.max(from + move.step, 0), list.length)
        } else {
          to = list.findIndex((item) => item.id === move.targetId)
          if (to < 0) return
          if (move.placement === 'after') to += 1
        }
        list.splice(to, 0, moved)
        items.value = list
      }

      return { sections, columns: tableColumns, onReorder, reorderable: () => true }
    },
    template: `
      <RlTable
        :sections="sections"
        :columns="columns"
        :show-section-header="false"
        :reorderable="reorderable"
        @reorder="onReorder"
      />
    `,
  }),
}
