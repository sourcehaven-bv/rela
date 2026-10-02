<script setup lang="ts">
/**
 * A keyboard key, or a combination of them.
 *
 * `keys` is split on the separator rather than the caller writing the glyphs,
 * so every shortcut in the product is punctuated the same way. The separator
 * carries meaning: `+` is held together, `then` is pressed in sequence.
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    /** One key, or several joined by the separator, such as `Ctrl+K`. */
    keys: string
    separator?: '+' | 'then' | 'or'
    size?: 'sm' | 'md'
  }>(),
  { separator: '+', size: 'md' },
)

const parts = computed(() => props.keys.split(props.separator).map((part) => part.trim()))

/*
 * The separator is read out, so "G then D" is announced as three tokens
 * rather than as the single unpronounceable string "GthenD".
 */
const spoken = computed(() => parts.value.join(` ${props.separator} `))
</script>

<template>
  <span class="rl-kbd" :class="`rl-kbd--${size}`" role="img" :aria-label="spoken">
    <template v-for="(part, index) in parts" :key="`${part}-${index}`">
      <span v-if="index > 0" class="rl-kbd__separator" aria-hidden="true">{{ separator }}</span>
      <kbd class="rl-kbd__key" aria-hidden="true">{{ part }}</kbd>
    </template>
  </span>
</template>

<style scoped>
.rl-kbd {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
}

.rl-kbd__key {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  padding: 1px var(--rl-space-1);
  border: 1px solid var(--rl-color-border-strong);
  /* The thicker bottom edge is what makes it read as a physical key. */
  border-bottom-width: 2px;
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg);
  color: var(--rl-color-text-muted);
  font-family: inherit;
  font-weight: var(--rl-font-weight-medium);
  line-height: var(--rl-line-height-tight);
}

.rl-kbd--sm .rl-kbd__key { font-size: 10px; }
.rl-kbd--md .rl-kbd__key { font-size: var(--rl-font-size-xs); }

.rl-kbd__separator {
  color: var(--rl-color-text-subtle);
  font-size: var(--rl-font-size-xs);
}
</style>
