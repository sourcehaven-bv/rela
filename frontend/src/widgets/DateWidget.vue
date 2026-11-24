<script setup lang="ts">
// The input box comes from the library's global `.rl-control`
// (rl/styles/control.css, imported by styles/rl.css): border, radius, focus
// ring, and the invalid and disabled visuals. It is declared globally rather
// than per component precisely so a host rendering a native element can reach
// it, which is what this widget is. Its own styles only lay out the picker,
// the phrase box and the preview.
//
// The invalid visual is driven by `aria-invalid` there, so the old error class
// is gone: the attribute now carries both the appearance and the announcement,
// where the class did the first and nothing for the second.
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'
import { formatDate } from '@/utils/format'
import { packsFor, parseNaturalDate } from '@/utils/naturalDate'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const stringValue = useStringValue(() => props.modelValue)

// Display-mode rendering reuses the existing utils/format.ts helper so
// dates render consistently with how PropertyDisplay formats them today
// (RR-UD1A). Falls back to the raw string for un-parseable values.
//
// The fallback is deliberately SILENT (no console.warn): this computed
// runs on every reactive tick, so warning here would spam the console
// for any stale/in-progress date value. The raw-string passthrough is
// the right visible signal that something is off (RR-UD2J).
const displayValue = computed(() => {
  if (!stringValue.value) return ''
  return formatDate(stringValue.value) ?? stringValue.value
})

// `<input type="date">` accepts ONLY `YYYY-MM-DD` and silently renders blank
// for anything else — including the RFC3339 timestamps the API returns for
// `date` properties (`2026-09-15T00:00:00Z`). Binding the raw string therefore
// showed an empty input over a stored value, which reads as "my data is gone".
// Narrow to the date part; pass through anything already in the right shape,
// and leave un-parseable input alone so the user's in-progress typing is never
// swallowed.
const inputValue = computed(() => {
  const raw = stringValue.value
  if (!raw) return ''
  if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) return raw
  const match = /^(\d{4}-\d{2}-\d{2})T/.exec(raw)
  return match ? match[1] : raw
})

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}

// The phrase box beside the picker takes typed dates such as "tomorrow" in the
// UI language (utils/naturalDate). It shows the resolved date as the user
// types and applies it only on Enter or blur, so nothing is saved that the
// user has not seen.
const { t, locale } = useI18n()
const phrase = ref('')
const previewId = useId()
const resolve = () =>
  phrase.value.trim() === ''
    ? null
    : parseNaturalDate(phrase.value, new Date(), packsFor(locale.value))
const resolved = computed(resolve)
const unknown = computed(() => phrase.value.trim() !== '' && resolved.value === null)

// The preview names the weekday, so "next friday" typed on a Thursday visibly
// means tomorrow before the user applies it.
const preview = computed(() => {
  if (unknown.value) return t('widgets.date.unknown')
  if (resolved.value === null) return ''
  const [y, m, d] = resolved.value.split('-').map(Number)
  const date = new Date(y, m - 1, d).toLocaleDateString(locale.value, {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
  return t('widgets.date.resolved', { date })
})

function applyPhrase() {
  // Resolve again rather than reuse the cached value: a box left open past
  // midnight must resolve against the new day.
  const value = resolve()
  if (value === null) return
  emit('update:modelValue', value)
  phrase.value = ''
}

function onPhraseEnter(event: KeyboardEvent) {
  // Enter applies a phrase that resolves and must not also submit the form.
  // Otherwise Enter keeps its usual meaning.
  if (resolved.value === null) return
  event.preventDefault()
  applyPhrase()
}

function onPhraseEscape(event: KeyboardEvent) {
  // Clearing the box consumes Escape, so a surrounding dialog stays open.
  if (phrase.value === '') return
  event.stopPropagation()
  phrase.value = ''
}
</script>

<template>
  <span v-if="mode === 'display'" class="display-value">{{ displayValue }}</span>
  <span v-else class="date-entry">
    <input
      :id="id"
      type="date"
      class="rl-control"
      :aria-describedby="describedBy"
      :aria-invalid="invalid || undefined"
      :value="inputValue"
      :placeholder="placeholder"
      :disabled="disabled"
      @input="onInput"
    />
    <input
      v-model="phrase"
      type="text"
      class="rl-control date-phrase"
      data-testid="date-phrase"
      maxlength="64"
      autocomplete="off"
      :aria-label="t('widgets.date.phraseLabel')"
      :placeholder="t('widgets.date.phrasePlaceholder')"
      :aria-describedby="previewId"
      :aria-invalid="unknown || undefined"
      :disabled="disabled"
      @keydown.enter="onPhraseEnter"
      @keydown.esc="onPhraseEscape"
      @blur="applyPhrase"
    />
    <output :id="previewId" class="date-preview" data-testid="date-preview" aria-live="polite">{{
      preview
    }}</output>
  </span>
</template>

<style scoped>
.date-entry {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
}

.date-phrase {
  flex: 1 1 12rem;
  min-width: 0;
}

.date-preview {
  flex-basis: 100%;
  color: var(--rl-color-text-muted);
  font-size: 0.875em;
}

.date-preview:empty {
  display: none;
}
</style>
