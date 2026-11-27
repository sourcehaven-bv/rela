<script setup lang="ts">
// The input box comes from the library's global `.rl-control`
// (rl/styles/control.css, imported by styles/rl.css): border, radius, focus
// ring, and the invalid and disabled visuals. It is declared globally rather
// than per component precisely so a host rendering a native element can reach
// it, which is what this widget is -- so the only styles here lay out the
// picker, the phrase box beside it and the phrase message.
//
// The invalid visual is driven by `aria-invalid` there, so the old error class
// is gone: the attribute now carries both the appearance and the announcement,
// where the class did the first and nothing for the second.
import { computed, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WidgetProps } from './types'
import { useStringValue } from './useStringValue'
import { formatDate } from '@/utils/format'
import { MAX_PHRASE_LENGTH, parseNaturalDate } from '@/utils/naturalDate'

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
  discardPhrase()
}

/** A date picked by hand replaces whatever the phrase box said. */
function discardPhrase() {
  phrase.value = ''
  phraseInvalid.value = false
  shownFromPhrase = false
}

// The phrase box beside the picker (TKT-ICADEGW). A phrase that parses sets
// the value as it is typed, so the picker shows the reading at once and a
// click that ends an inline edit keeps it. Part of a word can parse on its
// own ("ma" is Monday on the way to "maart"), so when the text stops parsing
// the value the field had before the phrase is put back. The message waits
// for Enter or leaving the box, so it does not flash while a phrase is half
// typed.
const { t } = useI18n()
const phrase = ref('')
const phraseInvalid = ref(false)
const phraseId = useId()
const phraseErrorId = `${phraseId}-error`
// The value before the phrase changed it, and whether the field now shows a
// reading of the phrase rather than that value.
let beforePhrase: unknown
let shownFromPhrase = false

function onPhraseInput(event: Event) {
  phrase.value = (event.target as HTMLInputElement).value
  phraseInvalid.value = false
  const date = parseNaturalDate(phrase.value)
  if (date) {
    if (!shownFromPhrase) beforePhrase = props.modelValue
    shownFromPhrase = true
    emit('update:modelValue', date)
  } else if (shownFromPhrase) {
    shownFromPhrase = false
    emit('update:modelValue', beforePhrase)
  }
}

/** Leaving the box: clear a phrase that was understood, flag one that was not. Returns whether it was understood. */
function settlePhrase(): boolean {
  if (!phrase.value.trim()) return true
  if (parseNaturalDate(phrase.value)) {
    phrase.value = ''
    shownFromPhrase = false
    return true
  }
  phraseInvalid.value = true
  return false
}

/** Enter on a phrase that does not parse keeps an inline edit open, so the message can be read. */
function onPhraseEnter(event: KeyboardEvent) {
  if (!settlePhrase()) event.preventDefault()
}
</script>

<template>
  <span v-if="mode === 'display'" class="display-value">{{ displayValue }}</span>
  <span v-else class="date-widget">
    <!-- The picker stays the first input: inline edit focuses it and opens the picker. -->
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
      :id="phraseId"
      type="text"
      class="rl-control date-phrase"
      :aria-label="t('widgets.date.natural.label')"
      :aria-describedby="phraseInvalid ? phraseErrorId : undefined"
      :aria-invalid="phraseInvalid || undefined"
      :placeholder="t('widgets.date.natural.placeholder')"
      :maxlength="MAX_PHRASE_LENGTH"
      :value="phrase"
      :disabled="disabled"
      @input="onPhraseInput"
      @keydown.enter="onPhraseEnter"
      @change="settlePhrase"
    />
    <span :id="phraseErrorId" class="date-phrase-error" role="status">{{
      phraseInvalid ? t('widgets.date.natural.invalid') : ''
    }}</span>
  </span>
</template>

<style scoped>
.date-widget {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-xs);
}

.date-phrase {
  width: 12em;
}

.date-phrase-error {
  flex-basis: 100%;
  color: var(--rl-color-danger);
  font-size: var(--font-size-sm);
}

.date-phrase-error:empty {
  display: none;
}
</style>
