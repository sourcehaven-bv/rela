<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlTable from 'rela-components/components/table/RlTable.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlNumberField from 'rela-components/components/form/RlNumberField.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlSwitch from 'rela-components/components/form/RlSwitch.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlHeading from 'rela-components/components/common/RlHeading.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import type { Section } from 'rela-components/types'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSplit from '@/components/configure/ConfigSplit.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import ConfigPreview from '@/components/configure/ConfigPreview.vue'
import SortEditor from '@/components/configure/SortEditor.vue'
import FilterControlsEditor from '@/components/configure/FilterControlsEditor.vue'
import FixedFiltersEditor from '@/components/configure/FixedFiltersEditor.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import {
  columnTitle,
  entityTypeLabel,
  forms,
  listModel,
  propertiesOf,
  queryScopeNames,
  relationsOfType,
} from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { getIn, isMap, keys, newMap, remove, type TreeValue } from '@/configure/tree'

/**
 * A list. Columns are a sortable list with a switch for whether people can
 * sort by each; the preview is the table header as the list will show it.
 */
const props = defineProps<{ name: string }>()
const draft = useConfigDraftStore()
const router = useRouter()

const list = computed(() => listModel(draft.currentDataEntry, props.name))
const path = computed(() => ['lists', props.name])
const properties = computed(() =>
  list.value ? propertiesOf(draft.currentSchema, list.value.entityType) : []
)

const rows = computed(() =>
  (list.value?.columns ?? []).map((c) => ({ ...c, id: String(c.index), title: columnTitle(c) }))
)

function setting(key: string, value: TreeValue | undefined) {
  draft.setAt('screens', path.value, key, value)
}

function column(index: number, key: string, value: TreeValue | undefined) {
  draft.setAt('screens', [...path.value, 'columns', index], key, value)
}

const unused = computed(() =>
  properties.value
    .filter((p) => !(list.value?.columns ?? []).some((c) => c.property === p.name))
    .map((p) => ({ value: p.name, label: p.label }))
)
const adding = ref('')

function addColumn() {
  if (!adding.value) return
  draft.appendAt('screens', path.value, 'columns', newMap({ property: adding.value }))
  adding.value = ''
}

const propertyOptions = computed(() =>
  properties.value.map((p) => ({ value: p.name, label: p.label }))
)
const typeRelations = computed(() =>
  list.value ? relationsOfType(draft.currentSchema, list.value.entityType) : []
)
const scopeOptions = computed(() => [
  { value: '', label: 'Every record' },
  ...(list.value ? queryScopeNames(draft.currentSchema, list.value.entityType) : []).map((n) => ({
    value: n,
    label: n,
  })),
])
const viewOptions = computed(() => [
  { value: '', label: 'The default page' },
  ...keys(getIn(draft.currentDataEntry, ['views'])).map((n) => ({ value: n, label: n })),
])
const formOptions = computed(() => [
  { value: '', label: 'None' },
  ...forms(draft.currentDataEntry)
    .filter((f) => f.entityType === list.value?.entityType)
    .map((f) => ({ value: f.name, label: f.title })),
])

function removeList() {
  draft.edit('screens', (tree) => {
    const all = getIn(tree, ['lists'])
    if (isMap(all)) remove(all, props.name)
  })
  void router.push(configureRoute.lists())
}

const previewColumns = computed<TableColumn[]>(() =>
  rows.value.slice(1).map((c) => ({ key: c.id, header: c.title, sortable: c.sortable }))
)
const previewSections = computed<Section<{ id: string; title: string }>[]>(() => [
  { id: 'preview', title: list.value?.title ?? '', items: [] },
])
</script>

<template>
  <ConfigurePage :title="list?.title ?? name" back-label="Lists" :back-to="configureRoute.lists()">
    <template v-if="list" #actions>
      <RlButton variant="ghost" tone="danger" icon="delete" @click="removeList">Remove</RlButton>
    </template>
    <RlEmptyState v-if="!list" title="No such list" />
    <ConfigSplit v-else>
      <ConfigSection
        title="Columns"
        :count="rows.length"
        description="The first column holds each record's title."
      >
        <RlSortableList
          :items="rows"
          :label="`Columns of ${list.title}`"
          data-testid="config-columns"
          @move="(row, to) => draft.moveAt('screens', [...path, 'columns'], row.index, to)"
        >
          <template #item="{ item }">
            <div class="column-row">
              <RlTextField
                :model-value="item.label"
                :label="`Header of ${item.title}`"
                label-hidden
                :placeholder="item.property || item.relation"
                size="sm"
                class="column-row__label"
                @update:model-value="column(item.index, 'label', $event)"
              />
              <RlText size="sm" tone="subtle" class="column-row__source">{{
                item.property || item.relation
              }}</RlText>
              <RlSwitch
                :model-value="item.sortable"
                label="Sortable"
                @update:model-value="column(item.index, 'sortable', $event)"
              />
              <RlIconButton
                icon="delete"
                :label="`Remove ${item.title}`"
                @click="draft.removeAt('screens', [...path, 'columns'], item.index)"
              />
            </div>
          </template>
        </RlSortableList>
        <form v-if="unused.length" class="add-row" @submit.prevent="addColumn">
          <RlSelect
            v-model="adding"
            label="Add a column"
            :options="unused"
            placeholder="Choose a property"
          />
          <RlButton variant="secondary" size="sm" icon="plus" type="submit" :disabled="!adding"
            >Add column</RlButton
          >
        </form>
      </ConfigSection>

      <ConfigSection
        title="Filters people can set"
        :count="list.filterControls.length"
        description="Shown above the list. Each filters on a property or a relation."
      >
        <FilterControlsEditor
          :path="path"
          :controls="list.filterControls"
          :properties="propertyOptions"
          :relations="typeRelations"
        />
      </ConfigSection>

      <ConfigSection
        title="Which records it shows"
        description="These always apply; people cannot turn them off."
      >
        <RlSelect
          :model-value="list.queryScope"
          label="Query scope"
          :options="scopeOptions"
          hint="A named set of records declared on the entity type."
          @update:model-value="setting('query_scope', $event)"
        />
        <FixedFiltersEditor
          :path="path"
          :filters="list.fixedFilters"
          :properties="propertyOptions"
        />
        <RlTextarea
          :model-value="list.condition"
          label="Condition"
          :rows="2"
          placeholder="entity.status ~= 'done' and entity.priority == 'high'"
          hint="An expression for rules the conditions above cannot express, such as or. Review checks it."
          @update:model-value="setting('condition', $event)"
        />
      </ConfigSection>

      <template #aside>
        <section class="settings">
          <RlHeading :level="2" size="md">Settings</RlHeading>
          <RlTextField
            :model-value="list.title"
            label="Title"
            required
            @update:model-value="setting('title', $event)"
          />
          <RlText size="sm" tone="muted"
            >Shows {{ entityTypeLabel(draft.currentSchema, list.entityType) }} records.</RlText
          >
          <SortEditor file="screens" :path="path" :keys="list.sort" :properties="propertyOptions" />
          <RlNumberField
            :model-value="list.pageSize"
            label="Rows per page"
            :min="1"
            @update:model-value="setting('page_size', $event)"
          />
          <RlSelect
            :model-value="list.createForm"
            label="New records use"
            :options="formOptions"
            @update:model-value="setting('create_form', $event)"
          />
          <RlSelect
            :model-value="list.detailView"
            label="Records open in"
            :options="viewOptions"
            @update:model-value="setting('detail_view', $event)"
          />
          <RlTextField
            :model-value="list.description"
            label="Description"
            @update:model-value="setting('description', $event)"
          />
        </section>
        <ConfigPreview subject="Table header">
          <RlTable
            :sections="previewSections"
            :columns="previewColumns"
            :name-label="rows[0]?.title ?? 'Name'"
            :show-section-header="false"
            :show-add="false"
          />
        </ConfigPreview>
      </template>
    </ConfigSplit>
  </ConfigurePage>
</template>

<style scoped>
.column-row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
}

.column-row__label {
  flex: 1;
}

.column-row__source {
  width: 120px;
}

.add-row {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: end;
  gap: var(--rl-space-3);
  max-width: 480px;
}

.settings {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}
</style>
