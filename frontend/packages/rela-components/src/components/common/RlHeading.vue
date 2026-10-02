<script setup lang="ts">
/**
 * Titles across the app. The semantic level and the visual size are separate
 * props so a heading can sit correctly in the document outline without being
 * forced to a particular size, which is what WCAG 1.3.1 asks for.
 */
withDefaults(
  defineProps<{
    /** Heading level in the document outline. */
    level?: 1 | 2 | 3 | 4 | 5 | 6
    /** Visual size. Defaults to the size that matches `level`. */
    size?: 'sm' | 'md' | 'lg' | 'xl' | '2xl'
    weight?: 'normal' | 'medium' | 'semibold'
    tone?: 'default' | 'muted'
    /** Tight suits display titles; normal suits headings inside running text. */
    lineHeight?: 'tight' | 'normal'
    /** Truncates to a single line with an ellipsis. */
    truncate?: boolean
  }>(),
  { level: 2, weight: 'semibold', tone: 'default', lineHeight: 'tight', truncate: false },
)

const sizeForLevel = { 1: 'xl', 2: 'lg', 3: 'md', 4: 'md', 5: 'sm', 6: 'sm' } as const
</script>

<template>
  <component
    :is="`h${level}`"
    class="rl-heading"
    :class="[
      `rl-heading--${size ?? sizeForLevel[level]}`,
      `rl-heading--${weight}`,
      `rl-heading--tone-${tone}`,
      `rl-heading--lh-${lineHeight}`,
      { 'rl-heading--truncate': truncate },
    ]"
  >
    <slot />
  </component>
</template>

<style scoped>
.rl-heading {
  margin: 0;
  overflow-wrap: break-word;
}

.rl-heading--lh-tight { line-height: var(--rl-line-height-tight); }
.rl-heading--lh-normal { line-height: var(--rl-line-height-normal); }

.rl-heading--sm { font-size: var(--rl-font-size-sm); }
.rl-heading--md { font-size: var(--rl-font-size-md); }
.rl-heading--lg { font-size: var(--rl-font-size-lg); }
.rl-heading--xl { font-size: var(--rl-font-size-xl); }
.rl-heading--2xl { font-size: var(--rl-font-size-2xl); }

.rl-heading--tone-default { color: var(--rl-color-text); }
.rl-heading--tone-muted { color: var(--rl-color-text-muted); }

.rl-heading--normal { font-weight: var(--rl-font-weight-normal); }
.rl-heading--medium { font-weight: var(--rl-font-weight-medium); }
.rl-heading--semibold { font-weight: var(--rl-font-weight-semibold); }

.rl-heading--truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
</style>
