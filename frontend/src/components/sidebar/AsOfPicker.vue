<script setup lang="ts">
import { computed, ref } from 'vue'
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
 * backends); elsewhere the server would refuse every such read. The sidebar
 * also leaves it out where `builtins: {as_of: ...}` hides it.
 *
 * Without a date it shows a "Now" button over the input. An empty date input
 * shows the browser's own placeholder (`dd-mm-jjjj` in a Dutch browser), in
 * the browser's language rather than the app's, and says nothing about what
 * the control does.
 */
const { t } = useI18n()
const schemaStore = useSchemaStore()
const { asOf, setAsOf } = useAsOf()

const input = ref<HTMLInputElement | null>(null)
const value = computed(() => (asOf.value ? localDate(asOf.value) : ''))
// The future has no recorded state, so the picker stops at today.
const today = computed(() => localDate(new Date().toISOString()))

function onChange(event: Event) {
  const date = (event.target as HTMLInputElement).value
  setAsOf(date ? endOfDay(date) : undefined)
}

// Opens the browser's date picker. A browser without showPicker gets the
// input focused, where typing a date works.
function openPicker() {
  const el = input.value
  if (!el) return
  if (typeof el.showPicker === 'function') {
    try {
      el.showPicker()
      return
    } catch {
      // why: refused outside a user gesture; fall back to focus.
    }
  }
  el.focus()
}
</script>

<template>
  <div v-if="schemaStore.asOfEnabled" class="as-of-picker">
    <input
      ref="input"
      type="date"
      class="as-of-input"
      :class="{ 'as-of-input--empty': !value }"
      data-testid="as-of-picker"
      :aria-label="t('asOf.pickerAria')"
      :title="t('asOf.pickerTitle')"
      :tabindex="value ? undefined : -1"
      :aria-hidden="value ? undefined : 'true'"
      :value="value"
      :max="today"
      @change="onChange"
    />
    <button
      v-if="!value"
      type="button"
      class="as-of-now"
      data-testid="as-of-now"
      :aria-label="t('asOf.pickerAria')"
      :title="t('asOf.pickerTitle')"
      @click="openPicker"
    >
      {{ t('asOf.now') }}
    </button>
  </div>
</template>

<style scoped>
/*
 * A row of its own under the switchers. In the switchers' row the input took
 * the free width and squeezed the space name down to a few letters.
 */
.as-of-picker {
  position: relative;
  display: flex;
  flex: 1 0 100%;
  order: 1;
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

/* Under the Now button, which covers it. Kept rendered (not hidden), so the
   browser can open its picker from it and anchor the picker to it. */
.as-of-input--empty {
  opacity: 0;
}

.as-of-now {
  position: absolute;
  inset: 0;
  padding: 4px 6px;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-md);
  background: var(--rl-color-bg);
  color: var(--rl-color-text-muted);
  font: inherit;
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}

.as-of-now:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 2px;
}
</style>
