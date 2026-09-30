<script setup lang="ts">
/**
 * Non-interactive block in a menu panel: who you are signed in as, or the
 * label over a group of rows.
 *
 * Exists because a plain div in the panel is almost right and never quite.
 * The rows are padded to line their text up under the panel's own padding, so
 * a caller's div either restates those declarations or sits a few pixels off
 * every label below it. That is worth one component rather than a note in
 * every consumer.
 *
 * ## Not a menu item
 *
 * There is no `role` here. `RlMenu` walks `[role="menuitem"]` for its arrow
 * keys, so an unroled block is skipped for free, and a screen reader reads
 * the text as the content it is. `role="presentation"` would be worse: it
 * strips the semantics of what is inside, which for a name and an email is
 * the only semantics there are.
 *
 * A heading level is deliberately not offered. A menu is not part of the
 * page outline, and a panel that only exists while it is open has no place in
 * it.
 */
withDefaults(
  defineProps<{
    /**
     * The first line, in the body colour: a person's name, or a group label.
     * Omit it and pass the whole block through the default slot instead.
     */
    label?: string
    /**
     * Quieter lines under the label, in order: an email, an organisation, a
     * role. An array rather than one string because they stack, and joining
     * them with a separator in the caller would read as one sentence to a
     * screen reader.
     */
    lines?: string[]
  }>(),
  { label: undefined, lines: () => [] },
)
</script>

<template>
  <div class="rl-menu-section">
    <slot>
      <span v-if="label" class="rl-menu-section__label">{{ label }}</span>
      <span v-for="line in lines" :key="line" class="rl-menu-section__line">{{ line }}</span>
    </slot>
  </div>
</template>

<style scoped>
.rl-menu-section {
  display: flex;
  flex-direction: column;
  gap: 2px;
  /*
   * The same padding a row carries, so the label's text starts on the same
   * vertical line as the labels below it. This is the whole reason the
   * component exists.
   */
  padding: var(--rl-space-2);
}

.rl-menu-section__label {
  font-size: var(--rl-font-size-md);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text);
}

.rl-menu-section__line {
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
  /*
   * An email is long and a panel is narrow, so it truncates rather than
   * widening the menu past the rows that matter.
   */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
