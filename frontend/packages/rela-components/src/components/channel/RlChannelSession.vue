<script setup lang="ts">
/**
 * The session of a channel, as a stepper: its states in declaration order
 * with the current one marked.
 *
 * The order is declaration order and nothing more. The stepper marks no
 * earlier state as done: in a branching flow a mark by position would claim
 * steps that never happened. Instead it marks the next states, where a
 * transition out of the current state leads. A state is never a button: the
 * composer is the only way to move a session.
 *
 * Exactly one status applies, in this order: stranded, loading, final,
 * paused, normal. A stranded session (its stored state is no longer
 * declared) has no current item and shows a warning. A loading one (the
 * app is still loading the states that include the current one) has no
 * current item and says so, with no warning. A final one shows its outcome:
 * a check and "Completed", "Cancelled", or a red "Failed" tag ("Ended"
 * when the app gives no status). A paused one (the channel is not open)
 * keeps its current item and says why it does not move. Next marks show
 * only in the normal status.
 *
 * In the normal status an `input_required` state shows an amber "Input
 * required" tag on the current item and, below the list, whom it waits
 * for, or a "Waiting for you" tag when it waits for the reader. The text
 * carries the meaning; the colour only repeats it. That line is a polite
 * live region, the one live part of the stepper: a session notice names no
 * waiting roles (it is history, and today's configuration cannot say whom
 * the session waited for then), so this line announces whose turn it is.
 *
 * Below 32rem of container width the list collapses to one summary line
 * and a toggle that shows it as a vertical list. Colour is never the only
 * signal: the current item has `aria-current="step"`, a next item has an
 * outline and hidden text, a final item an icon or hidden text.
 *
 * `compact` (data-entry.yaml `channels.<type>.session.display`) folds the
 * list at every width. At 32rem and wider the fold shows a short list in
 * place of the summary: the previous state (`session.previous`, the state
 * the session left), the current state and the next states, each with
 * hidden text that names its role. The toggle shows the full list and
 * removes the short one. While the short list is shown, only its current
 * item carries `aria-current`; once it is removed, the full list's does.
 * Without a current state (stranded or loading) the summary shows instead.
 * Below 32rem a compact stepper folds like any other.
 *
 * In the normal status a `working` state shows an activity line in the
 * same live region: "Working…" and a busy indicator with the state's
 * label. So the strip announces both whose turn it is and that someone is
 * at work, and it never holds a second live region.
 *
 * The stepper is not a live region: every move appends a notice to the log,
 * and the channel announces that. The `aside` slot holds what later belongs
 * next to the states, such as timers.
 *
 * The section takes focus (`tabindex="-1"`, with a visible focus style) so
 * the app can put focus here after a decision made from the header removes
 * the button that had it; `focus()` is exposed for that.
 */
import { computed, ref, useId, useTemplateRef } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlTag from '../common/RlTag.vue'
import RlBusyIndicator from '../feedback/RlBusyIndicator.vue'
import type { ChannelSessionView } from './types'
import { useMessages, type ChannelTextKey } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    session: ChannelSessionView
    /** Folds the list at every width, with the short list at 32rem and wider. */
    compact?: boolean
  }>(),
  { compact: false },
)

const strings = useMessages()
const t = (key: ChannelTextKey) => strings.channelText({ key })
const labelId = useId()
const listId = useId()
const expanded = ref(false)
const root = useTemplateRef<HTMLElement>('root')

defineExpose({
  /** Moves focus to the session header, without scrolling it into view. */
  focus: () => root.value?.focus({ preventScroll: true }),
})

type Status = 'stranded' | 'loading' | 'final' | 'paused' | 'normal'

const status = computed<Status>(() => {
  const s = props.session
  if (s.stranded) return 'stranded'
  if (s.loading) return 'loading'
  if (s.final) return 'final'
  if (s.paused) return 'paused'
  return 'normal'
})

const current = computed(() =>
  status.value === 'stranded' || status.value === 'loading' ? undefined : props.session.current,
)
const next = computed(() => new Set(status.value === 'normal' ? props.session.next : []))

/** The caption of a final state, from its status. */
const outcome = computed(() => {
  switch (props.session.status) {
    case 'cancelled':
      return t('sessionCancelled')
    case 'failed':
      return t('sessionFailed')
    case 'completed':
      return t('sessionCompleted')
    default:
      return t('sessionEnded')
  }
})

const failed = computed(() => props.session.status === 'failed')
const ticked = computed(() => !failed.value && props.session.status !== 'cancelled')
const inputRequired = computed(() => status.value === 'normal' && props.session.status === 'input_required')
const working = computed(() => status.value === 'normal' && props.session.status === 'working')
const waitingText = computed(() => {
  const labels = (props.session.waitingFor ?? []).map((w) => w.label)
  return labels.length ? strings.channelSessionWaiting({ labels }) : ''
})

const summary = computed(() =>
  strings.channelSessionSummary({
    status: status.value,
    label: props.session.currentLabel,
    ...(status.value === 'final' ? { outcome: outcome.value } : {}),
  }),
)

/** The hidden text of a state that is next or final, else undefined. */
function stepText(name: string, label: string, final?: boolean): string | undefined {
  const isNext = name !== current.value && next.value.has(name)
  return isNext || final ? strings.channelSessionStep({ label, next: isNext, final: !!final }) : undefined
}

/** A state of the short list of a compact stepper. */
interface ContextItem {
  name: string
  label: string
  role: 'previous' | 'current' | 'next'
}

/**
 * The short list: the previous state, when it is declared and not the
 * current one; the current state; the next states except the current one,
 * in declaration order. Next states show only in the normal status, as in
 * the full list.
 */
const contextItems = computed<ContextItem[]>(() => {
  const cur = current.value
  if (!props.compact || cur === undefined) return []
  const byName = new Map(props.session.states.map((s) => [s.name, s]))
  const items: ContextItem[] = []
  const prev = props.session.previous ? byName.get(props.session.previous) : undefined
  if (prev && prev.name !== cur) items.push({ name: prev.name, label: prev.label, role: 'previous' })
  items.push({ name: cur, label: props.session.currentLabel, role: 'current' })
  for (const s of props.session.states) {
    if (s.name !== cur && next.value.has(s.name)) items.push({ name: s.name, label: s.label, role: 'next' })
  }
  return items
})
const showContext = computed(() => contextItems.value.length > 0)
/**
 * Whether the short list is in the page. While it is, the full list's
 * current item drops `aria-current`, so the markup, not only CSS, has one
 * current item.
 */
const contextShown = computed(() => showContext.value && !expanded.value)

const pausedText = computed(() =>
  t(props.session.paused === 'archived' ? 'sessionPausedArchived' : 'sessionPausedClosed'),
)
</script>

<template>
  <section
    ref="root"
    class="rl-channel-session"
    :class="{
      'rl-channel-session--compact': compact,
      'rl-channel-session--no-context': compact && !showContext,
    }"
    tabindex="-1"
    :aria-labelledby="labelId"
  >
    <div class="rl-channel-session__inner">
      <div :id="labelId" class="rl-channel-session__label">
        <span class="rl-channel-session__kicker">{{ t('session') }}</span>
        <span class="rl-channel-session__name">{{ session.label }}</span>
      </div>

      <p class="rl-channel-session__summary">{{ summary }}</p>
      <ol
        v-if="contextShown"
        class="rl-channel-session__context"
        :aria-label="t('sessionContext')"
      >
        <li
          v-for="(item, i) in contextItems"
          :key="`${item.role}:${item.name}`"
          class="rl-channel-session__context-item"
          :data-role="item.role"
          :aria-current="item.role === 'current' ? 'step' : undefined"
        >
          <RlIcon
            v-if="i > 0 && item.role !== contextItems[i - 1].role"
            name="arrow-right"
            :size="12"
            class="rl-channel-session__arrow"
          />
          <span
            class="rl-channel-session__step"
            :class="{
              'rl-channel-session__step--current': item.role === 'current',
              'rl-channel-session__step--next': item.role === 'next',
            }"
          >
            <span class="rl-channel-session__bullet">
              <RlIcon v-if="item.role === 'current' && status === 'final' && ticked" name="check" :size="10" />
            </span>
            <span aria-hidden="true">{{ item.label }}</span>
            <span class="rl-visually-hidden">
              {{ strings.channelSessionContextItem({ role: item.role, label: item.label }) }}
            </span>
            <template v-if="item.role === 'current' && status === 'final'">
              <RlTag v-if="failed" :label="outcome" color="red" />
              <span v-else class="rl-channel-session__caption">{{ outcome }}</span>
            </template>
            <RlTag
              v-if="item.role === 'current' && inputRequired"
              :label="t('sessionInputRequired')"
              color="amber"
            />
          </span>
        </li>
      </ol>
      <button
        type="button"
        class="rl-channel-session__toggle"
        :aria-expanded="expanded"
        :aria-controls="listId"
        @click="expanded = !expanded"
      >
        <RlIcon :name="expanded ? 'chevron-up' : 'chevron-down'" :size="14" />
        {{ t(expanded ? 'sessionHideAll' : 'sessionShowAll') }}
      </button>

      <ol :id="listId" class="rl-channel-session__steps" :class="{ 'rl-channel-session__steps--expanded': expanded }">
        <li
          v-for="s in session.states"
          :key="s.name"
          class="rl-channel-session__step"
          :class="{
            'rl-channel-session__step--current': s.name === current,
            'rl-channel-session__step--next': s.name !== current && next.has(s.name),
          }"
          :aria-current="s.name === current && !contextShown ? 'step' : undefined"
        >
          <span class="rl-channel-session__bullet">
            <RlIcon v-if="s.name === current && status === 'final' && ticked" name="check" :size="10" />
          </span>
          <span :aria-hidden="stepText(s.name, s.label, s.final) ? 'true' : undefined">{{ s.label }}</span>
          <template v-if="s.name === current && status === 'final'">
            <RlTag v-if="failed" :label="outcome" color="red" />
            <span v-else class="rl-channel-session__caption">{{ outcome }}</span>
          </template>
          <RlTag v-if="s.name === current && inputRequired" :label="t('sessionInputRequired')" color="amber" />
          <span v-if="stepText(s.name, s.label, s.final)" class="rl-visually-hidden">
            {{ stepText(s.name, s.label, s.final) }}
          </span>
        </li>
      </ol>

      <div role="status" class="rl-channel-session__waiting">
        <template v-if="inputRequired">
          <RlTag v-if="session.waitingOnMe" :label="t('sessionWaitingForYou')" color="blue" />
          <span v-else-if="waitingText">{{ waitingText }}</span>
        </template>
        <span v-else-if="working" class="rl-channel-session__working">
          {{ t('sessionWorking') }}
          <RlBusyIndicator :entries="[{ label: session.currentLabel }]" />
        </span>
      </div>
      <span v-if="status === 'paused'" class="rl-channel-session__caption">{{ pausedText }}</span>
      <span v-if="status === 'loading'" class="rl-channel-session__caption">{{ t('sessionLoading') }}</span>
      <p v-if="status === 'stranded'" class="rl-channel-session__stranded">
        <RlTag :label="t('sessionStranded')" color="amber" />
        <span>{{ strings.channelSessionStrandedText({ state: session.currentLabel }) }}</span>
      </p>

      <div v-if="$slots.aside" class="rl-channel-session__aside">
        <slot name="aside" />
      </div>
      <div v-if="$slots.decisions" class="rl-channel-session__decisions">
        <slot name="decisions" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.rl-channel-session {
  container-type: inline-size;
  flex-shrink: 0;
  border-bottom: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg-sunken);
  color: var(--rl-color-text);
  font-size: var(--rl-font-size-sm);
}

.rl-channel-session:focus {
  outline: none;
}

.rl-channel-session:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-channel-session__inner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2) var(--rl-space-5);
  padding: var(--rl-space-3) var(--rl-space-5);
}

.rl-channel-session__label {
  display: flex;
  flex-direction: column;
  line-height: var(--rl-line-height-tight);
}

.rl-channel-session__kicker {
  color: var(--rl-color-text-subtle);
  font-size: var(--rl-font-size-xs);
}

.rl-channel-session__name {
  font-weight: var(--rl-font-weight-semibold);
}

.rl-channel-session__summary {
  display: none;
  flex: 1;
  min-width: 0;
  margin: 0;
  font-weight: var(--rl-font-weight-semibold);
}

.rl-channel-session__toggle {
  display: none;
  align-items: center;
  gap: var(--rl-space-1);
  padding: 2px var(--rl-space-2);
  border: 1px solid var(--rl-color-border-strong);
  border-radius: var(--rl-radius-md);
  background: transparent;
  color: var(--rl-color-text);
  font: inherit;
  font-size: var(--rl-font-size-xs);
  cursor: pointer;
}

.rl-channel-session__toggle:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: 1px;
}

/* A compact stepper folds the full list at every width; the short list
   stands in for it. */
.rl-channel-session--compact .rl-channel-session__toggle {
  display: inline-flex;
}

.rl-channel-session--compact .rl-channel-session__steps:not(.rl-channel-session__steps--expanded) {
  display: none;
}

.rl-channel-session--compact .rl-channel-session__steps--expanded {
  flex-basis: 100%;
}

/* Without a current state there is no short list, so the summary says
   where the session is. */
.rl-channel-session--no-context .rl-channel-session__summary {
  display: block;
}

.rl-channel-session__context {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-1);
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-channel-session__context-item {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
}

.rl-channel-session__arrow {
  color: var(--rl-color-text-subtle);
}

.rl-channel-session__steps {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-1);
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-channel-session__step {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 10px 2px 4px;
  border: 1px solid transparent;
  border-radius: var(--rl-radius-pill);
  color: var(--rl-color-text-muted);
}

.rl-channel-session__bullet {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border: 1.5px solid var(--rl-color-border-strong);
  border-radius: var(--rl-radius-pill);
}

.rl-channel-session__step--current {
  background: var(--rl-color-bg-selected);
  color: var(--rl-color-info-fg);
  font-weight: var(--rl-font-weight-semibold);
}

.rl-channel-session__step--current .rl-channel-session__bullet {
  border-color: var(--rl-color-accent);
}

.rl-channel-session__step--next {
  border: 1px dotted var(--rl-color-border-strong);
  color: var(--rl-color-text);
}

.rl-channel-session__caption {
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-normal);
}

/* An empty live region stays in the tree, so a screen reader hears the
   text it gains, but takes no room in the flex row. */
.rl-channel-session__waiting:empty {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
}

.rl-channel-session__waiting {
  margin: 0;
  color: var(--rl-color-text-muted);
}

.rl-channel-session__working {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-channel-session__stranded {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
  margin: 0;
  color: var(--rl-color-text-muted);
}

.rl-channel-session__aside {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-2);
  margin-left: auto;
}

.rl-channel-session__decisions {
  flex-basis: 100%;
  min-width: 0;
}

@container (max-width: 32rem) {
  .rl-channel-session__inner {
    gap: var(--rl-space-2) var(--rl-space-3);
  }

  .rl-channel-session__label {
    display: none;
  }

  .rl-channel-session__summary {
    display: block;
  }

  .rl-channel-session__toggle {
    display: inline-flex;
  }

  /* Below 32rem a compact stepper folds like any other: the summary, not
     the short list. */
  .rl-channel-session--compact .rl-channel-session__context {
    display: none;
  }

  .rl-channel-session__steps {
    display: none;
  }

  .rl-channel-session__steps--expanded {
    display: flex;
    flex-direction: column;
    flex-basis: 100%;
    align-items: flex-start;
  }

  .rl-channel-session__caption,
  .rl-channel-session__waiting,
  .rl-channel-session__stranded {
    flex-basis: 100%;
  }
}
</style>
