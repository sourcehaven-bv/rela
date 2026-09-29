<script setup lang="ts">
/**
 * The bar that appears once rows are selected: how many, what can be done to
 * them, and a way out.
 *
 * `RlTable` already carries `selectedIds` and emits `toggle` and
 * `toggleAll`; this is the half that acts on them. It holds no selection of
 * its own and takes the count rather than the rows, because what the actions
 * need is the caller's business and passing the items would invite the bar
 * to filter them.
 *
 * Floats over the content rather than pushing it down. A bar that displaced
 * the list would move the rows a user is still ticking, and the one thing a
 * selection must not do is make the next row hard to hit.
 *
 * Announced politely rather than assertively: the count changes on every
 * tick, and a live region that interrupted on each one would make selecting
 * ten rows unbearable.
 */
import RlIconButton from '../common/RlIconButton.vue'
import { useMessages } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    /** How many rows are selected. Zero hides the bar. */
    count: number
    /**
     * What the count is counting, singular. Pluralised with a trailing `s`
     * unless `pluralLabel` says otherwise.
     */
    label?: string
    /** The plural, where adding `s` gets it wrong. */
    pluralLabel?: string
    /**
     * Whether to offer clearing the selection. On by default: a user who
     * selected by mistake needs a way back that is not unticking each row.
     */
    clearable?: boolean
  }>(),
  { label: 'item', pluralLabel: undefined, clearable: true },
)

const emit = defineEmits<{
  /** The selection should be emptied. */
  clear: []
}>()

/*
 * The tally is composed rather than a prop: the count is not known until
 * render, and the word order around it differs by language. `useMessages` is
 * the seam an app translates it through.
 */
const messages = useMessages()

const tally = () =>
  messages.selectedCount({
    count: props.count,
    noun: props.label,
    pluralNoun: props.pluralLabel,
  })
</script>

<template>
  <Transition name="rl-bulk-bar">
    <div v-if="count > 0" class="rl-bulk-bar" role="status" aria-live="polite">
      <span class="rl-bulk-bar__count">{{ tally() }}</span>

      <div class="rl-bulk-bar__actions">
        <slot />
      </div>

      <!--
        Last and visually separated, so a reach for the nearest control does
        not land on the one that discards the selection.
      -->
      <RlIconButton
        v-if="clearable"
        icon="x"
        label="Clear selection"
        class="rl-bulk-bar__clear"
        @click="emit('clear')"
      />
    </div>
  </Transition>
</template>

<style scoped>
.rl-bulk-bar {
  position: absolute;
  left: 50%;
  bottom: calc(var(--rl-space-6) + var(--rl-safe-inset-bottom));
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  max-width: calc(100% - 2 * var(--rl-space-5));
  padding: var(--rl-space-2) var(--rl-space-2) var(--rl-space-2) var(--rl-space-4);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg-raised);
  box-shadow: var(--rl-shadow-lg);
  /* Over the list it acts on, under anything the overlay stack owns. */
  z-index: var(--rl-z-sticky-raised);
}

.rl-bulk-bar__count {
  flex: none;
  font-size: var(--rl-font-size-sm);
  font-variant-numeric: tabular-nums;
  color: var(--rl-color-text-muted);
  white-space: nowrap;
}

.rl-bulk-bar__actions {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
  overflow-x: auto;
}

/*
 * A rule rather than a gap, because the clear is not one of the actions: it
 * undoes the state the actions apply to.
 */
.rl-bulk-bar__clear {
  flex: none;
  margin-left: var(--rl-space-1);
  border-left: 1px solid var(--rl-color-border);
  border-radius: 0 var(--rl-radius-md) var(--rl-radius-md) 0;
  padding-left: var(--rl-space-2);
}

/*
 * Rises into place, so the bar reads as arriving because of the tick rather
 * than as something that was always there.
 */
.rl-bulk-bar-enter-active,
.rl-bulk-bar-leave-active {
  transition: transform var(--rl-duration-base) ease-out, opacity var(--rl-duration-base) ease-out;
}

.rl-bulk-bar-enter-from,
.rl-bulk-bar-leave-to {
  transform: translate(-50%, calc(100% + var(--rl-space-6)));
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .rl-bulk-bar-enter-active,
  .rl-bulk-bar-leave-active {
    transition: opacity var(--rl-duration-fast) var(--rl-ease);
  }

  .rl-bulk-bar-enter-from,
  .rl-bulk-bar-leave-to {
    transform: translateX(-50%);
  }
}

/* On a phone it spans the width: a centred pill leaves too little for actions. */
@media (max-width: 767px) {
  .rl-bulk-bar {
    left: var(--rl-space-3);
    right: var(--rl-space-3);
    max-width: none;
    transform: none;
  }

  .rl-bulk-bar-enter-from,
  .rl-bulk-bar-leave-to {
    transform: translateY(calc(100% + var(--rl-space-6)));
  }

  @media (prefers-reduced-motion: reduce) {
    .rl-bulk-bar-enter-from,
    .rl-bulk-bar-leave-to { transform: none; }
  }
}
</style>
