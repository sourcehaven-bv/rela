<script setup lang="ts">
import { computed, ref } from 'vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import type { FilterControlModel } from '@/configure/models'
import { ensureList, ensurePath, isMap, newMap, setOrRemove, type TreePath } from '@/configure/tree'

/**
 * The `filter_controls:` of a list or board: the filters people can set
 * above it. A control filters on a property, or on the records at the other
 * end of a relation. Controls are edited in place, so their other settings
 * stay.
 */
const props = defineProps<{
  /** The mapping that holds `filter_controls`. */
  path: TreePath
  controls: FilterControlModel[]
  properties: { value: string; label: string }[]
  relations: { name: string; label: string; direction: 'outgoing' | 'incoming' }[]
}>()
const draft = useConfigDraftStore()
const KEY = 'filter_controls'

/** Choices are `p:<property>` or `r:<direction>:<relation>`. */
function choiceOf(c: Pick<FilterControlModel, 'property' | 'relation' | 'direction'>): string {
  return c.relation ? `r:${c.direction}:${c.relation}` : `p:${c.property}`
}

const choices = computed(() => [
  ...props.properties.map((p) => ({ value: `p:${p.value}`, label: p.label })),
  ...props.relations.map((r) => ({
    value: `r:${r.direction}:${r.name}`,
    label: r.direction === 'incoming' ? `${r.label} (incoming)` : r.label,
  })),
])
const titleOf = (c: FilterControlModel) =>
  c.label || choices.value.find((o) => o.value === choiceOf(c))?.label || c.property || c.relation
const unused = computed(() =>
  choices.value.filter((o) => !props.controls.some((c) => choiceOf(c) === o.value))
)

/** Points control `index` (or a new one) at a choice, keeping its other keys. */
function apply(index: number | undefined, choice: string) {
  const [kind, a, b] = choice.split(':')
  draft.edit('screens', (tree) => {
    const owner = ensurePath(tree, props.path)
    if (!owner) return
    const list = ensureList(owner, KEY)
    const item = index === undefined ? newMap() : list[index]
    if (!isMap(item)) return
    setOrRemove(item, 'property', kind === 'p' ? a : undefined)
    setOrRemove(item, 'relation', kind === 'r' ? b : undefined)
    setOrRemove(item, 'direction', kind === 'r' && a === 'incoming' ? 'incoming' : undefined)
    if (index === undefined) list.push(item)
  })
}

const adding = ref('')
function add() {
  if (!adding.value) return
  apply(undefined, adding.value)
  adding.value = ''
}
</script>

<template>
  <div class="filter-controls" data-testid="config-filter-controls">
    <RlText v-if="!controls.length" size="sm" tone="muted">People cannot filter this yet.</RlText>
    <div v-for="control in controls" :key="control.index" class="filter-controls__row">
      <RlSelect
        :model-value="choiceOf(control)"
        :label="`Filter on, for ${titleOf(control)}`"
        label-hidden
        size="sm"
        :options="[
          choices.find((o) => o.value === choiceOf(control)) ?? {
            value: choiceOf(control),
            label: control.property || control.relation,
          },
          ...unused,
        ]"
        @update:model-value="apply(control.index, $event)"
      />
      <RlTextField
        :model-value="control.label"
        :label="`Label of the ${titleOf(control)} filter`"
        label-hidden
        placeholder="Label"
        size="sm"
        @update:model-value="draft.setAt('screens', [...path, KEY, control.index], 'label', $event)"
      />
      <RlIconButton
        icon="delete"
        :label="`Remove the ${titleOf(control)} filter`"
        @click="draft.removeItemAt('screens', path, KEY, control.index)"
      />
    </div>
    <form v-if="unused.length" class="filter-controls__row" @submit.prevent="add">
      <RlSelect
        v-model="adding"
        label="Add a filter"
        label-hidden
        size="sm"
        :options="unused"
        placeholder="Choose a property or relation"
      />
      <RlButton variant="secondary" size="sm" icon="plus" type="submit" :disabled="!adding"
        >Add filter</RlButton
      >
    </form>
  </div>
</template>

<style scoped>
.filter-controls {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.filter-controls__row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}
</style>
