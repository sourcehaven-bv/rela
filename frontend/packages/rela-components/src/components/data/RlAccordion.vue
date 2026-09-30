<script setup lang="ts">
/**
 * Collapsible section with a heading row.
 *
 * Built on `details`/`summary`, so it opens and closes without JavaScript, is
 * keyboard operable for free, and stays findable by the browser's in-page
 * search even while collapsed.
 *
 * The heading level is a prop because an accordion's position in the document
 * outline depends on where it is used, and a fixed level would break the
 * heading order on half the pages it appears on.
 */
import RlIcon from '../common/RlIcon.vue'
import RlCount from '../common/RlCount.vue'

withDefaults(
  defineProps<{
    title: string
    /** Starts expanded. Not two-way: `details` owns its own state. */
    open?: boolean
    /** Shown after the title, such as the number of rows inside. */
    count?: number
    /** Secondary text on the right of the row, such as a status summary. */
    meta?: string
    level?: 2 | 3 | 4
    disabled?: boolean
  }>(),
  { open: false, level: 3, disabled: false },
)

defineEmits<{ toggle: [open: boolean] }>()
</script>

<template>
  <details
    class="rl-accordion"
    :open="open"
    @toggle="$emit('toggle', ($event.target as HTMLDetailsElement).open)"
  >
    <summary class="rl-accordion__summary" :class="{ 'rl-accordion__summary--disabled': disabled }">
      <RlIcon name="chevron-right" :size="16" class="rl-accordion__twisty" aria-hidden="true" />

      <!--
        The heading is inside the summary rather than around it: a summary is
        already the button, and wrapping it in a heading would leave the
        heading itself unreachable.
      -->
      <component :is="`h${level}`" class="rl-accordion__title">{{ title }}</component>

      <RlCount v-if="count !== undefined" :value="count" />
      <span v-if="meta" class="rl-accordion__meta">{{ meta }}</span>
      <span v-if="$slots.summaryExtra" class="rl-accordion__extra"><slot name="summaryExtra" /></span>
    </summary>

    <div class="rl-accordion__body"><slot /></div>
  </details>
</template>

<style scoped>
.rl-accordion {
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-accordion__summary {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-3) var(--rl-space-2);
  cursor: pointer;
  /* The default triangle is replaced by the chevron. */
  list-style: none;
}
.rl-accordion__summary::-webkit-details-marker { display: none; }

.rl-accordion__summary:hover { background: var(--rl-color-bg-hover); }

.rl-accordion__summary:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-accordion__summary--disabled { pointer-events: none; opacity: 0.5; }

.rl-accordion__twisty {
  flex: none;
  color: var(--rl-color-text-muted);
  transition: transform var(--rl-duration-fast) var(--rl-ease);
}
.rl-accordion[open] .rl-accordion__twisty { transform: rotate(90deg); }

.rl-accordion__title {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: var(--rl-font-size-md);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text);
}

.rl-accordion__meta,
.rl-accordion__extra {
  flex: none;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-muted);
}

.rl-accordion__body { padding: 0 var(--rl-space-2) var(--rl-space-4); }

@media (prefers-reduced-motion: reduce) {
  .rl-accordion__twisty { transition: none; }
}
</style>
