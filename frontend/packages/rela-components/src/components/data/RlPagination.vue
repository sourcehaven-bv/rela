<script setup lang="ts">
/**
 * Page controls, with the middle of a long range collapsed to an ellipsis.
 *
 * Wrapped in a nav landmark and marking the current page with
 * `aria-current`, so a screen reader user can find the control and knows
 * where they are without counting buttons.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlText from '../common/RlText.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    page: number
    pageCount: number
    /** How many numbers to show around the current page. */
    siblings?: number
    /** Summary such as `1-25 of 340`, shown beside the controls. */
    summary?: string
    disabled?: boolean
  }>(),
  { siblings: 1, disabled: false },
)

defineEmits<{ 'update:page': [page: number] }>()

/**
 * The first and last pages are always offered, with a gap standing in for
 * whatever is skipped. Gaps are typed rather than being a magic number, so
 * the template never confuses one with a real page.
 */
const items = computed<Array<number | 'gap'>>(() => {
  const { page, pageCount, siblings } = props
  const total = siblings * 2 + 5

  if (pageCount <= total) {
    return Array.from({ length: pageCount }, (_, index) => index + 1)
  }

  const start = Math.max(2, page - siblings)
  const end = Math.min(pageCount - 1, page + siblings)
  const result: Array<number | 'gap'> = [1]

  if (start > 2) result.push('gap')
  for (let index = start; index <= end; index += 1) result.push(index)
  if (end < pageCount - 1) result.push('gap')

  result.push(pageCount)
  return result
})
</script>

<template>
  <nav class="rl-pagination" aria-label="Pagination">
    <RlText v-if="summary" size="sm" tone="muted" class="rl-pagination__summary">
      {{ summary }}
    </RlText>

    <ul class="rl-pagination__list">
      <li>
        <button
          type="button"
          class="rl-pagination__button"
          :disabled="disabled || page <= 1"
          aria-label="Previous page"
          @click="$emit('update:page', page - 1)"
        >
          <RlIcon name="chevron-right" :size="16" class="rl-pagination__prev" aria-hidden="true" />
        </button>
      </li>

      <li v-for="(item, index) in items" :key="`${item}-${index}`">
        <!-- A gap is not interactive, and is hidden so it is not read as "ellipsis". -->
        <span v-if="item === 'gap'" class="rl-pagination__gap" aria-hidden="true">…</span>
        <button
          v-else
          type="button"
          class="rl-pagination__button"
          :class="{ 'rl-pagination__button--current': item === page }"
          :disabled="disabled"
          :aria-label="messages.pageNumber({ page: item })"
          :aria-current="item === page ? 'page' : undefined"
          @click="$emit('update:page', item)"
        >
          {{ item }}
        </button>
      </li>

      <li>
        <button
          type="button"
          class="rl-pagination__button"
          :disabled="disabled || page >= pageCount"
          aria-label="Next page"
          @click="$emit('update:page', page + 1)"
        >
          <RlIcon name="chevron-right" :size="16" aria-hidden="true" />
        </button>
      </li>
    </ul>
  </nav>
</template>

<style scoped>
.rl-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--rl-space-4);
  flex-wrap: wrap;
}

.rl-pagination__list {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-pagination__button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 32px;
  padding: 0 var(--rl-space-2);
  border: none;
  border-radius: var(--rl-radius-md);
  background: transparent;
  color: var(--rl-color-text-muted);
  font-family: inherit;
  font-size: var(--rl-font-size-sm);
  cursor: pointer;
}

.rl-pagination__button:hover:not(:disabled) {
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text);
}

.rl-pagination__button--current {
  background: var(--rl-color-accent);
  color: var(--rl-color-text-inverse);
  font-weight: var(--rl-font-weight-medium);
}
.rl-pagination__button--current:hover:not(:disabled) {
  background: var(--rl-color-accent-hover);
  color: var(--rl-color-text-inverse);
}

.rl-pagination__button:disabled { opacity: 0.4; cursor: not-allowed; }

.rl-pagination__button:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: var(--rl-focus-ring-gap);
}

/* One chevron glyph, flipped, rather than two icons that might drift apart. */
.rl-pagination__prev { transform: rotate(180deg); }

.rl-pagination__gap {
  display: inline-flex;
  justify-content: center;
  min-width: 24px;
  color: var(--rl-color-text-subtle);
}

@media (pointer: coarse) {
  .rl-pagination__button { min-width: var(--rl-tap-target); height: var(--rl-tap-target); }
}
</style>
