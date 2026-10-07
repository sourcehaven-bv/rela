<script setup lang="ts" generic="T extends RelatedItem">
/**
 * One linked record: an optional state glyph, the title, and a few values at
 * the end. The whole row is the target, and the title is what receives focus.
 *
 * The title is a button by default and emits `select`. A row that navigates
 * should set `item.as` to a link instead, so it gets the middle click and the
 * status bar a link has.
 */
import { computed } from 'vue'
import type { RelatedItem } from './types'

const props = defineProps<{ item: T }>()
const emit = defineEmits<{ select: [item: T] }>()

const tag = computed(() => props.item.as ?? 'button')

// The hidden labels end in a space inside the interpolation: the template
// compiler trims a trailing literal space before a closing tag, and a screen
// reader would then read "DueAug 23" as one word.
</script>

<template>
  <div class="rl-related-row">
    <span
      v-if="item.icon"
      class="rl-related-row__icon"
      :class="`rl-related-row--${item.iconTone ?? 'subtle'}`"
    >
      <component :is="item.icon" :size="15" aria-hidden="true" />
    </span>
    <component
      :is="tag"
      :type="tag === 'button' ? 'button' : undefined"
      class="rl-related-row__title"
      v-bind="item.attrs"
      @click="emit('select', item)"
    >
      <span v-if="item.iconLabel" class="rl-visually-hidden">{{ `${item.iconLabel}: ` }}</span>{{ item.title }}
    </component>
    <span v-if="item.meta?.length" class="rl-related-row__meta">
      <span v-for="m in item.meta" :key="m.id" :class="`rl-related-row--${m.tone ?? 'subtle'}`">
        <span v-if="m.label" class="rl-visually-hidden">{{ `${m.label} ` }}</span>{{ m.value }}
      </span>
    </span>
  </div>
</template>

<style scoped>
.rl-related-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-3) var(--rl-space-2);
  border-bottom: 1px solid var(--rl-color-border);
}
.rl-related-row:hover { background: var(--rl-color-bg-sunken); }

.rl-related-row:has(.rl-related-row__title:focus-visible) {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-related-row__icon {
  display: inline-flex;
  flex: none;
}

.rl-related-row__title {
  flex: 1;
  min-width: 0;
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: var(--rl-color-text);
  text-align: left;
  text-decoration: none;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Stretches the title's hit area over the whole row. */
.rl-related-row__title::after {
  content: '';
  position: absolute;
  inset: 0;
}

.rl-related-row__title:focus-visible { outline: none; }

.rl-related-row__meta {
  display: flex;
  flex: none;
  align-items: center;
  gap: var(--rl-space-5);
  margin-left: var(--rl-space-2);
  font-size: var(--rl-font-size-md);
}

.rl-related-row--subtle { color: var(--rl-color-text-subtle); }
.rl-related-row--muted { color: var(--rl-color-text-muted); }
.rl-related-row--info { color: var(--rl-color-info-fg); }
.rl-related-row--success { color: var(--rl-color-success-fg); }
.rl-related-row--warning { color: var(--rl-color-warning-fg); }
.rl-related-row--danger { color: var(--rl-color-danger); }
</style>
