import { computed, ref, watch, type Ref } from 'vue'
import type { EntityType } from '@/types'

export type FormMode = 'create' | 'edit'

export interface EntityIDControls {
  prefixOptions: Ref<string[]>
  showManualIDInput: Ref<boolean>
  showPrefixPicker: Ref<boolean>
  manualId: Ref<string>
  selectedPrefix: Ref<string>
  reset: () => void
  buildPayloadFields: () => { id?: string; prefix?: string }
}

export function useEntityIDControls(
  entityType: Ref<EntityType | undefined>,
  mode: FormMode | Ref<FormMode>
): EntityIDControls {
  const manualId = ref('')
  const selectedPrefix = ref('')

  const modeRef = computed(() => (typeof mode === 'string' ? mode : mode.value))

  const prefixOptions = computed<string[]>(() => {
    const def = entityType.value
    if (!def) return []
    if (def.id_prefixes?.length) return def.id_prefixes
    if (def.id_prefix) return [def.id_prefix]
    return []
  })

  const isManual = computed(() => entityType.value?.id_type === 'manual')

  const showManualIDInput = computed(() => modeRef.value === 'create' && isManual.value)

  const showPrefixPicker = computed(
    () => modeRef.value === 'create' && !isManual.value && prefixOptions.value.length > 1
  )

  function reset() {
    manualId.value = ''
    selectedPrefix.value = prefixOptions.value[0] ?? ''
  }

  // Auto-reset when the entity type identity changes — DynamicForm and
  // InlineCreateModal can be reused across form-id navigations without
  // remounting, which would otherwise leave stale manualId/selectedPrefix
  // visible against the new type. See code-review #9 (RR-…).
  watch(
    () => entityType.value,
    (next, prev) => {
      if (next === prev) return
      if (!prev || !next || prev.id_type !== next.id_type) {
        reset()
        return
      }
      const prevPrefixes = (prev.id_prefixes ?? (prev.id_prefix ? [prev.id_prefix] : [])).join('|')
      const nextPrefixes = (next.id_prefixes ?? (next.id_prefix ? [next.id_prefix] : [])).join('|')
      if (prevPrefixes !== nextPrefixes) reset()
    }
  )

  function buildPayloadFields(): { id?: string; prefix?: string } {
    if (modeRef.value !== 'create') return {}
    const out: { id?: string; prefix?: string } = {}
    if (showManualIDInput.value && manualId.value) {
      out.id = manualId.value
    }
    if (showPrefixPicker.value && selectedPrefix.value) {
      out.prefix = selectedPrefix.value
    }
    return out
  }

  return {
    prefixOptions,
    showManualIDInput,
    showPrefixPicker,
    manualId,
    selectedPrefix,
    reset,
    buildPayloadFields,
  }
}
