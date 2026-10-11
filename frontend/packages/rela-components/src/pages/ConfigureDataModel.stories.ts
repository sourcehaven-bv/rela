/**
 * Mockups: editing rela's data model (what `schema.yaml` holds) in the app.
 *
 * Every screen talks about the model in the reader's terms. An enum type is a
 * "choice list", a validation is a "rule", and the file the change ends up in
 * never appears. Edits collect in one draft across all Configure screens and
 * are written only after the review step; the shell's "Review and save"
 * button opens it.
 *
 * These are mockups: the data is a fixed sample and nothing is stored.
 */
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { computed, ref } from 'vue'

import RlConfigShell from '../fixtures/RlConfigShell.vue'
import RlTable from '../components/table/RlTable.vue'
import RlSortableList from '../components/data/RlSortableList.vue'
import RlSearchBox from '../components/data/RlSearchBox.vue'
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
import RlOptionSelect from '../components/form/RlOptionSelect.vue'
import RlCheckbox from '../components/form/RlCheckbox.vue'
import RlRadioGroup from '../components/form/RlRadioGroup.vue'
import RlDrawer from '../components/overlay/RlDrawer.vue'
import RlMenu from '../components/overlay/RlMenu.vue'
import RlMenuItem from '../components/overlay/RlMenuItem.vue'
import RlCallout from '../components/feedback/RlCallout.vue'
import RlViewTabs from '../components/layout/RlViewTabs.vue'
import RlChangeList from '../components/data/RlChangeList.vue'
import RlChangeItem from '../components/data/RlChangeItem.vue'
import { storyComponent } from '../components/storyGeneric'

import {
  automations,
  choiceLists,
  colorLabels,
  colorOptions,
  entityTypes,
  migrations,
  propertyTypeOptions,
  relations,
  rules,
  ticketProperties,
  ticketStatusValues,
  type ChoiceValueRow,
  type EntityTypeRow,
  type MigrationRow,
  type PropertyRow,
} from '../fixtures/config'
import type { Section } from '../types'
import type { TableColumn } from '../components/table/types'

const meta: Meta = {
  title: 'Mockups/Configure data model',
  parameters: { layout: 'fullscreen' },
}
export default meta
type Story = StoryObj

const RlTableAny = storyComponent(RlTable)
const RlSortableListAny = storyComponent(RlSortableList)

function reorder<T>(items: T[], item: T, toIndex: number): T[] {
  const rest = items.filter((entry) => entry !== item)
  rest.splice(toIndex, 0, item)
  return rest
}

const entityTypeOptions = entityTypes.map((type) => ({ value: type.title, label: type.title }))

/** A narrow column of settings beside a wider one, the layout every editor uses. */
const twoColumns = 'display:grid; grid-template-columns:minmax(0,1fr) 320px; gap:40px; align-items:start; max-width:1100px'
const stack = 'display:flex; flex-direction:column; gap:16px'

/**
 * Where an operator starts: every entity type, grouped by the area of the
 * work it belongs to. The counts say how big a type is before opening it.
 */
export const EntityTypes: Story = {
  render: () => ({
    components: { RlConfigShell, RlTable: RlTableAny, RlTag, RlText, RlSearchBox, RlButton },
    setup() {
      const query = ref('')
      const sections = computed<Section<EntityTypeRow>[]>(() => {
        const match = entityTypes.filter((type) => type.title.toLowerCase().includes(query.value.toLowerCase()))
        const areas = [...new Set(match.map((type) => type.area))]
        return areas.map((area) => ({ id: area, title: area, items: match.filter((type) => type.area === area) }))
      })
      const columns: TableColumn[] = [
        { key: 'id', header: 'IDs look like', width: 160 },
        { key: 'properties', header: 'Properties', width: 110, field: 'properties', align: 'end' },
        { key: 'relations', header: 'Relations', width: 110, field: 'relations', align: 'end' },
      ]
      return { query, sections, columns }
    },
    template: `
      <RlConfigShell active-id="entity-types" title="Entity types" :draft-count="0">
        <template #actions>
          <RlButton variant="primary" icon="plus">New entity type</RlButton>
        </template>
        <div style="max-width:900px; display:flex; flex-direction:column; gap:16px">
          <RlText tone="muted" size="sm" as="p" style="margin:0">
            An entity type is a kind of record, such as a ticket or a decision. It decides which properties a
            record has and how its ID is made.
          </RlText>
          <RlSearchBox v-model="query" placeholder="Find an entity type" size="sm" style="max-width:320px" />
          <RlTable :sections="sections" :columns="columns" name-label="Entity type" :show-add="false">
            <template #cell-id="{ item }">
              <RlText size="sm" tone="muted">{{ item.idPrefix ? item.idPrefix + 'X8K2PQ' : 'Chosen by hand' }}</RlText>
            </template>
          </RlTable>
        </div>
      </RlConfigShell>
    `,
  }),
}

const propertyEditorSetup = (openNew: boolean) => () => {
  const properties = ref<PropertyRow[]>(ticketProperties.map((property) => ({ ...property })))
  if (openNew) {
    properties.value.splice(5, 0, { id: 'due', title: 'Due', name: 'due', type: 'Date', description: 'When this must be done' })
  }
  const editing = ref<PropertyRow | undefined>(openNew ? properties.value[5] : undefined)
  const draft = ref<PropertyRow>({ id: '', title: '', name: '', type: 'Text' })

  function edit(property: PropertyRow) {
    editing.value = property
    draft.value = { ...property }
  }
  if (editing.value) draft.value = { ...editing.value }

  const move = (item: PropertyRow, to: number) => (properties.value = reorder(properties.value, item, to))
  const color = ref('blue')
  const idStyle = ref('short')

  return {
    properties, editing, draft, edit, move, color, idStyle, propertyTypeOptions, colorOptions, colorLabels,
    outgoing: relations.filter((relation) => relation.from.includes('Ticket')),
    incoming: relations.filter((relation) => relation.to.includes('Ticket')),
    idStyleOptions: [
      { value: 'short', label: 'Short code', description: 'A prefix and six random characters: TKT-8UCV32' },
      { value: 'manual', label: 'Chosen by hand', description: 'The author types the ID, such as a package name' },
    ],
  }
}

const entityTypeTemplate = `
  <RlConfigShell active-id="entity-types" title="Ticket" back-label="Entity types" back-to="mockups-configure-data-model--entity-types">
    <template #actions>
      <RlMenu>
        <template #trigger="{ toggle, attrs }">
          <RlIconButton icon="ellipsis" label="More options" v-bind="attrs" @click="toggle" />
        </template>
        <RlMenuItem icon="copy">Duplicate entity type</RlMenuItem>
        <RlMenuItem icon="trash-2" tone="danger" disabled>Delete (1,617 tickets exist)</RlMenuItem>
      </RlMenu>
    </template>

    <div :style="twoColumns">
      <div style="display:flex; flex-direction:column; gap:32px">
        <section :style="stack">
          <RlSectionHeading title="Properties" :count="properties.length" :level="2" />
          <RlText size="sm" tone="muted" as="p" style="margin:0">
            The order here is the order new forms and the detail page start from.
          </RlText>
          <RlSortableList :items="properties" label="Properties of Ticket" @move="move">
            <template #item="{ item }">
              <button
                type="button"
                style="all:unset; box-sizing:border-box; display:flex; align-items:center; gap:8px; width:100%; cursor:pointer"
                @click="edit(item)"
              >
                <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                <RlText size="sm" tone="subtle">{{ item.type }}{{ item.many ? ', several' : '' }}</RlText>
                <span style="margin-left:auto; display:flex; gap:6px">
                  <RlTag v-if="item.id === 'due'" label="New" color="green" />
                  <RlTag v-if="item.required" label="Required" color="blue" />
                  <RlTag v-if="item.setBy" :label="'Set by ' + item.setBy.toLowerCase()" />
                </span>
              </button>
            </template>
          </RlSortableList>
          <div><RlButton variant="secondary" icon="plus" size="sm">Add property</RlButton></div>
        </section>

        <section :style="stack">
          <RlSectionHeading title="Relations" :count="outgoing.length + incoming.length" :level="2" />
          <RlText size="sm" tone="muted" as="p" style="margin:0">
            Relations are shared between entity types and edited on their own page.
          </RlText>
          <div style="display:flex; flex-direction:column; gap:8px">
            <div v-for="relation in outgoing" :key="'out-' + relation.id" style="display:flex; align-items:center; gap:8px">
              <RlText size="sm">A ticket <strong>{{ relation.title }}</strong> {{ relation.to.join(' or ') }}</RlText>
              <RlTag v-if="relation.atLeast" :label="'at least ' + relation.atLeast" color="blue" />
            </div>
            <div v-for="relation in incoming" :key="'in-' + relation.id" style="display:flex; align-items:center; gap:8px">
              <RlText size="sm">A ticket <strong>{{ relation.inverse }}</strong> {{ relation.from.filter(t => t !== 'Ticket').join(' or ') || 'another ticket' }}</RlText>
            </div>
          </div>
        </section>
      </div>

      <aside :style="stack">
        <RlHeading :level="2" size="md">Settings</RlHeading>
        <RlTextField model-value="Ticket" label="Name" required />
        <RlRadioGroup v-model="idStyle" label="IDs" :options="idStyleOptions" />
        <RlTextField v-if="idStyle === 'short'" model-value="TKT-" label="ID prefix" hint="Existing IDs keep their prefix." />
        <div>
          <RlText size="sm" weight="medium" as="div" style="margin-bottom:6px">Colour</RlText>
          <RlOptionSelect v-model="color" :options="colorOptions" label="Colour">
            <template #option="{ value }"><RlTag :label="colorLabels[value]" :color="value" /></template>
          </RlOptionSelect>
        </div>
        <RlTextarea model-value="A unit of planned work: an enhancement, refactor, docs or chore." label="Description" :rows="3" />
      </aside>
    </div>

    <RlDrawer :title="editing && editing.id === 'due' ? 'New property' : 'Property: ' + (editing?.title ?? '')" size="md" :open="!!editing" @close="editing = undefined">
      <div :style="stack">
        <RlTextField v-model="draft.title" label="Label" required hint="What people see on forms and in lists." />
        <RlSelect v-model="draft.type" label="Type" :options="propertyTypeOptions" />
        <RlCheckbox v-model="draft.required" label="Required" hint="A ticket cannot be saved without it." />
        <RlCheckbox v-model="draft.many" label="Allow several values" />
        <RlTextarea v-model="draft.description" label="Help text" :rows="2" hint="Shown under the field on forms." />
        <RlCallout v-if="draft.required && editing && editing.id !== 'title'" tone="info" title="Existing tickets">
          1,617 tickets exist. Ones without a value stay valid until someone edits them.
        </RlCallout>
      </div>
      <template #actions>
        <RlButtonGroup>
          <RlButton variant="secondary" @click="editing = undefined">Cancel</RlButton>
          <RlButton variant="primary" @click="editing = undefined">Add to draft</RlButton>
        </RlButtonGroup>
      </template>
    </RlDrawer>
  </RlConfigShell>
`

const entityTypeComponents = {
  RlConfigShell, RlSortableList: RlSortableListAny, RlSectionHeading, RlButton, RlButtonGroup, RlIconButton, RlTag, RlText,
  RlHeading, RlTextField, RlTextarea, RlSelect, RlOptionSelect, RlCheckbox, RlRadioGroup, RlDrawer, RlMenu, RlMenuItem, RlCallout,
}

/**
 * One entity type. Properties are a sortable list, because their order is
 * where every form and detail page starts. Clicking a property opens it in a
 * drawer; the settings that describe the type itself sit beside the list.
 */
export const EntityType: Story = {
  render: () => ({
    components: entityTypeComponents,
    setup: () => ({ ...propertyEditorSetup(false)(), twoColumns, stack }),
    template: entityTypeTemplate,
  }),
}

/** Adding a property: the drawer, with the new property already placed in the list. */
export const AddProperty: Story = {
  render: () => ({
    components: entityTypeComponents,
    setup: () => ({ ...propertyEditorSetup(true)(), twoColumns, stack }),
    template: entityTypeTemplate,
  }),
}

/**
 * The model at a glance, one tab per kind of thing. Choice lists, relations,
 * rules and automations all have their own sidebar entry too; this is the
 * same content for someone browsing rather than editing.
 */
export const Overview: Story = {
  render: () => ({
    components: { RlConfigShell, RlViewTabs, RlTable: RlTableAny, RlTag, RlText },
    setup() {
      const tab = ref('choice-lists')
      const tabs = [
        { id: 'choice-lists', label: 'Choice lists', icon: 'list' },
        { id: 'relations', label: 'Relations', icon: 'network' },
        { id: 'rules', label: 'Rules', icon: 'shield-check' },
        { id: 'automations', label: 'Automations', icon: 'zap' },
      ]
      const flat = <T extends { id: string; title: string }>(items: T[]): Section<T>[] => [{ id: 'all', title: '', items }]
      return {
        tab, tabs,
        choiceSections: flat(choiceLists),
        relationSections: flat(relations),
        ruleSections: flat(rules),
        automationSections: flat(automations),
        choiceColumns: [
          { key: 'values', header: 'Options', field: 'values', width: 100, align: 'end' },
          { key: 'usedBy', header: 'Used by', width: 260 },
        ],
        relationColumns: [
          { key: 'reads', header: 'Reads as', width: 380 },
          { key: 'atLeast', header: 'Required', width: 120 },
        ],
        ruleColumns: [
          { key: 'appliesTo', header: 'Applies to', field: 'appliesTo', width: 140 },
          { key: 'severity', header: 'When broken', width: 160 },
        ],
        automationColumns: [
          { key: 'trigger', header: 'When', field: 'trigger', width: 300 },
          { key: 'actions', header: 'Then', field: 'actions', width: 380 },
        ],
      }
    },
    template: `
      <RlConfigShell active-id="choice-lists" title="Data model" :draft-count="0">
        <template #tabs><RlViewTabs v-model="tab" :tabs="tabs" /></template>
        <div style="max-width:1000px">
          <RlTable v-if="tab === 'choice-lists'" :sections="choiceSections" :columns="choiceColumns" name-label="Choice list" :show-section-header="false" :show-add="false">
            <template #cell-usedBy="{ item }"><RlText size="sm" tone="muted">{{ item.usedBy.join(', ') }}</RlText></template>
          </RlTable>
          <RlTable v-else-if="tab === 'relations'" :sections="relationSections" :columns="relationColumns" name-label="Relation" :show-section-header="false" :show-add="false">
            <template #cell-reads="{ item }"><RlText size="sm" tone="muted">{{ item.from.join(', ') }} → {{ item.to.join(', ') }}</RlText></template>
            <template #cell-atLeast="{ item }"><RlTag v-if="item.atLeast" :label="'at least ' + item.atLeast" color="blue" /></template>
          </RlTable>
          <RlTable v-else-if="tab === 'rules'" :sections="ruleSections" :columns="ruleColumns" name-label="Rule" :show-section-header="false" :show-add="false">
            <template #cell-severity="{ item }"><RlTag :label="item.severity === 'Error' ? 'Blocks saving' : 'Shows a warning'" :color="item.severity === 'Error' ? 'red' : 'amber'" /></template>
          </RlTable>
          <RlTable v-else :sections="automationSections" :columns="automationColumns" name-label="Automation" :show-section-header="false" :show-add="false" />
        </div>
      </RlConfigShell>
    `,
  }),
}

/**
 * A choice list. Each option has a colour and a count of the records using
 * it, so removing a busy option is visibly a bigger act than removing an
 * unused one. A removal stays in the list, struck through, until the draft is
 * saved; the review step asks where its records go.
 */
export const ChoiceList: Story = {
  render: () => ({
    components: {
      RlConfigShell, RlSortableList: RlSortableListAny, RlSectionHeading, RlOptionSelect, RlTag, RlText, RlButton,
      RlIconButton, RlTextField, RlHeading,
    },
    setup() {
      const values = ref<(ChoiceValueRow & { removed?: boolean; added?: boolean })[]>([
        ...ticketStatusValues.map((value) => ({ ...value, removed: value.id === 'blocked' })),
        { id: 'on-hold', title: 'On hold', color: 'amber', inUse: 0, added: true },
      ])
      const move = (item: ChoiceValueRow, to: number) => (values.value = reorder(values.value, item, to))
      const setDefault = (id: string) => values.value.forEach((value) => (value.isDefault = value.id === id))
      return { values, move, setDefault, colorOptions, colorLabels, twoColumns, stack }
    },
    template: `
      <RlConfigShell active-id="choice-lists" title="Ticket status" back-label="Choice lists" back-to="mockups-configure-data-model--overview">
        <div :style="twoColumns">
          <section :style="stack">
            <RlSectionHeading title="Options" :count="values.length" :level="2" />
            <RlText size="sm" tone="muted" as="p" style="margin:0">
              The order here is the order of a status menu and of board columns.
            </RlText>
            <RlSortableList :items="values" label="Options of Ticket status" @move="move">
              <template #item="{ item }">
                <div style="display:flex; align-items:center; gap:12px">
                  <div style="width:84px; flex:none">
                    <RlOptionSelect
                      variant="inline"
                      :model-value="item.color"
                      :options="colorOptions"
                      :label="'Colour of ' + item.title"
                      @update:model-value="item.color = $event"
                    >
                      <template #option="{ value }"><RlTag :label="colorLabels[value]" :color="value" /></template>
                    </RlOptionSelect>
                  </div>
                  <RlText size="sm" weight="medium" :style="item.removed ? 'text-decoration:line-through; color:var(--rl-color-text-subtle)' : ''" style="flex:1">
                    {{ item.title }}
                  </RlText>
                  <RlTag v-if="item.added" label="New" color="green" />
                  <RlTag v-if="item.isDefault" label="Default" color="blue" />
                  <RlText size="sm" tone="subtle" style="width:110px; text-align:right">
                    {{ item.inUse ? item.inUse.toLocaleString('en') + ' tickets' : 'Unused' }}
                  </RlText>
                  <div style="width:56px; flex:none; display:flex; justify-content:flex-end">
                    <RlButton v-if="item.removed" size="sm" variant="ghost" @click="item.removed = false">Undo</RlButton>
                    <RlIconButton v-else icon="trash-2" :label="'Remove ' + item.title" @click="item.removed = true" />
                  </div>
                </div>
              </template>
            </RlSortableList>
            <div><RlButton variant="secondary" icon="plus" size="sm">Add option</RlButton></div>
          </section>

          <aside :style="stack">
            <RlHeading :level="2" size="md">Settings</RlHeading>
            <RlTextField model-value="Ticket status" label="Name" required />
            <div>
              <RlText size="sm" weight="medium" as="div">Used by</RlText>
              <RlText size="sm" tone="muted" as="p" style="margin:4px 0 0">
                Ticket › Status, the Ticket board's columns, and the Active tickets list's filter.
              </RlText>
            </div>
          </aside>
        </div>
      </RlConfigShell>
    `,
  }),
}

/**
 * A relation, written as the sentence it produces in both directions. The
 * sentence is what an operator checks; the fields are how they change it.
 */
export const Relation: Story = {
  render: () => ({
    components: { RlConfigShell, RlTextField, RlTextarea, RlMultiSelect, RlNumberField, RlCheckbox, RlCallout, RlText, RlHeading },
    setup() {
      const name = ref('implements')
      const inverse = ref('implemented by')
      const from = ref(['Ticket'])
      const to = ref(['Feature'])
      const required = ref(true)
      const atLeast = ref(1)
      return { name, inverse, from, to, required, atLeast, entityTypeOptions, twoColumns, stack }
    },
    template: `
      <RlConfigShell active-id="relations" title="implements" back-label="Relations" back-to="mockups-configure-data-model--overview">
        <div :style="twoColumns">
          <div :style="stack">
            <RlCallout tone="info" title="Reads as">
              A {{ from.join(' or ').toLowerCase() || '…' }} <strong>{{ name || '…' }}</strong> a {{ to.join(' or ').toLowerCase() || '…' }}.
              A {{ to.join(' or ').toLowerCase() || '…' }} is <strong>{{ inverse || '…' }}</strong> a {{ from.join(' or ').toLowerCase() || '…' }}.
            </RlCallout>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px">
              <RlMultiSelect v-model="from" label="From" :options="entityTypeOptions" required />
              <RlMultiSelect v-model="to" label="To" :options="entityTypeOptions" required />
              <RlTextField v-model="name" label="Name" required />
              <RlTextField v-model="inverse" label="Name the other way round" hint="Shown on the target's page." />
            </div>
            <RlCheckbox v-model="required" :label="'Every ' + (from[0] ?? 'record').toLowerCase() + ' needs one'" />
            <RlNumberField v-if="required" v-model="atLeast" label="At least" :min="1" style="max-width:160px" />
            <RlTextarea model-value="Ticket delivers this feature" label="Description" :rows="2" />
          </div>
          <aside :style="stack">
            <RlHeading :level="2" size="md">In use</RlHeading>
            <RlText size="sm" tone="muted" as="p" style="margin:0">
              1,402 links. 214 tickets have none, so they break the "at least 1" requirement and are listed
              under Analyze.
            </RlText>
          </aside>
        </div>
      </RlConfigShell>
    `,
  }),
}

/**
 * A rule. Conditions are expressions, typed as text: there is no builder yet,
 * and an expression field with an inline error is the stopgap. The count of
 * records breaking the rule today is the check that it says what was meant.
 */
export const Rule: Story = {
  render: () => ({
    components: { RlConfigShell, RlTextField, RlSelect, RlRadioGroup, RlCallout, RlText, RlHeading },
    setup() {
      const when = ref("entity.status == 'ready'")
      const then = ref('entity.effort != nil and entity.priorty != nil')
      const severity = ref('error')
      const error = computed(() =>
        then.value.includes('priorty') ? 'Ticket has no property "priorty". Did you mean "priority"?' : undefined,
      )
      return { when, then, severity, error, entityTypeOptions, twoColumns, stack }
    },
    template: `
      <RlConfigShell active-id="rules" title="Ready tickets need an estimate" back-label="Rules" back-to="mockups-configure-data-model--overview">
        <div :style="twoColumns">
          <div :style="stack">
            <RlTextField model-value="Ready tickets need an estimate" label="Name" required />
            <RlSelect model-value="Ticket" label="Applies to" :options="entityTypeOptions" />
            <RlTextField v-model="when" label="When" hint="The rule is checked only for records that match this." style="font-family:var(--rl-font-family-mono)" />
            <RlTextField v-model="then" label="Then" :error="error" hint="What must be true of those records." style="font-family:var(--rl-font-family-mono)" />
            <RlRadioGroup
              v-model="severity"
              label="When a record breaks it"
              :options="[
                { value: 'error', label: 'Block saving', description: 'The record cannot be saved until it is fixed.' },
                { value: 'warning', label: 'Show a warning', description: 'The record saves; the warning shows on its page.' },
              ]"
            />
          </div>
          <aside :style="stack">
            <RlHeading :level="2" size="md">Today</RlHeading>
            <RlCallout v-if="!error" tone="warning" title="6 tickets break this rule">
              They keep working. Each shows the problem until someone fixes it.
            </RlCallout>
            <RlText v-else size="sm" tone="muted" as="p" style="margin:0">Fix the condition to see which tickets break the rule.</RlText>
          </aside>
        </div>
      </RlConfigShell>
    `,
  }),
}

/**
 * An automation: one trigger, then an ordered list of actions. The actions
 * run in the order shown, so the list is sortable.
 */
export const Automation: Story = {
  render: () => ({
    components: { RlConfigShell, RlSelect, RlTextField, RlSortableList: RlSortableListAny, RlSectionHeading, RlButton, RlText, RlTag, RlMenu, RlMenuItem },
    setup() {
      const actions = ref([
        { id: 'create', title: 'Create a Planning checklist', detail: 'titled "Planning: {title}", linked by has planning, unless one exists' },
        { id: 'set', title: 'Set Started to today', detail: 'only if Started is empty' },
        { id: 'notify', title: 'Send a notification', detail: 'to the assignee' },
      ])
      const move = (item: (typeof actions.value)[number], to: number) => (actions.value = reorder(actions.value, item, to))
      return { actions, move, entityTypeOptions, stack }
    },
    template: `
      <RlConfigShell active-id="automations" title="Start planning" back-label="Automations" back-to="mockups-configure-data-model--overview">
        <div style="max-width:720px; display:flex; flex-direction:column; gap:32px">
          <section :style="stack">
            <RlSectionHeading title="When" :level="2" />
            <div style="display:grid; grid-template-columns:repeat(3, 1fr); gap:16px">
              <RlSelect model-value="Ticket" label="A record of type" :options="entityTypeOptions" />
              <RlSelect model-value="status" label="Changes" :options="[{ value: 'status', label: 'Status' }, { value: 'priority', label: 'Priority' }]" />
              <RlSelect model-value="planning" label="To" :options="[{ value: 'planning', label: 'Planning' }, { value: 'in-progress', label: 'In progress' }, { value: 'review', label: 'Review' }]" />
            </div>
          </section>
          <section :style="stack">
            <RlSectionHeading title="Then" :count="actions.length" :level="2" />
            <RlSortableList :items="actions" label="Actions, in the order they run" @move="move">
              <template #item="{ item, index }">
                <div style="display:flex; align-items:baseline; gap:8px">
                  <RlTag :label="String(index + 1)" />
                  <RlText size="sm" weight="medium">{{ item.title }}</RlText>
                  <RlText size="sm" tone="subtle">{{ item.detail }}</RlText>
                </div>
              </template>
            </RlSortableList>
            <div>
              <RlMenu>
                <template #trigger="{ toggle, attrs }">
                  <RlButton variant="secondary" size="sm" icon="plus" v-bind="attrs" @click="toggle">Add action</RlButton>
                </template>
                <RlMenuItem icon="file-add">Create a record</RlMenuItem>
                <RlMenuItem icon="edit">Set a property</RlMenuItem>
                <RlMenuItem icon="link">Link to a record</RlMenuItem>
                <RlMenuItem icon="mail">Send a notification</RlMenuItem>
              </RlMenu>
            </div>
          </section>
        </div>
      </RlConfigShell>
    `,
  }),
}

/**
 * The review step. Every change in the draft, grouped by what it changes and
 * worded as a change to that thing. A change that does not fit existing
 * records (removing a status three tickets use) makes saving also generate a
 * data migration; the review shows its steps and asks the one thing the draft
 * cannot decide, where those tickets go.
 */
export const ReviewAndSave: Story = {
  render: () => ({
    components: { RlConfigShell, RlText },
    template: `
      <RlConfigShell active-id="choice-lists" title="Ticket status" back-label="Choice lists" back-to="mockups-configure-data-model--overview" review-open>
        <RlText tone="muted">The page behind the review drawer.</RlText>
      </RlConfigShell>
    `,
  }),
}

/**
 * Every data migration that has run, newest first. Most were generated by
 * saving a draft; a data-only one fixes records without changing the model.
 * Opening one shows its steps, worded the way the review showed them.
 */
export const Migrations: Story = {
  render: () => ({
    components: { RlConfigShell, RlTable: RlTableAny, RlText, RlTag, RlDrawer, RlChangeList, RlChangeItem },
    setup() {
      const open = ref<MigrationRow | null>(null)
      const sections: Section<MigrationRow>[] = [{ id: 'all', title: '', items: migrations }]
      const columns: TableColumn[] = [
        { key: 'origin', header: 'Made by', width: 130 },
        { key: 'records', header: 'Records changed', width: 150, align: 'end' },
        { key: 'by', header: 'By', width: 170, field: 'by' },
        { key: 'date', header: 'When', width: 140, field: 'date' },
      ]
      return { open, sections, columns }
    },
    template: `
      <RlConfigShell active-id="migrations" title="Migrations" :draft-count="0">
        <div style="max-width:1000px; display:flex; flex-direction:column; gap:16px">
          <RlText tone="muted" size="sm" as="p" style="margin:0">
            A migration updates existing records once, when a change to the data model does not fit them. Saving a
            draft generates one when it needs to.
          </RlText>
          <RlTable
            :sections="sections"
            :columns="columns"
            name-label="Migration"
            :show-section-header="false"
            :show-add="false"
            :selected-id="open?.id"
            @select="open = $event"
          >
            <template #cell-origin="{ item }">
              <RlTag :label="item.origin" :color="item.origin === 'Data only' ? 'purple' : 'grey'" />
            </template>
            <template #cell-records="{ item }">
              <RlText size="sm">{{ item.records.toLocaleString('en-GB') }}</RlText>
            </template>
          </RlTable>
        </div>

        <RlDrawer :open="open !== null" :title="open?.title ?? ''" size="md" @close="open = null">
          <div v-if="open" style="display:flex; flex-direction:column; gap:16px">
            <RlText size="sm" tone="muted">{{ open.origin === 'Data only' ? 'A data-only fix' : 'Generated by a saved draft' }}, by {{ open.by }}, {{ open.date }}.</RlText>
            <RlChangeList title="Steps" :count="open.steps.length">
              <RlChangeItem
                v-for="step in open.steps"
                :key="step.label"
                :kind="step.kind"
                :label="step.label"
                :detail="step.detail"
                :before="step.before"
                :after="step.after"
              />
            </RlChangeList>
          </div>
        </RlDrawer>
      </RlConfigShell>
    `,
  }),
}
