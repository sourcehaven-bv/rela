<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import EntityPickerModal from '@/components/forms/EntityPickerModal.vue'
import { getEntity } from '@/api'
import { ApiError } from '@/api/errors'
import { isCancelledFetch } from '@/api/errors'
import { useWorld } from '@/composables/useWorld'
import { useSchemaStore } from '@/stores'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import type { Entity } from '@/types'

/**
 * The target of one relation default on the Settings page (TKT-WJP6PBM).
 *
 * The settings response names a relation's target types, not its targets,
 * so the page never lists a whole type. A new target is picked through
 * EntityPickerModal, which searches with a limit. The saved target's title
 * comes from one entity read. With several target types it tries first the
 * types whose id prefix the id carries, then the rest, and moves on only
 * after a 404. A target no type answers for (deleted, or not readable)
 * shows its id marked "not found"; any other failure shows it marked
 * "could not load". Either way it stays saved until the user changes or
 * clears it.
 */
/** The saved target id, or '' for none. */
const model = defineModel<string>({ required: true })

const props = defineProps<{
  /** The relation's declared target types. */
  types: readonly string[]
  /** The relation name, for the accessible names. */
  label: string
}>()

const { t } = useI18n()
const { worldParam } = useWorld()
const schemaStore = useSchemaStore()

const PICKER_LIMIT = 20

type TitleState = 'none' | 'loading' | 'shown' | 'missing' | 'failed'

const title = ref('')
const state = ref<TitleState>('none')
const pickerOpen = ref(false)
const valueId = `relation-default-value-${Math.random().toString(36).slice(2, 10)}`

// The entity the user just picked, so choosing it costs no read.
let picked: Entity | null = null
let abort: AbortController | null = null

function prefixesOf(type: string): string[] {
  const def = schemaStore.entityTypes.get(type)
  return [def?.id_prefix, ...(def?.id_prefixes ?? [])].filter((p): p is string => !!p)
}

// The types to try, the ones whose id prefix matches first.
function readOrder(id: string, types: readonly string[]): string[] {
  const matches = (type: string) => prefixesOf(type).some((p) => id.startsWith(p))
  return [...types.filter(matches), ...types.filter((ty) => !matches(ty))]
}

async function resolveTitle(id: string, types: readonly string[]): Promise<void> {
  abort?.abort()
  abort = null
  if (!id) {
    title.value = ''
    state.value = 'none'
    return
  }
  if (picked?.id === id) {
    title.value = entityDisplayTitle(picked)
    state.value = 'shown'
    return
  }
  // The id is not shown while the read runs, so a fast read does not flash it.
  state.value = 'loading'
  const ctrl = new AbortController()
  abort = ctrl
  for (const type of readOrder(id, types)) {
    try {
      const entity = await getEntity(type, id, { world: worldParam.value }, ctrl.signal)
      if (ctrl.signal.aborted) return
      title.value = entityDisplayTitle(entity)
      state.value = 'shown'
      return
    } catch (err) {
      if (isCancelledFetch(err) || ctrl.signal.aborted) return
      if (!(err instanceof ApiError && err.status === 404)) {
        title.value = id
        state.value = 'failed'
        return
      }
    }
  }
  title.value = id
  state.value = 'missing'
}

watch([model, () => props.types], ([id, types]) => void resolveTitle(id, types), {
  immediate: true,
})

onBeforeUnmount(() => abort?.abort())

function handleSelect(id: string, entity: Entity): void {
  picked = entity
  model.value = id
}
</script>

<template>
  <div class="relation-default-picker" data-testid="relation-default-picker">
    <span v-if="!model" :id="valueId" class="relation-default-none">
      {{ t('settings.relationDefault.none') }}
    </span>
    <span v-else :id="valueId" class="relation-default-value" :title="model">
      <span class="relation-default-title">{{ state === 'loading' ? '' : title }}</span>
      <span v-if="state === 'missing'" class="relation-default-mark">
        {{ t('settings.relationDefault.notFound') }}
      </span>
      <span v-else-if="state === 'failed'" class="relation-default-mark">
        {{ t('settings.relationDefault.loadFailed') }}
      </span>
    </span>
    <RlButton
      variant="secondary"
      size="sm"
      :disabled="types.length === 0"
      :aria-label="
        model
          ? t('settings.relationDefault.changeFor', { relation: label })
          : t('settings.relationDefault.chooseFor', { relation: label })
      "
      :aria-describedby="valueId"
      @click="pickerOpen = true"
    >
      {{ model ? t('settings.relationDefault.change') : t('settings.relationDefault.choose') }}
    </RlButton>
    <RlButton
      v-if="model"
      variant="ghost"
      size="sm"
      :aria-label="t('settings.relationDefault.clearFor', { relation: label })"
      :aria-describedby="valueId"
      @click="model = ''"
    >
      {{ t('settings.relationDefault.clear') }}
    </RlButton>
    <EntityPickerModal
      v-if="types.length > 0"
      :open="pickerOpen"
      :types="types"
      :max-results="PICKER_LIMIT"
      :label="t('settings.relationDefault.chooseFor', { relation: label })"
      @close="pickerOpen = false"
      @select="handleSelect"
    />
  </div>
</template>

<style scoped>
.relation-default-picker {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
}

.relation-default-value,
.relation-default-none {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.relation-default-none {
  color: var(--rl-color-text-muted);
}

.relation-default-mark {
  margin-left: var(--rl-space-2);
  font-size: 0.75rem;
  color: var(--rl-color-warning, inherit);
}
</style>
