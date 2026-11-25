<script setup lang="ts">
/**
 * Quick entry (TKT-24L1H36): the `quick_entry.form` create form, prefilled
 * with the `quick_entry.list` assign defaults. In the desktop popup
 * (`?popup=1`) a save closes the window; on the web it shows a toast and
 * opens a fresh form.
 */
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import DynamicForm from '@/components/forms/DynamicForm.vue'
import { getAssign, type AssignValues } from '@/api/assign'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import {
  autofocusQuickEntry,
  closeQuickEntryPopup,
  fitQuickEntryPopup,
  leaveQuickEntry,
} from '@/utils/quickEntry'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const popup = computed(() => route.query.popup === '1')
const root = ref<HTMLElement | null>(null)
let stopFit = () => {}
let stopFocus = () => {}
onMounted(() => {
  if (!popup.value || !root.value) return
  stopFit = fitQuickEntryPopup(root.value)
  stopFocus = autofocusQuickEntry(root.value)
})
onBeforeUnmount(() => {
  stopFit()
  stopFocus()
})
const schemaStore = useSchemaStore()
const ui = useUIStore()

const cfg = computed(() => schemaStore.quickEntry)
const formKey = ref(0)
const assign = shallowRef<AssignValues>()
const ready = ref(false)

watch(
  () => cfg.value?.list,
  async (list) => {
    assign.value = undefined
    ready.value = false
    if (list) {
      try {
        assign.value = await getAssign({ list })
      } catch {
        // why: the form still opens; the new item may just miss the list.
        ui.warning(t('quickEntry.noDefaults'))
      }
    }
    ready.value = true
  },
  { immediate: true }
)

const prefill = computed(() => {
  const a = assign.value
  if (!a) return undefined
  const relations: Record<string, { id: string; type: string }[]> = {}
  for (const [rel, ops] of Object.entries(a.relations ?? {})) {
    relations[rel] = ops.data ?? ops.add ?? []
  }
  return { properties: { ...(a.properties ?? {}) }, relations }
})

function onCreated() {
  if (closeQuickEntryPopup(route)) return
  ui.success(t('quickEntry.saved'))
  formKey.value++
}

function onCancelled() {
  leaveQuickEntry(router, route)
}
</script>

<template>
  <div ref="root" class="quick-entry" :class="{ 'quick-entry--popup': popup }">
    <h1 class="quick-entry-title">{{ t('quickEntry.title') }}</h1>
    <p v-if="!cfg">{{ t('quickEntry.notConfigured') }}</p>
    <DynamicForm
      v-else-if="ready"
      :key="formKey"
      :form-id="cfg.form"
      embedded
      :embedded-prefill="prefill"
      @inline-created="onCreated"
      @inline-cancelled="onCancelled"
    />
  </div>
</template>

<style scoped>
.quick-entry {
  max-width: 760px;
  margin: 0 auto;
  padding: var(--space-md);
}
.quick-entry-title {
  font-size: var(--font-size-xl);
  margin: 0 0 var(--space-md);
}
/* Popup: a native-style dialog. The top padding keeps the title clear of the
   macOS traffic lights, which sit over a transparent title bar. */
.quick-entry--popup {
  max-width: none;
  margin: 0;
  padding: calc(28px + var(--space-sm)) var(--space-md) 0;
}
.quick-entry--popup .quick-entry-title {
  font-size: var(--font-size-lg);
  --wails-draggable: drag;
}
.quick-entry--popup :deep(.form-actions) {
  /* Stays in view when a tall form scrolls inside the capped window. */
  position: sticky;
  bottom: 0;
  z-index: 1;
  margin: 0 calc(0px - var(--space-md));
  padding: var(--space-md);
  background: var(--rl-color-bg);
  border-top: none;
  box-shadow: none;
  justify-content: flex-end;
}
.quick-entry--popup :deep(.form-actions .rl-button) {
  flex: 0 0 auto;
}
</style>
