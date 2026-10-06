/**
 * Mockups: editing rela's screens (what `data-entry.yaml` holds) in the app.
 *
 * Each editor sits beside a live preview built from the same library
 * components the real screen uses, so an operator sees the result rather than
 * a description of it. The preview follows the draft: reorder a field and the
 * form preview reorders with it.
 *
 * These are mockups: the data is a fixed sample and nothing is stored.
 */
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { computed, ref } from 'vue'

import RlConfigShell from '../fixtures/RlConfigShell.vue'
import RlConfigPreview from '../fixtures/RlConfigPreview.vue'
import RlSortableList from '../components/data/RlSortableList.vue'
import RlSegmentedControl from '../components/data/RlSegmentedControl.vue'
import RlTable from '../components/table/RlTable.vue'
import RlBoard from '../components/board/RlBoard.vue'
import RlTaskCard from '../components/board/RlTaskCard.vue'
import RlSidebar from '../components/layout/RlSidebar.vue'
import RlSectionHeading from '../components/layout/RlSectionHeading.vue'
import RlButton from '../components/common/RlButton.vue'
import RlButtonGroup from '../components/common/RlButtonGroup.vue'
import RlIconButton from '../components/common/RlIconButton.vue'
import RlTag from '../components/common/RlTag.vue'
import RlText from '../components/common/RlText.vue'
import RlHeading from '../components/common/RlHeading.vue'
import RlTextField from '../components/form/RlTextField.vue'
import RlTextarea from '../components/form/RlTextarea.vue'
import RlNumberField from '../components/form/RlNumberField.vue'
import RlSelect from '../components/form/RlSelect.vue'
import RlMultiSelect from '../components/form/RlMultiSelect.vue'
import RlCheckbox from '../components/form/RlCheckbox.vue'
import RlSwitch from '../components/form/RlSwitch.vue'
import RlDrawer from '../components/overlay/RlDrawer.vue'
import RlMenu from '../components/overlay/RlMenu.vue'
import RlMenuItem from '../components/overlay/RlMenuItem.vue'
import { storyComponent } from '../components/storyGeneric'

import {
  allTicketsColumns,
  boardColumns,
  dashboardCards,
  deliveryNavigation,
  editTicketFields,
  editTicketRelations,
  entityTypes,
  forms,
  sampleTickets,
  type FormFieldRow,
  type NavGroupDraft,
} from '../fixtures/config'
import type { NavGroup, Section } from '../types'
import type { TableColumn } from '../components/table/types'

const meta: Meta = {
  title: 'Mockups/Configure screens',
  parameters: { layout: 'fullscreen' },
}
export default meta
type Story = StoryObj

const RlSortableListAny = storyComponent(RlSortableList)
const RlTableAny = storyComponent(RlTable)
const RlBoardAny = storyComponent(RlBoard)

function reorder<T>(items: T[], item: T, toIndex: number): T[] {
  const rest = items.filter((entry) => entry !== item)
  rest.splice(toIndex, 0, item)
  return rest
}

/** Editor on the left, preview on the right. */
const editorAndPreview = 'display:grid; grid-template-columns:minmax(360px, 1fr) minmax(0, 1.2fr); gap:40px; align-items:start'
const stack = 'display:flex; flex-direction:column; gap:16px'

const kindIcons: Record<string, string> = { Dashboard: 'dashboard', List: 'list', Board: 'kanban', Entities: 'boxes', Calendar: 'calendar' }

/**
 * A space's sidebar. Each group is its own sortable list, and an entry moves
 * between groups through its menu: one level of drag, which covers the common
 * case, and a menu for the rare one. The sidebar on the right is the real
 * component, drawn from the draft.
 */
export const Navigation: Story = {
  render: () => ({
    components: {
      RlConfigShell, RlConfigPreview, RlSortableList: RlSortableListAny, RlSidebar, RlSectionHeading, RlButton, RlIconButton, RlTag,
      RlText, RlHeading, RlTextField, RlMultiSelect, RlMenu, RlMenuItem,
    },
    setup() {
      const groups = ref<NavGroupDraft[]>(deliveryNavigation.map((group) => ({ ...group, entries: [...group.entries] })))
      const createTypes = ref(['Ticket', 'Bug'])

      function moveWithin(group: NavGroupDraft, item: NavGroupDraft['entries'][number], to: number) {
        group.entries = reorder(group.entries, item, to)
      }
      function moveTo(entry: NavGroupDraft['entries'][number], from: NavGroupDraft, to: NavGroupDraft) {
        from.entries = from.entries.filter((candidate) => candidate !== entry)
        to.entries = [...to.entries, entry]
      }

      const preview = computed<NavGroup[]>(() =>
        groups.value.map((group) => ({
          id: group.id,
          label: group.id === 'top' ? undefined : group.title,
          items: group.entries.map((entry) => ({ id: entry.id, label: entry.title, icon: kindIcons[entry.kind] })),
        })),
      )
      return {
        groups, createTypes, moveWithin, moveTo, preview, kindIcons, stack, editorAndPreview,
        typeOptions: entityTypes.map((type) => ({ value: type.title, label: type.title })),
      }
    },
    template: `
      <RlConfigShell active-id="navigation" title="Navigation: Delivery">
        <div style="display:grid; grid-template-columns:minmax(360px, 680px) 262px; gap:40px; align-items:start">
          <div style="display:flex; flex-direction:column; gap:32px">
            <section :style="stack">
              <RlHeading :level="2" size="md">Space</RlHeading>
              <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px">
                <RlTextField model-value="Delivery" label="Name" required />
                <RlTextField model-value="rocket" label="Icon" />
              </div>
              <RlMultiSelect v-model="createTypes" label="The New button creates" :options="typeOptions" />
            </section>

            <section v-for="group in groups" :key="group.id" :style="stack">
              <div style="display:flex; align-items:center; gap:8px">
                <RlHeading :level="2" size="md" style="flex:1">{{ group.title }}</RlHeading>
                <RlIconButton v-if="group.id !== 'top'" icon="edit" :label="'Rename ' + group.title" />
              </div>
              <RlSortableList :items="group.entries" :label="'Entries in ' + group.title" @move="(item, to) => moveWithin(group, item, to)">
                <template #item="{ item }">
                  <div style="display:flex; align-items:center; gap:8px">
                    <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                    <RlText size="sm" tone="subtle">{{ item.kind }}: {{ item.target }}</RlText>
                    <span style="margin-left:auto; display:flex; align-items:center; gap:6px">
                      <RlTag v-if="item.badges" :label="'Counts ' + item.badges" />
                      <RlMenu>
                        <template #trigger="{ toggle, attrs }">
                          <RlIconButton icon="ellipsis" :label="'Options for ' + item.title" v-bind="attrs" @click="toggle" />
                        </template>
                        <RlMenuItem icon="edit">Edit entry</RlMenuItem>
                        <RlMenuItem
                          v-for="target in groups.filter((other) => other !== group)"
                          :key="target.id"
                          icon="arrow-right"
                          @click="moveTo(item, group, target)"
                        >Move to {{ target.title }}</RlMenuItem>
                        <RlMenuItem icon="trash-2" tone="danger">Remove</RlMenuItem>
                      </RlMenu>
                    </span>
                  </div>
                </template>
              </RlSortableList>
              <div><RlButton variant="ghost" size="sm" icon="plus">Add entry</RlButton></div>
            </section>
            <div><RlButton variant="secondary" icon="plus">Add group</RlButton></div>
          </div>

          <RlConfigPreview subject="Sidebar" flush sticky style="height:640px">
            <RlSidebar workspace-name="Delivery" :groups="preview" active-id="active" :theme-toggle="false" />
          </RlConfigPreview>
        </div>
      </RlConfigShell>
    `,
  }),
}

/** All forms, with where each is used, so an operator knows what an edit reaches. */
export const Forms: Story = {
  render: () => ({
    components: { RlConfigShell, RlConfigPreview, RlTable: RlTableAny, RlText, RlButton },
    setup() {
      const sections: Section<(typeof forms)[number]>[] = [{ id: 'all', title: '', items: forms }]
      const columns: TableColumn[] = [
        { key: 'entityType', header: 'Creates or edits', field: 'entityType', width: 160 },
        { key: 'usedIn', header: 'Used in', field: 'usedIn', width: 340 },
      ]
      return { sections, columns }
    },
    template: `
      <RlConfigShell active-id="forms" title="Forms" :draft-count="0">
        <template #actions><RlButton variant="primary" icon="plus">New form</RlButton></template>
        <div style="max-width:900px">
          <RlTable :sections="sections" :columns="columns" name-label="Form" :show-section-header="false" :show-add="false" />
        </div>
      </RlConfigShell>
    `,
  }),
}

const formEditorSetup = (openStatus: boolean) => () => {
  const fields = ref<FormFieldRow[]>(editTicketFields.map((field) => ({ ...field })))
  fields.value.splice(3, 0, { id: 'due', title: 'Due', type: 'Date', width: 'Half' })
  const relationRows = ref(editTicketRelations.map((relation) => ({ ...relation })))
  const editing = ref<FormFieldRow | undefined>(openStatus ? fields.value.find((field) => field.id === 'status') : undefined)

  const moveField = (item: FormFieldRow, to: number) => (fields.value = reorder(fields.value, item, to))
  const moveRelation = (item: (typeof relationRows.value)[number], to: number) =>
    (relationRows.value = reorder(relationRows.value, item, to))

  const spans: Record<FormFieldRow['width'], string> = { Full: 'span 6', Half: 'span 3', Third: 'span 2' }
  const states = ['Backlog', 'Ready', 'Planning', 'In progress', 'Review', 'Done', "Won't fix", 'On hold']
  const transitions = ref<Record<string, string[]>>({
    Backlog: ['Ready', "Won't fix"],
    Ready: ['Backlog', 'Planning', "Won't fix"],
    Planning: ['Ready', 'In progress', 'On hold'],
    'In progress': ['Planning', 'Review', 'On hold'],
    Review: ['In progress', 'Done', 'On hold'],
    Done: ['Review'],
    "Won't fix": ['Backlog'],
    'On hold': ['Planning', 'In progress', 'Review', "Won't fix"],
  })
  return {
    fields, relationRows, editing, moveField, moveRelation, spans, states, transitions,
    widthOptions: [
      { value: 'Third', label: 'Third' },
      { value: 'Half', label: 'Half' },
      { value: 'Full', label: 'Full' },
    ],
    stack, editorAndPreview,
  }
}

const formEditorTemplate = `
  <RlConfigShell active-id="forms" title="Edit ticket" back-label="Forms" back-to="mockups-configure-screens--forms">
    <div :style="editorAndPreview">
      <div style="display:flex; flex-direction:column; gap:32px">
        <section :style="stack">
          <RlSectionHeading title="Fields" :count="fields.length" :level="2" />
          <RlSortableList :items="fields" label="Fields of Edit ticket" @move="moveField">
            <template #item="{ item }">
              <button type="button" style="all:unset; box-sizing:border-box; display:flex; align-items:center; gap:8px; width:100%; cursor:pointer" @click="editing = item">
                <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                <RlText size="sm" tone="subtle">{{ item.type }}</RlText>
                <span style="margin-left:auto; display:flex; gap:6px">
                  <RlTag v-if="item.id === 'due'" label="New" color="green" />
                  <RlTag v-if="item.id === 'status'" label="8 transitions" color="purple" />
                  <RlTag :label="item.width" />
                </span>
              </button>
            </template>
          </RlSortableList>
          <div><RlButton variant="secondary" size="sm" icon="plus">Add field</RlButton></div>
        </section>

        <section :style="stack">
          <RlSectionHeading title="Linked records" :count="relationRows.length" :level="2" />
          <RlSortableList :items="relationRows" label="Linked records on Edit ticket" @move="moveRelation">
            <template #item="{ item }">
              <div style="display:flex; align-items:center; gap:8px">
                <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                <RlText size="sm" tone="subtle">{{ item.relation }}</RlText>
                <span style="margin-left:auto; display:flex; gap:6px">
                  <RlTag v-if="item.createInline" label="Can create in place" color="blue" />
                  <RlTag :label="item.picker === 'Multiple' ? 'Several' : 'One'" />
                </span>
              </div>
            </template>
          </RlSortableList>
        </section>

        <section :style="stack">
          <RlSectionHeading title="Form" :level="2" />
          <RlTextField model-value="Edit ticket" label="Title" required />
          <RlCheckbox :model-value="true" label="Include the description editor" />
        </section>
      </div>

      <RlConfigPreview subject="Form" sticky>
        <RlHeading :level="2" size="lg" style="margin-bottom:16px">Edit ticket</RlHeading>
        <div style="display:grid; grid-template-columns:repeat(6, 1fr); gap:16px">
          <div v-for="field in fields" :key="field.id" :style="{ gridColumn: spans[field.width] }">
            <RlTextField v-if="field.type === 'Text'" :label="field.title" :placeholder="field.placeholder" />
            <RlTextField v-else-if="field.type === 'Date'" :label="field.title" placeholder="dd-mm-yyyy" />
            <RlMultiSelect v-else-if="field.id === 'tags'" :label="field.title" :options="[]" placeholder="Add tags" />
            <RlSelect v-else :label="field.title" :options="[]" placeholder="Choose…" />
          </div>
          <div v-for="relation in relationRows" :key="relation.id" style="grid-column:span 6">
            <RlSelect :label="relation.title" :options="[]" :placeholder="relation.picker === 'Multiple' ? 'Link records…' : 'Link a record…'" />
          </div>
        </div>
      </RlConfigPreview>
    </div>

    <RlDrawer :title="'Field: ' + (editing?.title ?? '')" size="md" :open="!!editing" @close="editing = undefined">
      <div v-if="editing" :style="stack">
        <RlTextField :model-value="editing.title" label="Label" hint="Leave as is to use the property's own label." />
        <RlTextField :model-value="editing.help" label="Help text" />
        <RlTextField :model-value="editing.placeholder" label="Placeholder" />
        <div>
          <RlText size="sm" weight="medium" as="div" style="margin-bottom:6px">Width</RlText>
          <RlSegmentedControl v-model="editing.width" label="Width" :options="widthOptions" />
        </div>
        <RlCheckbox :model-value="!!editing.hidden" label="Hidden" hint="The value is kept and set by defaults or automations." />

        <template v-if="editing.id === 'status'">
          <RlSectionHeading title="Allowed changes" :level="3" size="sm" />
          <RlText size="sm" tone="muted" as="p" style="margin:0">From each status, which statuses someone may move a ticket to.</RlText>
          <RlMultiSelect
            v-for="state in states"
            :key="state"
            v-model="transitions[state]"
            :label="'From ' + state"
            :options="states.filter((s) => s !== state).map((s) => ({ value: s, label: s }))"
          />
        </template>
      </div>
      <template #actions>
        <RlButtonGroup>
          <RlButton variant="secondary" @click="editing = undefined">Cancel</RlButton>
          <RlButton variant="primary" @click="editing = undefined">Done</RlButton>
        </RlButtonGroup>
      </template>
    </RlDrawer>
  </RlConfigShell>
`

const formEditorComponents = {
  RlConfigShell, RlConfigPreview, RlSortableList: RlSortableListAny, RlSegmentedControl, RlSectionHeading, RlButton, RlButtonGroup, RlTag, RlText,
  RlHeading, RlTextField, RlSelect, RlMultiSelect, RlCheckbox, RlDrawer,
}

/**
 * A form. Fields and linked records are sortable lists; the preview on the
 * right is the form as it will render, and follows every reorder and width
 * change. The new Due field from the draft is already in place.
 */
export const FormEditor: Story = {
  render: () => ({ components: formEditorComponents, setup: formEditorSetup(false), template: formEditorTemplate }),
}

/**
 * One field opened. A status field also carries its allowed changes: one
 * multi-select per status is the stopgap for a transition grid.
 */
export const FormFieldWithTransitions: Story = {
  render: () => ({ components: formEditorComponents, setup: formEditorSetup(true), template: formEditorTemplate }),
}

/**
 * A list. Columns are a sortable list with a switch for sorting; the preview
 * is the real table over sample tickets, so a reorder is seen as a reorder.
 */
export const ListEditor: Story = {
  render: () => ({
    components: {
      RlConfigShell, RlConfigPreview, RlSortableList: RlSortableListAny, RlTable: RlTableAny, RlSectionHeading, RlButton, RlText, RlTag,
      RlSwitch, RlSelect, RlMultiSelect, RlNumberField, RlTextField, RlTextarea,
    },
    setup() {
      const columns = ref([...allTicketsColumns, { id: 'due', title: 'Due', sortable: true }])
      const move = (item: (typeof columns.value)[number], to: number) => (columns.value = reorder(columns.value, item, to))
      const filters = ref(['Status', 'Priority', 'Kind'])
      const pageSize = ref(25)
      const rows = sampleTickets.map((ticket, index) => ({ ...ticket, due: ['12 Oct', '3 Nov', '', '21 Oct'][index] }))
      const sections = computed(() => [{ id: 'all', title: '', items: rows }])
      const tableColumns = computed<TableColumn[]>(() =>
        columns.value
          .filter((column) => column.id !== 'title')
          .map((column) => ({ key: column.id, header: column.title, field: column.id, sortable: column.sortable, width: 120 })),
      )
      return {
        columns, move, filters, pageSize, sections, tableColumns, stack, editorAndPreview,
        propertyOptions: ['Status', 'Priority', 'Effort', 'Kind', 'Tags', 'Due'].map((p) => ({ value: p, label: p })),
      }
    },
    template: `
      <RlConfigShell active-id="lists" title="All tickets">
        <div style="display:flex; flex-direction:column; gap:32px">
          <div style="display:grid; grid-template-columns:minmax(360px, 1fr) minmax(360px, 1fr); gap:40px; align-items:start">
            <section :style="stack">
              <RlSectionHeading title="Columns" :count="columns.length" :level="2" />
              <RlSortableList :items="columns" label="Columns of All tickets" @move="move">
                <template #item="{ item }">
                  <div style="display:flex; align-items:center; gap:8px">
                    <RlText size="sm" weight="medium" style="flex:1">{{ item.title }}</RlText>
                    <RlTag v-if="item.id === 'due'" label="New" color="green" />
                    <RlTag v-if="item.opensDetail" label="Opens the ticket" color="blue" />
                    <RlSwitch v-model="item.sortable" label="Sortable" size="sm" />
                  </div>
                </template>
              </RlSortableList>
              <div><RlButton variant="secondary" size="sm" icon="plus">Add column</RlButton></div>
            </section>
            <section :style="stack">
              <RlSectionHeading title="List" :level="2" />
              <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px">
                <RlTextField model-value="All tickets" label="Title" required />
                <RlSelect model-value="Ticket" label="Shows" :options="[{ value: 'Ticket', label: 'Tickets' }]" />
                <RlSelect model-value="Status" label="Sorted by" :options="propertyOptions" />
                <RlNumberField v-model="pageSize" label="Rows per page" :min="5" :max="200" />
              </div>
              <RlMultiSelect v-model="filters" label="Filters people can use" :options="propertyOptions" />
              <RlTextarea model-value="Every ticket. Use the filters to narrow it down." label="Text above the list" :rows="2" />
            </section>
          </div>
          <RlConfigPreview subject="List, with sample tickets">
            <RlTable :sections="sections" :columns="tableColumns" name-label="Title" :show-section-header="false" :show-add="false" />
          </RlConfigPreview>
        </div>
      </RlConfigShell>
    `,
  }),
}

/**
 * A board. The columns are a property's options, so they are reordered here
 * but added on the choice list; a switch hides an option that should not get
 * a column. The preview is the real board over sample tickets.
 */
export const BoardEditor: Story = {
  render: () => ({
    components: {
      RlConfigShell, RlConfigPreview, RlSortableList: RlSortableListAny, RlBoard: RlBoardAny, RlTaskCard, RlSectionHeading, RlButton, RlText, RlTag,
      RlSwitch, RlSelect, RlMultiSelect, RlTextField,
    },
    setup() {
      const columns = ref(boardColumns.map((column) => ({ ...column, shown: true })))
      const move = (item: (typeof columns.value)[number], to: number) => (columns.value = reorder(columns.value, item, to))
      const cardFields = ref(['Priority', 'Effort'])
      const sections = computed(() =>
        columns.value
          .filter((column) => column.shown)
          .map((column) => ({
            id: column.id,
            title: column.title,
            items: sampleTickets.filter((ticket) => ticket.status === column.title),
          })),
      )
      return {
        columns, move, cardFields, sections, stack,
        propertyOptions: ['Priority', 'Effort', 'Kind', 'Tags', 'Due'].map((p) => ({ value: p, label: p })),
      }
    },
    template: `
      <RlConfigShell active-id="boards" title="Ticket board">
        <div style="display:flex; flex-direction:column; gap:32px">
          <div style="display:grid; grid-template-columns:minmax(360px, 1fr) minmax(360px, 1fr); gap:40px; align-items:start">
            <section :style="stack">
              <RlSectionHeading title="Columns" :count="columns.filter(c => c.shown).length" :level="2" />
              <RlText size="sm" tone="muted" as="p" style="margin:0">One column per Status option. Add options on the Ticket status choice list.</RlText>
              <RlSortableList :items="columns" label="Columns of Ticket board" @move="move">
                <template #item="{ item }">
                  <div style="display:flex; align-items:center; gap:8px">
                    <RlTag :label="item.title" :color="item.color" />
                    <span style="flex:1" />
                    <RlSwitch v-model="item.shown" :label="'Show ' + item.title" label-hidden size="sm" />
                  </div>
                </template>
              </RlSortableList>
            </section>
            <section :style="stack">
              <RlSectionHeading title="Board" :level="2" />
              <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px">
                <RlTextField model-value="Ticket board" label="Title" required />
                <RlSelect model-value="status" label="Columns from" :options="[{ value: 'status', label: 'Status' }, { value: 'priority', label: 'Priority' }]" />
              </div>
              <RlMultiSelect v-model="cardFields" label="Shown on each card" :options="propertyOptions" />
            </section>
          </div>
          <RlConfigPreview subject="Board, with sample tickets" style="height:400px">
            <RlBoard :sections="sections" :show-add="false" style="flex:1; min-height:0">
              <template #card="{ item }">
                <RlTaskCard :task="{ id: item.id, title: item.title, tags: cardFields.map((field) => ({ id: field, label: item[field.toLowerCase()] ?? '—' })) }" />
              </template>
            </RlBoard>
          </RlConfigPreview>
        </div>
      </RlConfigShell>
    `,
  }),
}

/**
 * The dashboard: an ordered list of cards. Queries are shown as sentences;
 * the card drawer (not shown) is where they are built.
 */
export const Dashboard: Story = {
  render: () => ({
    components: { RlConfigShell, RlConfigPreview, RlSortableList: RlSortableListAny, RlSectionHeading, RlButton, RlText, RlTag, RlTextField },
    setup() {
      const cards = ref([...dashboardCards])
      const move = (item: (typeof cards.value)[number], to: number) => (cards.value = reorder(cards.value, item, to))
      return { cards, move, stack }
    },
    template: `
      <RlConfigShell active-id="dashboard" title="Dashboard" :draft-count="0">
        <div style="max-width:760px; display:flex; flex-direction:column; gap:24px">
          <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px">
            <RlTextField model-value="Rela Development" label="Title" />
            <RlTextField model-value="Overview of architecture, features, and ideas" label="Description" />
          </div>
          <section :style="stack">
            <RlSectionHeading title="Cards" :count="cards.length" :level="2" />
            <RlSortableList :items="cards" label="Dashboard cards" @move="move">
              <template #item="{ item }">
                <div style="display:flex; align-items:center; gap:8px">
                  <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                  <RlText size="sm" tone="subtle" truncate style="flex:1">{{ item.query }}</RlText>
                  <RlTag :label="item.shows" :color="item.shows === 'Count' ? 'blue' : item.shows === 'Table' ? 'purple' : 'green'" />
                </div>
              </template>
            </RlSortableList>
            <div><RlButton variant="secondary" size="sm" icon="plus">Add card</RlButton></div>
          </section>
        </div>
      </RlConfigShell>
    `,
  }),
}
