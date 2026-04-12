<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { RRule, Weekday } from 'rrule'

const props = defineProps<{
  modelValue: string
  label?: string
  help?: string
  readonly?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const frequencies = [
  { value: RRule.DAILY, label: 'Daily' },
  { value: RRule.WEEKLY, label: 'Weekly' },
  { value: RRule.MONTHLY, label: 'Monthly' },
  { value: RRule.YEARLY, label: 'Yearly' },
]

const weekdays = [
  { value: RRule.MO, label: 'Mon' },
  { value: RRule.TU, label: 'Tue' },
  { value: RRule.WE, label: 'Wed' },
  { value: RRule.TH, label: 'Thu' },
  { value: RRule.FR, label: 'Fri' },
  { value: RRule.SA, label: 'Sat' },
  { value: RRule.SU, label: 'Sun' },
]

const freq = ref(RRule.WEEKLY)
const interval = ref(1)
const selectedDays = ref<Weekday[]>([])
const dtstart = ref('')
const selectedMonth = ref<number | null>(null)
const selectedDay = ref<number | null>(null)

const months = [
  { value: 1, label: 'January' },
  { value: 2, label: 'February' },
  { value: 3, label: 'March' },
  { value: 4, label: 'April' },
  { value: 5, label: 'May' },
  { value: 6, label: 'June' },
  { value: 7, label: 'July' },
  { value: 8, label: 'August' },
  { value: 9, label: 'September' },
  { value: 10, label: 'October' },
  { value: 11, label: 'November' },
  { value: 12, label: 'December' },
]

const maxDay = computed(() => {
  if (!selectedMonth.value) return 31
  return new Date(2023, selectedMonth.value, 0).getDate()
})

watch(selectedMonth, () => {
  if (selectedDay.value && selectedDay.value > maxDay.value) {
    selectedDay.value = maxDay.value
  }
})

// Parse existing RRULE string on mount
function parseRrule(value: string) {
  if (!value) return

  try {
    // Handle both "FREQ=...", "RRULE:FREQ=...", and "DTSTART:...\nRRULE:FREQ=..." formats
    const normalized = value.includes('RRULE:')
      ? value.replace(/\s+/g, '\n') // ensure newlines between DTSTART and RRULE
      : `RRULE:${value}`
    const rule = RRule.fromString(normalized)
    const opts = rule.origOptions

    if (opts.freq !== undefined) freq.value = opts.freq
    if (opts.interval) interval.value = opts.interval
    if (opts.byweekday) {
      selectedDays.value = (Array.isArray(opts.byweekday) ? opts.byweekday : [opts.byweekday]).map(
        (d) => (d instanceof Weekday ? d : new Weekday(d as number)),
      )
    }
    if (opts.bymonth) {
      const m = Array.isArray(opts.bymonth) ? opts.bymonth[0] : opts.bymonth
      if (m) selectedMonth.value = m
    }
    if (opts.bymonthday) {
      const d = Array.isArray(opts.bymonthday) ? opts.bymonthday[0] : opts.bymonthday
      if (d) selectedDay.value = d
    }
    if (opts.dtstart) {
      const d = opts.dtstart
      const y = d.getUTCFullYear()
      const m = String(d.getUTCMonth() + 1).padStart(2, '0')
      const day = String(d.getUTCDate()).padStart(2, '0')
      dtstart.value = `${y}-${m}-${day}`
    }
  } catch {
    // If parsing fails, leave defaults
  }
}

onMounted(() => parseRrule(props.modelValue))

watch(
  () => props.modelValue,
  (val) => parseRrule(val),
)

// Build RRULE string from form state
const rruleString = computed(() => {
  const opts: Partial<ConstructorParameters<typeof RRule>[0]> = {
    freq: freq.value,
  }

  if (interval.value > 1) {
    opts.interval = interval.value
    if (dtstart.value) {
      const [y, m, d] = dtstart.value.split('-').map(Number)
      opts.dtstart = new Date(Date.UTC(y, m - 1, d))
    }
  }

  if (freq.value === RRule.WEEKLY && selectedDays.value.length > 0) {
    opts.byweekday = selectedDays.value
  }

  if (freq.value === RRule.YEARLY && selectedMonth.value && selectedDay.value) {
    opts.bymonth = [selectedMonth.value]
    opts.bymonthday = [selectedDay.value]
  }

  const rule = new RRule(opts)
  // Strip the RRULE: prefix from the RRULE part, keep DTSTART if present.
  // RRule.toString() produces "RRULE:FREQ=..." or "DTSTART:...\nRRULE:FREQ=..."
  return rule.toString().replace('RRULE:', '')
})

// Human-readable preview
const preview = computed(() => {
  try {
    const str = rruleString.value
    const normalized = str.includes('RRULE:')
      ? str.replace(/\s+/g, '\n')
      : str.includes('FREQ=')
        ? `RRULE:${str}`
        : str
    return RRule.fromString(normalized).toText()
  } catch {
    return ''
  }
})

// Emit changes
watch(rruleString, (val) => {
  emit('update:modelValue', val)
})

function toggleDay(day: Weekday) {
  const idx = selectedDays.value.findIndex((d) => d.weekday === day.weekday)
  if (idx >= 0) {
    selectedDays.value.splice(idx, 1)
  } else {
    selectedDays.value.push(day)
  }
  // Trigger reactivity
  selectedDays.value = [...selectedDays.value]
}

function isDaySelected(day: Weekday): boolean {
  return selectedDays.value.some((d) => d.weekday === day.weekday)
}
</script>

<template>
  <div class="rrule-builder">
    <label v-if="label" class="rrule-builder__label">{{ label }}</label>

    <div class="rrule-builder__row">
      <label class="rrule-builder__field-label">Every</label>
      <input
        v-model.number="interval"
        type="number"
        min="1"
        max="99"
        class="rrule-builder__interval"
        :disabled="readonly"
      />
      <select v-model="freq" class="rrule-builder__freq" :disabled="readonly">
        <option v-for="f in frequencies" :key="f.value" :value="f.value">
          {{ f.label }}
        </option>
      </select>
    </div>

    <div v-if="freq === RRule.WEEKLY" class="rrule-builder__weekdays">
      <button
        v-for="day in weekdays"
        :key="day.value.weekday"
        type="button"
        class="rrule-builder__day"
        :class="{ 'rrule-builder__day--selected': isDaySelected(day.value) }"
        :disabled="readonly"
        @click="toggleDay(day.value)"
      >
        {{ day.label }}
      </button>
    </div>

    <div v-if="freq === RRule.YEARLY" class="rrule-builder__yearly">
      <div class="rrule-builder__row">
        <label class="rrule-builder__field-label">On</label>
        <select
          :value="selectedMonth ?? ''"
          class="rrule-builder__month"
          :disabled="readonly"
          @change="selectedMonth = ($event.target as HTMLSelectElement).value ? Number(($event.target as HTMLSelectElement).value) : null"
        >
          <option value="">Month...</option>
          <option v-for="m in months" :key="m.value" :value="m.value">
            {{ m.label }}
          </option>
        </select>
        <input
          :value="selectedDay ?? ''"
          type="number"
          min="1"
          :max="maxDay"
          placeholder="Day"
          class="rrule-builder__day-input"
          :disabled="readonly"
          @input="selectedDay = (() => { const v = Number(($event.target as HTMLInputElement).value); return Number.isFinite(v) && v >= 1 ? Math.min(v, maxDay) : null })()"
        />
      </div>
    </div>

    <div v-if="interval > 1" class="rrule-builder__dtstart">
      <div class="rrule-builder__row">
        <label class="rrule-builder__field-label">Starting from</label>
        <input
          v-model="dtstart"
          type="date"
          class="rrule-builder__date"
          :disabled="readonly"
          required
        />
      </div>
      <p v-if="!dtstart" class="rrule-builder__warning">
        Required when interval &gt; 1
      </p>
    </div>

    <div v-if="preview" class="rrule-builder__preview">
      <span class="rrule-builder__preview-icon">&#x21bb;</span>
      {{ preview }}
    </div>

    <p v-if="help" class="rrule-builder__help">{{ help }}</p>
  </div>
</template>

<style scoped>
.rrule-builder {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
}

.rrule-builder__label {
  font-weight: 600;
  font-size: 14px;
  color: var(--text-color);
}

.rrule-builder__row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.rrule-builder__field-label {
  font-size: 13px;
  color: var(--muted-text);
  white-space: nowrap;
}

.rrule-builder__interval {
  width: 4rem;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.rrule-builder__freq {
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.rrule-builder__interval:focus,
.rrule-builder__freq:focus,
.rrule-builder__date:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.rrule-builder__weekdays {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.rrule-builder__day {
  width: 36px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--input-bg);
  color: var(--muted-text);
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: all 0.15s;
}

.rrule-builder__day:hover {
  border-color: var(--accent-color);
  color: var(--text-color);
}

.rrule-builder__day--selected {
  background: var(--accent-color);
  color: white;
  border-color: var(--accent-color);
}

.rrule-builder__day--selected:hover {
  opacity: 0.9;
}

.rrule-builder__yearly {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.rrule-builder__month {
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.rrule-builder__day-input {
  width: 5rem;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.rrule-builder__month:focus,
.rrule-builder__day-input:focus {
  outline: none;
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.rrule-builder__dtstart {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 6px;
}

.rrule-builder__dtstart .rrule-builder__row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.rrule-builder__date {
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
}

.rrule-builder__warning {
  color: #f59e0b;
  font-size: 12px;
  margin: 0;
}

.rrule-builder__preview {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: var(--input-bg);
  border: 1px solid var(--border-color);
  border-left: 3px solid var(--accent-color);
  border-radius: 0 6px 6px 0;
  font-size: 13px;
  color: var(--text-color);
}

.rrule-builder__preview-icon {
  color: var(--accent-color);
  font-size: 14px;
}

.rrule-builder__help {
  font-size: 12px;
  color: var(--muted-text);
  margin: 0;
}
</style>
