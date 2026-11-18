<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAsOf, endOfDay, localDate } from '@/composables/useAsOf'
import { useSchemaStore } from '@/stores/schema'

/**
 * Picks the date the app reads at, written to `?as_of=` (TKT-6KLUT3U).
 *
 * It sits beside the world switcher because the two answer one question
 * together: which state of the data is on screen. The world picks the face
 * of each entity; the date picks the moment. A picked date means the end of
 * that day in the reader's time zone, which is what "the state on
 * 31 December" means to an auditor. Clearing the date returns to the present.
 *
 * Rendered only when the storage keeps version history (the database
 * backends); elsewhere the server would refuse every such read.
 */
const { t } = useI18n()
const schemaStore = useSchemaStore()
const { asOf, setAsOf } = useAsOf()

const value = computed(() => (asOf.value ? localDate(asOf.value) : ''))
// The future has no recorded state, so the picker stops at today.
const today = computed(() => localDate(new Date().toISOString()))

function onChange(event: Event) {
  const date = (event.target as HTMLInputElement).value
  setAsOf(date ? endOfDay(date) : undefined)
}
</script>

<template>
  <div v-if="schemaStore.asOfEnabled" class="as-of-picker">
    <input
      type="date"
      class="as-of-input"
      data-testid="as-of-picker"
      :aria-label="t('asOf.pickerAria')"
      :title="t('asOf.pickerTitle')"
      :value="value"
      :max="today"
      @change="onChange"
    />
  </div>
</template>

<style scoped>
.as-of-picker {
  display: flex;
  min-width: 0;
}

.as-of-input {
  flex: 1;
  min-width: 0;
  padding: 4px 6px;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-md);
  background: var(--rl-color-bg);
  color: var(--rl-color-text);
  font-size: 12px;
}
</style>
