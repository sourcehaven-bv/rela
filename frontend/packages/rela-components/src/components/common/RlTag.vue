<script setup lang="ts">
import type { TagColor } from '../../types'

withDefaults(defineProps<{ label: string; color?: TagColor }>(), { color: 'grey' })
</script>

<template>
  <span class="rl-tag" :class="`rl-tag--${color}`" :title="label">
    <span class="rl-tag__label">{{ label }}</span>
  </span>
</template>

<style scoped>
.rl-tag {
  display: inline-flex;
  align-items: center;
  padding: 1px var(--rl-space-2);
  border-radius: var(--rl-radius-sm);
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-medium);
  line-height: 18px;
  /*
   * A label is user-set and can be a sentence, so in a fixed-width cell the
   * tag gives way and ellipsizes rather than pushing its way out.
   *
   * The ellipsis has to live on an inner element. `text-overflow` only acts
   * on a block container with inline content, and this tag is a flex item,
   * which blockifies `inline-flex` to `flex` — so the same declaration on the
   * outer span hard-clips the text mid-word with no ellipsis to show for it.
   */
  min-width: 0;
  overflow: hidden;
}

.rl-tag__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-tag--grey   { background: var(--rl-tag-grey-bg);   color: var(--rl-tag-grey-fg); }
.rl-tag--blue   { background: var(--rl-tag-blue-bg);   color: var(--rl-tag-blue-fg); }
.rl-tag--green  { background: var(--rl-tag-green-bg);  color: var(--rl-tag-green-fg); }
.rl-tag--amber  { background: var(--rl-tag-amber-bg);  color: var(--rl-tag-amber-fg); }
.rl-tag--red    { background: var(--rl-tag-red-bg);    color: var(--rl-tag-red-fg); }
.rl-tag--purple { background: var(--rl-tag-purple-bg); color: var(--rl-tag-purple-fg); }
</style>
