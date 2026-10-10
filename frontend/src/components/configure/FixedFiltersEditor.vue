<script setup lang="ts">
import { ref } from 'vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import type { FixedFilterModel } from '@/configure/models'
import { newMap, type TreePath } from '@/configure/tree'

/**
 * The `filters:` of a list or board: conditions every record it shows must
 * meet. People cannot turn these off. `$USER` as a value means the person
 * looking at the list.
 */
const props = defineProps<{
  /** The mapping that holds `filters`. */
  path: TreePath
  filters: FixedFilterModel[]
  properties: { value: string; label: string }[]
}>()
const draft = useConfigDraftStore()
const KEY = 'filters'

const operators = [
  { value: '=', label: 'is' },
  { value: '!=', label: 'is not' },
  { value: '~', label: 'contains' },
  { value: 'in', label: 'is one of (comma-separated)' },
  { value: '<', label: 'is less than' },
  { value: '<=', label: 'is at most' },
  { value: '>', label: 'is more than' },
  { value: '>=', label: 'is at least' },
]
const titleOf = (property: string) =>
  props.properties.find((p) => p.value === property)?.label ?? property
const at = (index: number) => [...props.path, KEY, index]

const adding = ref('')
function add() {
  if (!adding.value) return
  draft.appendAt(
    'screens',
    props.path,
    KEY,
    newMap({ property: adding.value, operator: '=', value: '' })
  )
  adding.value = ''
}
</script>

<template>
  <div class="fixed-filters" data-testid="config-fixed-filters">
    <RlText v-if="!filters.length" size="sm" tone="muted">Shows every record.</RlText>
    <div v-for="filter in filters" :key="filter.index" class="fixed-filters__row">
      <RlSelect
        :model-value="filter.property"
        :label="`Property of filter ${filter.index + 1}`"
        label-hidden
        size="sm"
        :options="properties"
        @update:model-value="draft.setAt('screens', at(filter.index), 'property', $event)"
      />
      <RlSelect
        :model-value="filter.operator"
        :label="`Comparison for ${titleOf(filter.property)}`"
        label-hidden
        size="sm"
        :options="operators"
        @update:model-value="draft.setAt('screens', at(filter.index), 'operator', $event)"
      />
      <RlTextField
        :model-value="filter.value"
        :label="`Value for ${titleOf(filter.property)}`"
        label-hidden
        placeholder="Value"
        size="sm"
        @update:model-value="draft.setAt('screens', at(filter.index), 'value', $event)"
      />
      <RlIconButton
        icon="delete"
        :label="`Remove the ${titleOf(filter.property)} condition`"
        @click="draft.removeItemAt('screens', path, KEY, filter.index)"
      />
    </div>
    <form class="fixed-filters__row" @submit.prevent="add">
      <RlSelect
        v-model="adding"
        label="Add a condition"
        label-hidden
        size="sm"
        :options="properties"
        placeholder="Choose a property"
      />
      <RlButton variant="secondary" size="sm" icon="plus" type="submit" :disabled="!adding"
        >Add condition</RlButton
      >
    </form>
  </div>
</template>

<style scoped>
.fixed-filters {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.fixed-filters__row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}
</style>
