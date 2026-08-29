<script setup lang="ts">
/**
 * NavIcon renders a sidebar item's glyph, or the space where one would be.
 *
 * The sidebar has four near-identical item render sites (link/button × grouped/
 * ungrouped). Putting the three-way icon decision here rather than inline means
 * it is made once; four copies of a conditional is how the surfaces drifted
 * apart the last time.
 *
 * The three cases:
 *
 *   name is a known icon   → render it
 *   name is NO_ICON        → render an empty box of the same size, so the label
 *                            stays aligned with its icon-bearing siblings
 *                            instead of jumping left in a mixed menu
 *   name is absent/unknown → resolveIcon's fallback (unchanged behaviour)
 *
 * Except when the sidebar is COLLAPSED, where labels are hidden. A row with
 * neither icon nor label is an invisible-but-clickable target: a sighted user
 * sees nothing to click while a keyboard or screen-reader user finds a normal
 * item. So a NO_ICON entry shows its derived glyph again while collapsed —
 * `none` means "this row needs no glyph to be told apart from its labelled
 * siblings", and collapsing removes the labels that premise depends on.
 */
import { computed } from 'vue'
import { hasIcon, resolveIcon, NO_ICON } from '@/utils/icons'

const props = withDefaults(
  defineProps<{
    /** The icon name from the sidebar payload. */
    name?: string | null
    /** Whether the sidebar is collapsed to icons only. */
    collapsed?: boolean
    /** The glyph to show for a NO_ICON entry while collapsed. Falls back to the
     * generic document icon when the item's kind derived nothing. */
    fallback?: string | null
  }>(),
  { name: null, collapsed: false, fallback: null },
)

/** showGlyph is false only for a deliberate NO_ICON in an expanded sidebar. */
const showGlyph = computed(() => hasIcon(props.name) || props.collapsed)

/** Collapsed NO_ICON rows borrow the kind-derived glyph; everything else uses
 * the name as given. */
const glyph = computed(() =>
  resolveIcon(props.name === NO_ICON ? props.fallback : props.name),
)
</script>

<template>
  <component
    :is="glyph"
    v-if="showGlyph"
    class="nav-icon"
    :size="18"
    aria-hidden="true"
  />
  <span v-else class="nav-icon-spacer" aria-hidden="true" />
</template>

<style scoped>
/* Reserves exactly the box .nav-icon occupies (18px glyph + 18px gutter).
 *
 * A plain <span>, never an SVG: Lucide emits width/height as PRESENTATION
 * attributes, and CSS beats those, so a `width` rule on an icon silently
 * stretches it (that bug shipped once — every sidebar icon rendered 24x18).
 * Nothing here can hit that, because there is no SVG to override. */
.nav-icon-spacer {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  margin-right: 18px;
}
</style>
