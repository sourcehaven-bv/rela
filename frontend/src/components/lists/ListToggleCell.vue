<script setup lang="ts">
/**
 * A list cell that flips a property in place (TKT-M0AT73B). The flip is an
 * ordinary PATCH, so the server applies the field permission and the
 * transition guards; the row's `_actions` and `_fields` only decide whether
 * the control is offered. The box shows the new state while the request
 * runs, reverts on error, and the list then reads the page again.
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlSwitch from 'rela-components/components/form/RlSwitch.vue'
import { updateEntity } from '@/api/entities'
import { getErrorMessage } from '@/api'
import { useSchemaStore, useUIStore } from '@/stores'
import { entityRef } from '@/utils/entityRef'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { toggleCellState } from '@/utils/toggleCell'
import type { Entity, ListColumn } from '@/types'

const props = defineProps<{ entity: Entity; column: ListColumn; entityType: string }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const uiStore = useUIStore()
const schemaStore = useSchemaStore()

const moves = computed(() =>
  schemaStore.movesForProperty(props.column.property ?? '', props.entityType)
)
const state = computed(() => toggleCellState(props.entity, props.column, moves.value))
// The state shown while a PATCH runs; cleared when a new row arrives.
const pending = ref<boolean | null>(null)
watch(
  () => [props.entity, state.value.checked],
  () => (pending.value = null)
)
const checked = computed(() => pending.value ?? state.value.checked)
const label = computed(() =>
  t('list.toggle.label', {
    field: props.column.label ?? props.column.property ?? '',
    title: entityDisplayTitle(props.entity),
  })
)

async function flip() {
  if (state.value.disabled || pending.value !== null) return
  const prop = props.column.property
  if (!prop) return
  pending.value = !state.value.checked
  try {
    await updateEntity(props.entityType, entityRef(props.entity), {
      properties: { [prop]: state.value.target },
    })
    emit('changed')
  } catch (err) {
    pending.value = null
    uiStore.error(getErrorMessage(err))
  }
}
</script>

<template>
  <span v-if="!state.hidden" class="list-toggle-cell" @click.stop>
    <RlSwitch
      v-if="column.widget === 'switch'"
      :model-value="checked"
      :label="label"
      label-hidden
      size="sm"
      :disabled="state.disabled"
      :pending="pending !== null"
      @update:model-value="flip"
    />
    <RlCheckbox
      v-else
      :model-value="checked"
      :label="label"
      label-hidden
      :disabled="state.disabled"
      @update:model-value="flip"
    />
  </span>
</template>

<style scoped>
.list-toggle-cell {
  display: inline-flex;
  align-items: center;
}
</style>
