<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlOptionSelect from 'rela-components/components/form/RlOptionSelect.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlHeading from 'rela-components/components/common/RlHeading.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigSplit from '@/components/configure/ConfigSplit.vue'
import ConfigSection from '@/components/configure/ConfigSection.vue'
import ConfigPreview from '@/components/configure/ConfigPreview.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { choiceListModel, entityTypeLabel } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { OPTION_COLORS, colorLabel, tagColor } from '@/configure/names'
import { getIn, isMap, set, strList } from '@/configure/tree'

/**
 * A choice list. Options are sortable, each with a colour. A removed option
 * stays in view, struck through, until the draft is saved or the removal is
 * undone; the review asks where records holding it go.
 */
const props = defineProps<{ name: string }>()
const draft = useConfigDraftStore()

const model = computed(() =>
  choiceListModel(draft.currentSchema, draft.currentDataEntry, props.name)
)
const baseValues = computed(() =>
  strList(getIn(draft.base?.schema, ['types', props.name, 'values']))
)

interface Row {
  id: string
  title: string
  color?: string
  added: boolean
  removed: boolean
  isDefault: boolean
}

const rows = computed<Row[]>(() => {
  const m = model.value
  if (!m) return []
  const live = m.options.map((o) => ({
    id: o.value,
    title: o.label,
    color: o.color,
    added: !baseValues.value.includes(o.value),
    removed: false,
    isDefault: m.default === o.value,
  }))
  const gone = baseValues.value
    .filter((v) => !m.options.some((o) => o.value === v))
    .map((v) => ({ id: v, title: v, added: false, removed: true, isDefault: false }))
  return [...live, ...gone]
})

const liveCount = computed(() => rows.value.filter((r) => !r.removed).length)

function setValues(fn: (values: string[]) => string[]) {
  draft.edit('schema', (tree) => {
    const def = getIn(tree, ['types', props.name])
    if (isMap(def)) set(def, 'values', fn(strList(getIn(def, ['values']))))
  })
}

function move(row: Row, to: number) {
  if (row.removed) return
  setValues((values) => {
    const rest = values.filter((v) => v !== row.id)
    rest.splice(Math.min(to, rest.length), 0, row.id)
    return rest
  })
}

function removeOption(row: Row) {
  setValues((values) => values.filter((v) => v !== row.id))
  if (model.value?.default === row.id)
    draft.setAt('schema', ['types', props.name], 'default', undefined)
}

function restore(row: Row) {
  setValues((values) => [...values, row.id])
}

function setColor(row: Row, color: string) {
  draft.setAt('screens', ['styles', props.name], row.id, color || undefined)
}

const newOption = ref('')
const newError = computed(() =>
  newOption.value.trim() && rows.value.some((r) => r.id === newOption.value.trim())
    ? 'This option exists.'
    : undefined
)

function addOption() {
  const value = newOption.value.trim()
  if (!value || newError.value) return
  setValues((values) => [...values, value])
  newOption.value = ''
}

const defaultOptions = computed(() => [
  { value: '', label: 'None' },
  ...(model.value?.options ?? []).map((o) => ({ value: o.value, label: o.label })),
])
</script>

<template>
  <ConfigurePage
    :title="model?.label ?? name"
    back-label="Choice lists"
    :back-to="configureRoute.choiceLists()"
  >
    <RlEmptyState v-if="!model" title="No such choice list" />
    <ConfigSplit v-else>
      <ConfigSection
        title="Options"
        :count="liveCount"
        description="The order here is the order of a status menu and of board columns."
      >
        <RlSortableList
          :items="rows"
          :label="`Options of ${model.label}`"
          data-testid="config-options"
          @move="move"
        >
          <template #item="{ item }">
            <div class="option-row">
              <div class="option-row__color">
                <RlOptionSelect
                  variant="inline"
                  :model-value="item.color ?? ''"
                  :options="OPTION_COLORS"
                  :label="`Colour of ${item.title}`"
                  placeholder="No colour"
                  :disabled="item.removed"
                  @update:model-value="setColor(item, $event)"
                >
                  <template #option="{ value }">
                    <RlTag :label="colorLabel(value)" :color="tagColor(value)" />
                  </template>
                </RlOptionSelect>
              </div>
              <RlText
                size="sm"
                weight="medium"
                class="option-row__title"
                :class="{ 'option-row__title--removed': item.removed }"
              >
                {{ item.title }}
              </RlText>
              <RlTag v-if="item.added" label="New" color="green" />
              <RlTag v-if="item.removed" label="Removed" color="red" />
              <RlTag v-if="item.isDefault" label="Default" color="blue" />
              <div class="option-row__action">
                <RlButton v-if="item.removed" size="sm" variant="ghost" @click="restore(item)"
                  >Undo</RlButton
                >
                <RlIconButton
                  v-else
                  icon="delete"
                  :label="`Remove ${item.title}`"
                  @click="removeOption(item)"
                />
              </div>
            </div>
          </template>
        </RlSortableList>
        <form class="add-option" @submit.prevent="addOption">
          <RlTextField v-model="newOption" label="New option" size="sm" :error="newError" />
          <RlButton
            variant="secondary"
            icon="plus"
            size="sm"
            type="submit"
            :disabled="!newOption.trim() || !!newError"
          >
            Add option
          </RlButton>
        </form>
      </ConfigSection>

      <template #aside>
        <section class="settings">
          <RlHeading :level="2" size="md">Settings</RlHeading>
          <RlSelect
            :model-value="model.default ?? ''"
            label="Default"
            :options="defaultOptions"
            hint="Filled in on new records."
            @update:model-value="
              draft.setAt('schema', ['types', name], 'default', $event || undefined)
            "
          />
          <div>
            <RlText size="sm" weight="medium" as="div">Used by</RlText>
            <RlText
              v-if="!model.usedBy.length"
              size="sm"
              tone="muted"
              as="p"
              class="settings__para"
            >
              No property uses it yet.
            </RlText>
            <ul v-else class="settings__list">
              <li v-for="u in model.usedBy" :key="`${u.entityType}.${u.property}`">
                <RouterLink
                  :to="`${configureRoute.entityType(u.entityType)}?property=${encodeURIComponent(u.property)}`"
                >
                  {{ entityTypeLabel(draft.currentSchema, u.entityType) }} › {{ u.property }}
                </RouterLink>
              </li>
            </ul>
          </div>
        </section>
        <ConfigPreview subject="Options as they show on records">
          <div class="preview-tags">
            <RlTag
              v-for="o in model.options"
              :key="o.value"
              :label="o.label"
              :color="tagColor(o.color)"
            />
          </div>
        </ConfigPreview>
      </template>
    </ConfigSplit>
  </ConfigurePage>
</template>

<style scoped>
.option-row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
}

.option-row__color {
  width: 96px;
  flex: none;
}

.option-row__title {
  flex: 1;
}

.option-row__title--removed {
  text-decoration: line-through;
  color: var(--rl-color-text-subtle);
}

.option-row__action {
  width: 56px;
  flex: none;
  display: flex;
  justify-content: flex-end;
}

.add-option {
  display: flex;
  align-items: flex-end;
  gap: var(--rl-space-3);
  max-width: 480px;
}

.settings {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.settings__para {
  margin: 4px 0 0;
}

.settings__list {
  margin: 4px 0 0;
  padding-left: var(--rl-space-4);
}

.preview-tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-2);
}
</style>
