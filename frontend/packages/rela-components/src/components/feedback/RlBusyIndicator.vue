<script setup lang="ts">
/**
 * Shows that something works on an item now: three pulsing dots, the state
 * it works in and how long it has been there, such as "Planning · 3h".
 * With more than one entry it shows the first and "+N". The label is cut
 * off with an ellipsis when space runs out; the time and "+N" never are.
 *
 * Screen readers read one name that lists every entry: "Agent run:
 * Planning, for 3 hours". The dots, the icon and the visible text are
 * hidden from them, so the name is read once.
 *
 * It is not a live region. A board refreshes many cards at once, and an
 * announcement per card would drown the page. The app announces what it
 * needs to elsewhere.
 *
 * The time comes from `since` and a clock that moves on once a minute
 * (`useMinuteClock`), so it keeps counting while the indicator stays on
 * screen, with no new props. The clock's one timer is shared by every
 * indicator on the page.
 *
 * With a reduced-motion preference the dots are hidden and a static icon
 * shows in their place. A row of still dots would read as cut-off text.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import { busyDuration, useMessages } from '../../composables/useMessages'
import { useMinuteClock } from '../../composables/useMinuteClock'

export interface BusyIndicatorEntry {
  /** What is busy, such as a channel type label: `Agent run`. */
  typeLabel?: string
  /** The state it is busy in: `Planning`. */
  label: string
  /** When it entered the state, as an ISO 8601 string or a Date. */
  since?: string | Date
}

const props = defineProps<{ entries: BusyIndicatorEntry[] }>()

const messages = useMessages()
const now = useMinuteClock()

/** The duration since `since`, or undefined for a missing or bad time. */
function durationOf(since: string | Date | undefined) {
  if (since === undefined) return undefined
  const at = new Date(since).getTime()
  return Number.isNaN(at) ? undefined : busyDuration(now.value - at)
}

const first = computed(() => props.entries[0])

/** The visible text after the label: " · 3h +2", or "". */
const tail = computed(() =>
  first.value
    ? messages.busyTail({ duration: durationOf(first.value.since), more: props.entries.length - 1 })
    : '',
)

const name = computed(() =>
  messages.busyName({
    entries: props.entries.map((e) => ({ type: e.typeLabel, label: e.label, duration: durationOf(e.since) })),
  }),
)
</script>

<template>
  <span v-if="first" class="rl-busy">
    <span class="rl-busy__dots" aria-hidden="true">
      <span class="rl-busy__dot" />
      <span class="rl-busy__dot" />
      <span class="rl-busy__dot" />
    </span>
    <RlIcon class="rl-busy__icon" name="progress" :size="12" aria-hidden="true" />
    <span class="rl-busy__text" aria-hidden="true"
      ><span class="rl-busy__label">{{ first.label }}</span
      ><span class="rl-busy__tail">{{ tail }}</span></span
    >
    <span class="rl-visually-hidden">{{ name }}</span>
  </span>
</template>

<style scoped>
.rl-busy {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  max-width: 100%;
  min-width: 0;
  color: var(--rl-color-info-fg);
  font-size: var(--rl-font-size-xs);
  line-height: var(--rl-line-height-tight);
}

.rl-busy__dots {
  display: inline-flex;
  flex: none;
  gap: 2px;
}

.rl-busy__dot {
  width: 4px;
  height: 4px;
  border-radius: var(--rl-radius-pill);
  background: currentColor;
  animation: rl-busy-pulse 1.2s ease-in-out infinite;
}

.rl-busy__dot:nth-child(2) { animation-delay: 0.2s; }
.rl-busy__dot:nth-child(3) { animation-delay: 0.4s; }

.rl-busy .rl-busy__icon {
  display: none;
  flex: none;
}

.rl-busy__text {
  display: inline-flex;
  min-width: 0;
  white-space: pre;
}

.rl-busy__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rl-busy__tail {
  flex: none;
}

@keyframes rl-busy-pulse {
  0%, 100% { opacity: 0.3; }
  50% { opacity: 1; }
}

/* No motion: hide the dots and show the static icon in their place. */
@media (prefers-reduced-motion: reduce) {
  .rl-busy__dots {
    display: none;
  }

  .rl-busy__dot {
    animation: none;
  }

  .rl-busy .rl-busy__icon {
    display: inline-flex;
  }
}
</style>
