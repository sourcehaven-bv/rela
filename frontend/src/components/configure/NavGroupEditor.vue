<script setup lang="ts">
import { computed, ref } from 'vue'
import RlSortableList from 'rela-components/components/data/RlSortableList.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlNumberField from 'rela-components/components/form/RlNumberField.vue'
import RlSwitch from 'rela-components/components/form/RlSwitch.vue'
import RlSegmentedControl from 'rela-components/components/data/RlSegmentedControl.vue'
import NavEntryDetails from './NavEntryDetails.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { lists, type NavGroupModel } from '@/configure/models'
import { bool, get, getIn, isMap, newMap, num, remove, set, str } from '@/configure/tree'

/**
 * One group of a sidebar: its name and its entries in order. An entry that
 * only some people see, or that runs an action, is shown and kept in place
 * but not edited, because who may see it is set in the project's files.
 */
const props = defineProps<{ group: NavGroupModel }>()
const draft = useConfigDraftStore()

const rows = computed(() =>
  props.group.entries.map((e) => ({ ...e, id: e.path.join('.'), title: e.label }))
)

/** Which entries have their details open, by path. */
const open = ref(new Set<string>())
function toggle(id: string) {
  const next = new Set(open.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  open.value = next
}

/** The group mapping itself, for a real group. */
const groupPath = computed(() => props.group.itemsPath.slice(0, -1))
const fromPath = computed(() => [...groupPath.value, 'items_from'])
const itemsFrom = computed(() => getIn(draft.currentDataEntry, fromPath.value))
const listOptions = computed(() =>
  lists(draft.currentDataEntry).map((l) => ({ value: l.name, label: l.title }))
)
const fillOptions = [
  { value: 'items', label: 'Chosen entries' },
  { value: 'list', label: 'Rows of a list' },
]

/**
 * Switches between entries chosen by hand and entries generated from the
 * rows of a list. A group holds one or the other, never both.
 */
function setFill(mode: string) {
  draft.edit('screens', (tree) => {
    const g = getIn(tree, groupPath.value)
    if (!isMap(g)) return
    if (mode === 'list') {
      remove(g, 'items')
      set(g, 'items_from', newMap({ list: listOptions.value[0]?.value ?? '' }))
    } else {
      remove(g, 'items_from')
    }
  })
}

/** Entries of a loose run sit in the navigation list itself, after `offset`. */
function move(row: { index: number }, to: number) {
  draft.moveAt('screens', props.group.itemsPath, row.index, props.group.offset + to)
}
</script>

<template>
  <div class="group" data-testid="config-nav-group">
    <div class="group__head">
      <RlTextField
        v-if="group.index >= 0"
        :model-value="group.label"
        label="Group name"
        size="sm"
        :disabled="group.locked"
        @update:model-value="draft.setAt('screens', group.itemsPath.slice(0, -1), 'group', $event)"
      />
      <RlText v-else size="sm" tone="muted">Not in a group</RlText>
      <RlTag v-if="group.locked" label="Read only" />
      <RlSegmentedControl
        v-if="group.index >= 0 && !group.locked && (group.fromList || !group.entries.length)"
        :model-value="group.fromList ? 'list' : 'items'"
        label="Entries"
        :options="fillOptions"
        @update:model-value="setFill"
      />
    </div>
    <div v-if="group.fromList" class="from-list" data-testid="config-nav-items-from">
      <RlText size="sm" tone="muted"
        >One entry per row the list shows each person, in the list's order.</RlText
      >
      <RlSelect
        :model-value="str(get(itemsFrom, 'list')) ?? ''"
        label="List"
        size="sm"
        :options="listOptions"
        :disabled="group.locked"
        @update:model-value="draft.setAt('screens', fromPath, 'list', $event)"
      />
      <RlNumberField
        :model-value="num(get(itemsFrom, 'limit'))"
        label="At most"
        size="sm"
        :min="1"
        :disabled="group.locked"
        @update:model-value="draft.setAt('screens', fromPath, 'limit', $event)"
      />
      <RlSwitch
        :model-value="bool(get(itemsFrom, 'create'))"
        label="Offer an add button on the group"
        :disabled="group.locked"
        @update:model-value="draft.setAt('screens', fromPath, 'create', $event)"
      />
    </div>
    <RlSortableList
      v-else
      :items="rows"
      :label="`Entries of ${group.label || 'the sidebar'}`"
      :disabled="group.locked"
      @move="move"
    >
      <template #item="{ item }">
        <div class="entry">
          <RlTextField
            :model-value="item.label"
            :label="`Name of ${item.title}`"
            label-hidden
            size="sm"
            class="entry__label"
            :disabled="item.locked || group.locked"
            @update:model-value="draft.setAt('screens', item.path, 'label', $event)"
          />
          <RlText size="sm" tone="subtle" class="entry__kind"
            >{{ item.kind }}{{ item.target ? `: ${item.target}` : '' }}</RlText
          >
          <RlTag v-if="item.locked" label="Read only" />
          <template v-else-if="!group.locked">
            <RlIconButton
              icon="settings"
              :label="`Settings of ${item.title}`"
              :aria-expanded="open.has(item.id)"
              @click="toggle(item.id)"
            />
            <RlIconButton
              icon="delete"
              :label="`Remove ${item.title}`"
              @click="draft.removeAt('screens', item.path.slice(0, -1), item.index)"
            />
          </template>
        </div>
        <NavEntryDetails v-if="open.has(item.id) && !item.locked" :entry="item" />
      </template>
    </RlSortableList>
  </div>
</template>

<style scoped>
.group {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.group__head {
  display: flex;
  align-items: flex-end;
  gap: var(--rl-space-2);
  max-width: 400px;
}

.entry {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
}

.entry__label {
  flex: 1;
}

.from-list {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--rl-space-3);
  max-width: 560px;
}

.from-list > :first-child {
  grid-column: 1 / -1;
}

.entry__kind {
  width: 200px;
}
</style>
