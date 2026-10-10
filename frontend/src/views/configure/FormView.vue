<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlHeading from 'rela-components/components/common/RlHeading.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSplit from '@/components/configure/ConfigSplit.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import ConfigPreview from '@/components/configure/ConfigPreview.vue'
import FieldDrawer from '@/components/configure/FieldDrawer.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypeLabel, formModel, propertiesOf } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { getIn, isMap, newMap, remove, type TreeValue } from '@/configure/tree'

/**
 * A form. Fields are a sortable list; the preview beside it is the form's
 * layout as it will render, and follows every reorder and width change.
 */
const props = defineProps<{ name: string }>()
const draft = useConfigDraftStore()
const router = useRouter()

const form = computed(() => formModel(draft.currentDataEntry, props.name))
const path = computed(() => ['forms', props.name])
const editing = ref<number | undefined>(undefined)

const properties = computed(() =>
  form.value ? propertiesOf(draft.currentSchema, form.value.entityType) : []
)
const typeOf = (property: string) =>
  properties.value.find((p) => p.name === property)?.typeLabel ?? ''

const rows = computed(() =>
  (form.value?.fields ?? []).map((f) => ({
    ...f,
    id: String(f.index),
    title: f.label || f.property,
    transitionCount: Object.keys(f.transitions).length,
  }))
)
const widthLabel = (span?: number) =>
  span === 4 ? 'Third' : span === 6 ? 'Half' : span && span < 12 ? `${span} of 12` : 'Full'

function move(row: { index: number }, to: number) {
  draft.moveAt('screens', [...path.value, 'fields'], row.index, to)
}

const unused = computed(() =>
  properties.value
    .filter((p) => !(form.value?.fields ?? []).some((f) => f.property === p.name))
    .map((p) => ({ value: p.name, label: p.label }))
)
const adding = ref('')

function addField() {
  if (!adding.value) return
  draft.appendAt('screens', path.value, 'fields', newMap({ property: adding.value }))
  adding.value = ''
}

function setting(key: string, value: TreeValue | undefined) {
  draft.setAt('screens', path.value, key, value)
}

function removeForm() {
  draft.edit('screens', (tree) => {
    const all = getIn(tree, ['forms'])
    if (isMap(all)) remove(all, props.name)
  })
  void router.push(configureRoute.forms())
}

const modeOptions = [
  { value: 'create', label: 'Creates a record' },
  { value: 'edit', label: 'Edits a record' },
]
</script>

<template>
  <ConfigurePage :title="form?.title ?? name" back-label="Forms" :back-to="configureRoute.forms()">
    <template v-if="form" #actions>
      <RlButton variant="ghost" tone="danger" icon="delete" @click="removeForm">Remove</RlButton>
    </template>
    <RlEmptyState v-if="!form" title="No such form" />
    <ConfigSplit v-else aside="preview">
      <RlCallout v-if="form.hasSteps" tone="info" title="A form with steps">
        This form asks its questions in steps. Its steps are changed in the project's files for now;
        the fields below are the ones outside any step.
      </RlCallout>
      <ConfigSection title="Fields" :count="rows.length">
        <RlSortableList
          :items="rows"
          :label="`Fields of ${form.title}`"
          data-testid="config-fields"
          @move="move"
        >
          <template #item="{ item }">
            <button type="button" class="field-row" @click="editing = item.index">
              <RlText size="sm" weight="medium">{{ item.title }}</RlText>
              <RlText size="sm" tone="subtle">{{ typeOf(item.property) }}</RlText>
              <span class="field-row__tags">
                <RlTag v-if="item.hidden" label="Hidden" />
                <RlTag
                  v-if="item.transitionCount"
                  :label="`${item.transitionCount} allowed changes`"
                  color="purple"
                />
                <RlTag :label="widthLabel(item.span)" />
              </span>
            </button>
          </template>
        </RlSortableList>
        <form v-if="unused.length" class="add-field" @submit.prevent="addField">
          <RlSelect
            v-model="adding"
            label="Add a field"
            :options="unused"
            placeholder="Choose a property"
          />
          <RlButton variant="secondary" size="sm" icon="plus" type="submit" :disabled="!adding"
            >Add field</RlButton
          >
        </form>
      </ConfigSection>
      <ConfigSection
        v-if="form.relations.length"
        title="Linked records"
        :count="form.relations.length"
      >
        <RlSortableList
          :items="
            form.relations.map((r) => ({ ...r, id: String(r.index), title: r.label || r.relation }))
          "
          :label="`Linked records on ${form.title}`"
          @move="(row, to) => draft.moveAt('screens', [...path, 'relations'], row.index, to)"
        >
          <template #item="{ item }">
            <div class="field-row">
              <RlText size="sm" weight="medium">{{ item.title }}</RlText>
              <RlText size="sm" tone="subtle">{{ item.relation }}</RlText>
            </div>
          </template>
        </RlSortableList>
      </ConfigSection>
      <ConfigSection title="Form">
        <RlTextField
          :model-value="form.title"
          label="Title"
          required
          @update:model-value="setting('title', $event)"
        />
        <RlText size="sm" tone="muted">
          For {{ entityTypeLabel(draft.currentSchema, form.entityType) }} records.
        </RlText>
        <RlSelect
          :model-value="form.mode"
          label="Mode"
          :options="modeOptions"
          @update:model-value="setting('mode', $event)"
        />
        <RlTextarea
          :model-value="form.description"
          label="Description"
          :rows="2"
          @update:model-value="setting('description', $event)"
        />
        <RlCheckbox
          :model-value="form.body"
          label="Include the description editor"
          @update:model-value="setting('body', $event)"
        />
      </ConfigSection>

      <template #aside>
        <ConfigPreview subject="Form" sticky>
          <RlHeading :level="2" size="lg">{{ form.title }}</RlHeading>
          <div class="preview-grid">
            <div
              v-for="f in form.fields.filter((x) => !x.hidden)"
              :key="f.index"
              :style="{ gridColumn: `span ${f.span ?? 12}` }"
            >
              <RlTextField
                :model-value="''"
                :label="f.label || f.property"
                :placeholder="f.placeholder"
                :hint="f.help || undefined"
                readonly
              />
            </div>
            <div v-for="r in form.relations" :key="`r${r.index}`" class="preview-grid__full">
              <RlTextField
                :model-value="''"
                :label="r.label || r.relation"
                placeholder="Link a record…"
                readonly
              />
            </div>
            <div v-if="form.body" class="preview-grid__full">
              <RlTextarea :model-value="''" label="Description" :rows="3" readonly />
            </div>
          </div>
        </ConfigPreview>
      </template>
    </ConfigSplit>
    <FieldDrawer :form="name" :index="editing" @close="editing = undefined" />
  </ConfigurePage>
</template>

<style scoped>
.field-row {
  all: unset;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  width: 100%;
  cursor: pointer;
}

.field-row:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
}

.field-row__tags {
  margin-left: auto;
  display: flex;
  gap: 6px;
}

.add-field {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: end;
  gap: var(--rl-space-3);
  max-width: 480px;
}

.preview-grid {
  display: grid;
  grid-template-columns: repeat(12, 1fr);
  gap: var(--rl-space-4);
}

.preview-grid__full {
  grid-column: span 12;
}
</style>
