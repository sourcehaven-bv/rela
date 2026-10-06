<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlBoard from 'rela-components/components/board/RlBoard.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlMultiSelect from 'rela-components/components/form/RlMultiSelect.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlHeading from 'rela-components/components/common/RlHeading.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import type { Section } from 'rela-components/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSplit from '@/components/configure/ConfigSplit.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import ConfigPreview from '@/components/configure/ConfigPreview.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import {
  boardModel,
  chooseCardFields,
  choiceListModel,
  entityTypeLabel,
  propertiesOf,
  queryScopeNames,
  relationsOfType,
} from '@/configure/models'
import FilterControlsEditor from '@/components/configure/FilterControlsEditor.vue'
import FixedFiltersEditor from '@/components/configure/FixedFiltersEditor.vue'
import { configureRoute } from '@/configure/routes'
import { getIn, isMap, newMap, remove, set, type TreeValue } from '@/configure/tree'

/**
 * A board. Without its own columns a board shows one column per option of
 * the column property; choosing columns here lets it leave some out or name
 * them differently.
 */
const props = defineProps<{ name: string }>()
const draft = useConfigDraftStore()
const router = useRouter()

const board = computed(() => boardModel(draft.currentDataEntry, props.name))
const path = computed(() => ['kanbans', props.name])
const properties = computed(() =>
  board.value ? propertiesOf(draft.currentSchema, board.value.entityType) : []
)
const choiceProperties = computed(() =>
  properties.value
    .filter((p) => p.choiceList || p.inlineValues.length)
    .map((p) => ({ value: p.name, label: p.label }))
)

/** The options of a property, from its choice list or its own values. */
function optionsOf(property: string): string[] {
  const p = properties.value.find((x) => x.name === property)
  if (!p) return []
  if (p.choiceList)
    return (
      choiceListModel(draft.currentSchema, undefined, p.choiceList)?.options.map((o) => o.value) ??
      []
    )
  return p.inlineValues
}

const options = computed(() => (board.value ? optionsOf(board.value.columnProperty) : []))
const ownColumns = computed(() => (board.value?.columns.length ?? 0) > 0)
const rows = computed(() =>
  (board.value?.columns ?? []).map((c) => ({
    ...c,
    id: String(c.index),
    title: c.label || c.value,
  }))
)
const missing = computed(() =>
  options.value.filter((o) => !(board.value?.columns ?? []).some((c) => c.value === o))
)

function setting(key: string, value: TreeValue | undefined) {
  draft.setAt('screens', path.value, key, value)
}

function setColumnProperty(value: string) {
  draft.edit('screens', (tree) => {
    const def = getIn(tree, path.value)
    if (!isMap(def)) return
    set(def, 'column_property', value)
    // Columns name values of the old property; they mean nothing for the new one.
    remove(def, 'columns')
  })
}

function chooseColumns() {
  draft.edit('screens', (tree) => {
    const def = getIn(tree, path.value)
    if (isMap(def))
      set(
        def,
        'columns',
        options.value.map((value) => newMap({ value }))
      )
  })
}

function useAllOptions() {
  draft.edit('screens', (tree) => {
    const def = getIn(tree, path.value)
    if (isMap(def)) remove(def, 'columns')
  })
}

function setCardFields(fields: string[]) {
  draft.edit('screens', (tree) => {
    const def = getIn(tree, path.value)
    if (!isMap(def)) return
    const card = getIn(def, ['card'])
    const target = isMap(card) ? card : newMap()
    const chosen = chooseCardFields(getIn(target, ['fields']), fields)
    if (chosen.length) set(target, 'fields', chosen)
    else remove(target, 'fields')
    if (!isMap(card)) set(def, 'card', target)
  })
}

const propertyOptions = computed(() =>
  properties.value.map((p) => ({ value: p.name, label: p.label }))
)
const typeRelations = computed(() =>
  board.value ? relationsOfType(draft.currentSchema, board.value.entityType) : []
)
const scopeOptions = computed(() => [
  { value: '', label: 'Every record' },
  ...(board.value ? queryScopeNames(draft.currentSchema, board.value.entityType) : []).map((n) => ({
    value: n,
    label: n,
  })),
])

function removeBoard() {
  draft.edit('screens', (tree) => {
    const all = getIn(tree, ['kanbans'])
    if (isMap(all)) remove(all, props.name)
  })
  void router.push(configureRoute.boards())
}

const previewSections = computed<Section<{ id: string; title: string }>[]>(() =>
  (ownColumns.value
    ? rows.value.map((r) => ({ id: r.value, title: r.title }))
    : options.value.map((o) => ({ id: o, title: o }))
  ).map((c) => ({ id: c.id, title: c.title, items: [] }))
)
</script>

<template>
  <ConfigurePage
    :title="board?.title ?? name"
    back-label="Boards"
    :back-to="configureRoute.boards()"
  >
    <template v-if="board" #actions>
      <RlButton variant="ghost" tone="danger" icon="delete" @click="removeBoard">Remove</RlButton>
    </template>
    <RlEmptyState v-if="!board" title="No such board" />
    <ConfigSplit v-else>
      <ConfigSection title="Columns" :count="ownColumns ? rows.length : options.length">
        <template v-if="!ownColumns">
          <RlText size="sm" tone="muted" as="p" class="para">
            The board shows one column for each option of {{ board.columnProperty }}, in the order
            of the options.
          </RlText>
          <RlButton variant="secondary" size="sm" class="start" @click="chooseColumns"
            >Choose columns</RlButton
          >
        </template>
        <template v-else>
          <RlSortableList
            :items="rows"
            :label="`Columns of ${board.title}`"
            data-testid="config-board-columns"
            @move="(row, to) => draft.moveAt('screens', [...path, 'columns'], row.index, to)"
          >
            <template #item="{ item }">
              <div class="column-row">
                <RlTextField
                  :model-value="item.label"
                  :label="`Header of ${item.value}`"
                  label-hidden
                  :placeholder="item.value"
                  size="sm"
                  class="column-row__label"
                  @update:model-value="
                    draft.setAt('screens', [...path, 'columns', item.index], 'label', $event)
                  "
                />
                <RlText size="sm" tone="subtle" class="column-row__value">{{ item.value }}</RlText>
                <RlIconButton
                  icon="delete"
                  :label="`Remove ${item.title}`"
                  @click="draft.removeAt('screens', [...path, 'columns'], item.index)"
                />
              </div>
            </template>
          </RlSortableList>
          <div class="buttons">
            <RlButton
              v-for="value in missing"
              :key="value"
              variant="secondary"
              size="sm"
              icon="plus"
              @click="draft.appendAt('screens', path, 'columns', newMap({ value }))"
            >
              {{ value }}
            </RlButton>
            <RlButton variant="ghost" size="sm" @click="useAllOptions">Show every option</RlButton>
          </div>
        </template>
      </ConfigSection>

      <ConfigSection
        title="Filters people can set"
        :count="board.filterControls.length"
        description="Shown above the board. Each filters on a property or a relation."
      >
        <FilterControlsEditor
          :path="path"
          :controls="board.filterControls"
          :properties="propertyOptions"
          :relations="typeRelations"
        />
      </ConfigSection>

      <ConfigSection
        title="Which records it shows"
        description="These always apply; people cannot turn them off."
      >
        <RlSelect
          :model-value="board.queryScope"
          label="Query scope"
          :options="scopeOptions"
          hint="A named set of records declared on the entity type."
          @update:model-value="setting('query_scope', $event)"
        />
        <FixedFiltersEditor
          :path="path"
          :filters="board.fixedFilters"
          :properties="propertyOptions"
        />
      </ConfigSection>

      <template #aside>
        <section class="settings">
          <RlHeading :level="2" size="md">Settings</RlHeading>
          <RlTextField
            :model-value="board.title"
            label="Title"
            required
            @update:model-value="setting('title', $event)"
          />
          <RlText size="sm" tone="muted"
            >Shows {{ entityTypeLabel(draft.currentSchema, board.entityType) }} records.</RlText
          >
          <RlSelect
            :model-value="board.columnProperty"
            label="Columns by"
            :options="choiceProperties"
            @update:model-value="setColumnProperty"
          />
          <RlSelect
            :model-value="board.swimlaneProperty"
            label="Rows by"
            :options="[{ value: '', label: 'No rows' }, ...choiceProperties]"
            @update:model-value="setting('swimlane_property', $event)"
          />
          <RlMultiSelect
            :model-value="board.cardFields"
            label="Shown on cards"
            :options="properties.map((p) => ({ value: p.name, label: p.label }))"
            @update:model-value="setCardFields"
          />
        </section>
        <ConfigPreview subject="Board columns">
          <RlBoard :sections="previewSections" :show-add="false" empty-label="No cards" />
        </ConfigPreview>
      </template>
    </ConfigSplit>
  </ConfigurePage>
</template>

<style scoped>
.para {
  margin: 0;
}

.start {
  align-self: flex-start;
}

.column-row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
}

.column-row__label {
  flex: 1;
}

.column-row__value {
  width: 140px;
}

.buttons {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-2);
}

.settings {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}
</style>
