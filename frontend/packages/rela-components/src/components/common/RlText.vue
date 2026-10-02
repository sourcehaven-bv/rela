<script setup lang="ts">
/** Body and supporting text. Keeps size, tone and weight on the token scale. */
withDefaults(
  defineProps<{
    as?: 'span' | 'p' | 'div' | 'dd'
    size?: 'xs' | 'sm' | 'md' | 'lg'
    tone?: 'default' | 'muted' | 'subtle'
    weight?: 'normal' | 'medium' | 'semibold'
    truncate?: boolean
  }>(),
  { as: 'span', size: 'md', tone: 'default', weight: 'normal', truncate: false },
)
</script>

<template>
  <component
    :is="as"
    class="rl-text"
    :class="[
      `rl-text--${size}`,
      `rl-text--${tone}`,
      `rl-text--${weight}`,
      { 'rl-text--truncate': truncate },
    ]"
  >
    <slot />
  </component>
</template>

<style scoped>
.rl-text { margin: 0; }

.rl-text--xs { font-size: var(--rl-font-size-xs); }
.rl-text--sm { font-size: var(--rl-font-size-sm); }
.rl-text--md { font-size: var(--rl-font-size-md); }
.rl-text--lg { font-size: var(--rl-font-size-lg); }

.rl-text--default { color: var(--rl-color-text); }
.rl-text--muted { color: var(--rl-color-text-muted); }
.rl-text--subtle { color: var(--rl-color-text-subtle); }

.rl-text--normal { font-weight: var(--rl-font-weight-normal); }
.rl-text--medium { font-weight: var(--rl-font-weight-medium); }
.rl-text--semibold { font-weight: var(--rl-font-weight-semibold); }

.rl-text--truncate {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
</style>
