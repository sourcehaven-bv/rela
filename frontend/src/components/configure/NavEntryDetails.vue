<script setup lang="ts">
import { computed } from 'vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import SortEditor from './SortEditor.vue'
import IconPicker from './IconPicker.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { propertiesOf, queryScopeNames, sortKeysOf, type NavEntryModel } from '@/configure/models'
import { get, getIn, mapItems, newMap, str } from '@/configure/tree'

/**
 * The settings of one sidebar entry beyond its name. A records entry
 * (`entities:`) lists the records of a type, filtered by a query scope and
 * sorted; a list entry may open as a flyout and flag a state of its list,
 * such as "3 overdue".
 */
const props = defineProps<{ entry: NavEntryModel }>()
const draft = useConfigDraftStore()

const node = computed(() => getIn(draft.currentDataEntry, props.entry.path))
const entityType = computed(() => (props.entry.kindKey === 'entities' ? props.entry.target : ''))

const scopeOptions = computed(() => [
  { value: '', label: "The type's default" },
  ...queryScopeNames(draft.currentSchema, entityType.value).map((n) => ({ value: n, label: n })),
])
const propertyOptions = computed(() =>
  propertiesOf(draft.currentSchema, entityType.value).map((p) => ({
    value: p.name,
    label: p.label,
  }))
)

const openOptions = [
  { value: '', label: 'As a page' },
  { value: 'flyout', label: 'In a panel beside the page' },
]
const tones = [
  { value: 'warning', label: 'Warning' },
  { value: 'error', label: 'Error' },
  { value: 'info', label: 'Information' },
  { value: 'new', label: 'New' },
  { value: 'success', label: 'Success' },
]
const rules = computed(() =>
  mapItems(get(node.value, 'status')).map((r, index) => ({
    index,
    tone: str(get(r, 'tone')) ?? '',
    label: str(get(r, 'label')) ?? '',
    condition: str(get(r, 'condition')) ?? '',
  }))
)
const rulePath = (index: number) => [...props.entry.path, 'status', index]

function set(key: string, value: string | undefined) {
  draft.setAt('screens', props.entry.path, key, value)
}

function addRule() {
  draft.appendAt(
    'screens',
    props.entry.path,
    'status',
    newMap({ tone: 'warning', label: '{count} need attention', condition: '' })
  )
}
</script>

<template>
  <div class="details" data-testid="config-nav-details">
    <IconPicker
      :model-value="str(get(node, 'icon')) ?? ''"
      default-label="The icon for the kind of entry"
      @update:model-value="set('icon', $event)"
    />

    <template v-if="entry.kindKey === 'entities'">
      <RlSelect
        :model-value="str(get(node, 'query_scope')) ?? ''"
        label="Which records"
        size="sm"
        :options="scopeOptions"
        hint="A query scope declared on the entity type."
        @update:model-value="set('query_scope', $event)"
      />
      <SortEditor
        file="screens"
        :path="entry.path"
        :keys="sortKeysOf(get(node, 'sort'))"
        :properties="propertyOptions"
        label="Order of the records"
      />
    </template>

    <template v-if="entry.kindKey === 'list'">
      <RlSelect
        :model-value="str(get(node, 'open')) ?? ''"
        label="Opens"
        size="sm"
        :options="openOptions"
        @update:model-value="set('open', $event)"
      />
      <div class="details__rules">
        <RlText size="sm" weight="medium">Status flags</RlText>
        <RlText size="sm" tone="muted"
          >The first flag whose condition matches a record of the list shows on the entry. {count}
          in the label is the number of records.</RlText
        >
        <div v-for="rule in rules" :key="rule.index" class="details__rule">
          <RlSelect
            :model-value="rule.tone"
            :label="`Tone of flag ${rule.index + 1}`"
            label-hidden
            size="sm"
            :options="tones"
            @update:model-value="draft.setAt('screens', rulePath(rule.index), 'tone', $event)"
          />
          <RlTextField
            :model-value="rule.label"
            :label="`Label of flag ${rule.index + 1}`"
            label-hidden
            size="sm"
            placeholder="{count} overdue"
            @update:model-value="draft.setAt('screens', rulePath(rule.index), 'label', $event)"
          />
          <RlTextarea
            :model-value="rule.condition"
            :label="`Condition of flag ${rule.index + 1}`"
            label-hidden
            :rows="1"
            placeholder="entity.due < today()"
            @update:model-value="draft.setAt('screens', rulePath(rule.index), 'condition', $event)"
          />
          <RlIconButton
            icon="delete"
            :label="`Remove flag ${rule.index + 1}`"
            @click="draft.removeItemAt('screens', entry.path, 'status', rule.index)"
          />
        </div>
        <RlButton variant="secondary" size="sm" icon="plus" @click="addRule">Add flag</RlButton>
      </div>
    </template>
  </div>
</template>

<style scoped>
.details {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
  padding: var(--rl-space-3);
  border-left: 2px solid var(--rl-color-border);
  margin-left: var(--rl-space-4);
}

.details__rules {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.details__rule {
  display: grid;
  grid-template-columns: 140px 1fr 1fr auto;
  gap: var(--rl-space-2);
  align-items: start;
}
</style>
