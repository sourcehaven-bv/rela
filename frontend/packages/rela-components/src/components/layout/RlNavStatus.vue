<script setup lang="ts">
/**
 * The status marker at the end of a sidebar row: a glyph, a colour, and an
 * optional count.
 *
 * Takes a tone rather than an icon name, so every row flagged `error` looks
 * the same across a workspace. The caller says what is true and this decides
 * how it is drawn, which is the only way a sidebar stays readable once
 * several teams have added rows to it.
 *
 * The label is not decoration. An icon carries no text, and colour carries
 * none either, so the label is what a screen reader reads and what the
 * tooltip shows: without it a red triangle says only "something".
 */
import { computed } from 'vue'
import type { NavItemStatus } from '../../types'
import RlIcon from '../common/RlIcon.vue'

const props = defineProps<{ status: NavItemStatus }>()

/*
 * `new` is a filled dot rather than a glyph. It means "there is something
 * here" and nothing more precise, and a glyph would imply a kind of thing it
 * does not know. The others each have a shape the user already reads: a
 * triangle warns, a circled cross is an error.
 */
const ICON = {
  info: 'info',
  warning: 'warning',
  error: 'alert',
  success: 'done',
} as const

const icon = computed(() =>
  props.status.tone === 'new' ? null : ICON[props.status.tone],
)

/*
 * The count is worth showing only where it adds something to the label. A
 * zero is not a state, so it is treated as absent rather than drawn.
 */
const count = computed(() => {
  const value = props.status.count
  if (!value || value < 1) return null
  return value > 99 ? '99+' : String(value)
})
</script>

<template>
  <span
    class="rl-nav-status"
    :class="`rl-nav-status--${status.tone}`"
    :title="status.label"
  >
    <!--
      The label rides with the indicator rather than on the row, so a reader
      hears it as part of the row's name: "Deployments, sync failed".
    -->
    <span class="rl-visually-hidden">{{ status.label }}</span>

    <RlIcon v-if="icon" :name="icon" :size="14" class="rl-nav-status__icon" />

    <!--
      Always rendered, hidden alongside the glyph at full width. The rail
      swaps which of the two shows, and a dot that only existed for one tone
      would leave the others with nothing to fall back to.
    -->
    <span class="rl-nav-status__dot" :class="{ 'rl-nav-status__dot--fallback': icon }" />

    <span v-if="count" class="rl-nav-status__count" aria-hidden="true">{{ count }}</span>
  </span>
</template>

<style scoped>
/*
 * Sits on the row's text baseline rather than its centre. The count is set
 * smaller than the label, and a smaller face centred in the row rides about
 * a pixel above the label's baseline, which reads as misaligned. The count is
 * the only child aligned by baseline, so it supplies this box's baseline; the
 * dot and glyph stay centred on it.
 */
.rl-nav-status {
  display: flex;
  align-items: baseline;
  align-self: baseline;
  gap: var(--rl-space-1);
  flex: none;
  /* Holds its own colour so the icon and the count agree without repeating. */
  color: var(--rl-nav-status-color);
}

.rl-nav-status--new { --rl-nav-status-color: var(--rl-color-accent); }
.rl-nav-status--info { --rl-nav-status-color: var(--rl-color-status-blue); }
.rl-nav-status--warning { --rl-nav-status-color: var(--rl-color-status-amber); }
.rl-nav-status--error { --rl-nav-status-color: var(--rl-color-status-red); }
.rl-nav-status--success { --rl-nav-status-color: var(--rl-color-status-green); }

.rl-nav-status__icon { flex: none; align-self: center; }

.rl-nav-status__dot {
  width: 8px;
  height: 8px;
  flex: none;
  align-self: center;
  border-radius: var(--rl-radius-pill);
  background: currentColor;
}

/* The glyph is saying it at full width, so the dot waits for the rail. */
.rl-nav-status__dot--fallback { display: none; }

.rl-nav-status__count {
  font-size: var(--rl-font-size-xs);
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

/*
 * In the rail there is no room for a glyph beside a clipped label, so every
 * tone collapses to one dot in the icon's top corner: the row still says
 * "look here", which is the part that survives losing the words.
 *
 * A container query rather than a prop, because the sidebar has no collapsed
 * state of its own. The app rebinds `--rl-sidebar-width` and the rail is
 * simply a narrow sidebar, so the indicator has to notice the width itself.
 */
@container rl-sidebar (max-width: 120px) {
  .rl-nav-status {
    position: absolute;
    top: 2px;
    left: 20px;
    gap: 0;
  }

  .rl-nav-status__icon,
  .rl-nav-status__count { display: none; }

  .rl-nav-status__dot,
  .rl-nav-status__dot--fallback {
    display: block;
    width: 7px;
    height: 7px;
    /* Rings the dot in the sidebar's own background so it reads as on top. */
    box-shadow: 0 0 0 2px var(--rl-color-bg-sunken);
  }
}
</style>
