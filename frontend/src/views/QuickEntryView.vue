<script setup lang="ts">
/**
 * Quick entry (TKT-24L1H36): the `quick_entry.form` create form, prefilled
 * with the `quick_entry.list` assign defaults. In the desktop popup
 * (`?popup=1`) a save closes the window; on the web it shows a toast and
 * opens a fresh form.
 */
import { computed, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import DynamicForm from '@/components/forms/DynamicForm.vue'
import { getAssign, type AssignValues } from '@/api/assign'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { closeQuickEntryPopup, leaveQuickEntry } from '@/utils/quickEntry'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
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
        // The form still opens, but the new item may not land in the list.
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
  <div class="quick-entry">
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
</style>
