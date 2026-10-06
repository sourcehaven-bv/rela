<script setup lang="ts">
import { computed, ref } from 'vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import type { SortKeyModel } from '@/configure/models'
import { newMap, type TreePath } from '@/configure/tree'

/**
 * A `sort:` list: records sort by the first key, then by the next. Each key
 * is edited in place, so settings a key holds besides these two stay.
 */
const props = withDefaults(
  defineProps<{
    file: 'schema' | 'screens'
    /** The mapping that holds the list. */
    path: TreePath
    keys: SortKeyModel[]
    /** The properties records can sort by. */
    properties: { value: string; label: string }[]
    listKey?: string
    label?: string
  }>(),
  { listKey: 'sort', label: 'Sort order' }
)
const draft = useConfigDraftStore()

const directions = [
  { value: 'asc', label: 'Ascending' },
  { value: 'desc', label: 'Descending' },
]
const keyPath = (index: number) => [...props.path, props.listKey, index]
const titleOf = (property: string) =>
  props.properties.find((p) => p.value === property)?.label ?? property

const unused = computed(() =>
  props.properties.filter((p) => !props.keys.some((k) => k.property === p.value))
)
const adding = ref('')

function add() {
  if (!adding.value) return
  draft.appendAt(props.file, props.path, props.listKey, newMap({ property: adding.value }))
  adding.value = ''
}
</script>

<template>
  <div class="sort-editor" data-testid="config-sort">
    <RlText size="sm" weight="medium">{{ label }}</RlText>
    <RlText v-if="!keys.length" size="sm" tone="muted">No set order.</RlText>
    <div v-for="(key, position) in keys" :key="key.index" class="sort-editor__row">
      <RlText size="sm" tone="subtle" class="sort-editor__then">{{
        position === 0 ? 'By' : 'Then by'
      }}</RlText>
      <RlSelect
        :model-value="key.property"
        :label="`Sort key ${position + 1}`"
        label-hidden
        size="sm"
        :options="[{ value: key.property, label: titleOf(key.property) }, ...unused]"
        @update:model-value="draft.setAt(file, keyPath(key.index), 'property', $event)"
      />
      <RlSelect
        :model-value="key.direction"
        :label="`Order of ${titleOf(key.property)}`"
        label-hidden
        size="sm"
        :options="directions"
        @update:model-value="
          draft.setAt(file, keyPath(key.index), 'direction', $event === 'asc' ? undefined : $event)
        "
      />
      <RlIconButton
        icon="arrow-up"
        :label="`Move ${titleOf(key.property)} up`"
        :disabled="position === 0"
        @click="draft.moveAt(file, [...path, listKey], key.index, key.index - 1)"
      />
      <RlIconButton
        icon="delete"
        :label="`Remove ${titleOf(key.property)} from the sort order`"
        @click="draft.removeItemAt(file, path, listKey, key.index)"
      />
    </div>
    <form v-if="unused.length" class="sort-editor__add" @submit.prevent="add">
      <RlSelect
        v-model="adding"
        :label="keys.length ? 'Then sort by' : 'Sort by'"
        label-hidden
        size="sm"
        :options="unused"
        placeholder="Choose a property"
      />
      <RlButton variant="secondary" size="sm" icon="plus" type="submit" :disabled="!adding"
        >Add sort key</RlButton
      >
    </form>
  </div>
</template>

<style scoped>
.sort-editor {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.sort-editor__row,
.sort-editor__add {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.sort-editor__then {
  width: 56px;
}
</style>
