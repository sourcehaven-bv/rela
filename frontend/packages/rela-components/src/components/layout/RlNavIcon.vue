<script setup lang="ts">
/**
 * A sidebar item's glyph, or the space where one would be.
 *
 * Without this, a nav item with no icon has no box at all, so its label sits
 * further left than its icon-bearing siblings and a mixed menu reads as ragged.
 * Reserving the space keeps every label on one line.
 *
 * Three cases:
 *
 *   a name        render that icon
 *   `reserve`     render an empty box the same size, keeping labels aligned
 *   neither       render nothing, for a surface with no column to reserve,
 *                 such as a board column header
 *
 * Except when the sidebar is collapsed and labels are hidden. A row with
 * neither icon nor label is invisible but still clickable: nothing to click for
 * a sighted reader, while a keyboard or screen-reader user finds a normal item.
 * So a reserved slot shows its `fallback` while collapsed, because `reserve`
 * means "this row needs no glyph to be told apart from its labelled siblings",
 * and collapsing removes the labels that premise rests on.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'

const props = withDefaults(
  defineProps<{
    /** The icon to draw. Leave unset for an item that has none. */
    name?: IconName | null
    /**
     * Keep the icon's space when there is no name, so this item's label lines
     * up with its siblings'.
     */
    reserve?: boolean
    /** Whether the sidebar is collapsed to icons only. */
    collapsed?: boolean
    /** The glyph a reserved slot shows while collapsed, where no label helps. */
    fallback?: IconName | null
    size?: number
  }>(),
  { name: null, reserve: false, collapsed: false, fallback: null, size: 16 },
)

/** The name to draw, or null to draw no glyph at all. */
const glyph = computed<IconName | null>(() => {
  if (props.name) return props.name
  if (props.collapsed) return props.fallback
  return null
})
</script>

<template>
  <RlIcon v-if="glyph" :name="glyph" :size="size" />
  <!--
    A plain span, never an empty icon: an icon's width and height are
    presentation attributes that CSS overrides, so sizing one by rule stretches
    the glyph instead of the box. There is no glyph here to stretch.
  -->
  <span
    v-else-if="reserve"
    class="rl-nav-icon__spacer"
    :style="{ width: `${size}px`, height: `${size}px` }"
    aria-hidden="true"
  />
</template>

<style scoped>
.rl-nav-icon__spacer {
  display: block;
  flex: 0 0 auto;
}
</style>
