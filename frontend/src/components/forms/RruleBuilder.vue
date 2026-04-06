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

// Parse existing RRULE string on mount
function parseRrule(value: string) {
  if (!value) return

  try {
    const cleaned = value.replace(/^RRULE:/, '')
    const rule = RRule.fromString(`RRULE:${cleaned}`)
    const opts = rule.origOptions

    if (opts.freq !== undefined) freq.value = opts.freq
    if (opts.interval) interval.value = opts.interval
    if (opts.byweekday) {
      selectedDays.value = (Array.isArray(opts.byweekday) ? opts.byweekday : [opts.byweekday]).map(
        (d) => (d instanceof Weekday ? d : new Weekday(d as number)),
      )
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

  const rule = new RRule(opts)
  // Return without RRULE: prefix — the metamodel stores the raw string
  return rule.toString().replace(/^RRULE:/, '')
})

// Human-readable preview
const preview = computed(() => {
  try {
    const rule = RRule.fromString(`RRULE:${rruleString.value}`)
    return rule.toText()
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

    <div v-if="interval > 1" class="rrule-builder__dtstart">
      <label class="rrule-builder__field-label">Starting from</label>
      <input
        v-model="dtstart"
        type="date"
        class="rrule-builder__date"
        :disabled="readonly"
        required
      />
      <span v-if="interval > 1 && !dtstart" class="rrule-builder__warning">
        Required when interval &gt; 1
      </span>
    </div>

    <div v-if="preview" class="rrule-builder__preview">
      {{ preview }}
    </div>

    <p v-if="help" class="rrule-builder__help">{{ help }}</p>
  </div>
</template>

<style scoped>
.rrule-builder {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.rrule-builder__label {
  font-weight: 600;
  font-size: 0.875rem;
}

.rrule-builder__row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.rrule-builder__field-label {
  font-size: 0.875rem;
  color: var(--color-text-secondary, #666);
}

.rrule-builder__interval {
  width: 4rem;
  padding: 0.375rem 0.5rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: 4px;
  font-size: 0.875rem;
}

.rrule-builder__freq {
  padding: 0.375rem 0.5rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: 4px;
  font-size: 0.875rem;
}

.rrule-builder__weekdays {
  display: flex;
  gap: 0.25rem;
  flex-wrap: wrap;
}

.rrule-builder__day {
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: 4px;
  background: var(--color-bg, #fff);
  cursor: pointer;
  font-size: 0.8rem;
  transition: all 0.15s;
}

.rrule-builder__day:hover {
  border-color: var(--color-primary, #4a90d9);
}

.rrule-builder__day--selected {
  background: var(--color-primary, #4a90d9);
  color: white;
  border-color: var(--color-primary, #4a90d9);
}

.rrule-builder__dtstart {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.rrule-builder__date {
  padding: 0.375rem 0.5rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: 4px;
  font-size: 0.875rem;
}

.rrule-builder__warning {
  color: var(--color-warning, #e67e22);
  font-size: 0.75rem;
}

.rrule-builder__preview {
  padding: 0.5rem;
  background: var(--color-bg-secondary, #f8f9fa);
  border-radius: 4px;
  font-size: 0.875rem;
  color: var(--color-text-secondary, #666);
  font-style: italic;
}

.rrule-builder__help {
  font-size: 0.75rem;
  color: var(--color-text-muted, #999);
  margin: 0;
}
</style>
