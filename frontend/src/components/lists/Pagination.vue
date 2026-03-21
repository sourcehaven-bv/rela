<script setup lang="ts">
import { computed } from 'vue'
import type { ListMeta } from '@/types'

const props = defineProps<{
  meta: ListMeta
}>()

const emit = defineEmits<{
  'page-change': [page: number]
}>()

const totalPages = computed(() => Math.ceil(props.meta.total / props.meta.per_page))

const visiblePages = computed(() => {
  const pages: (number | 'ellipsis')[] = []
  const current = props.meta.page
  const total = totalPages.value

  if (total <= 7) {
    for (let i = 1; i <= total; i++) {
      pages.push(i)
    }
  } else {
    pages.push(1)

    if (current > 3) {
      pages.push('ellipsis')
    }

    const start = Math.max(2, current - 1)
    const end = Math.min(total - 1, current + 1)

    for (let i = start; i <= end; i++) {
      pages.push(i)
    }

    if (current < total - 2) {
      pages.push('ellipsis')
    }

    pages.push(total)
  }

  return pages
})

function goToPage(page: number) {
  if (page < 1 || page > totalPages.value || page === props.meta.page) return
  emit('page-change', page)
}
</script>

<template>
  <div class="pagination">
    <div class="pagination-info">
      Showing {{ (meta.page - 1) * meta.per_page + 1 }} -
      {{ Math.min(meta.page * meta.per_page, meta.total) }} of {{ meta.total }}
    </div>

    <div class="pagination-controls">
      <button
        class="pagination-btn pagination-nav"
        :disabled="meta.page === 1"
        @click="goToPage(meta.page - 1)"
      >
        &larr; <kbd>H</kbd>
      </button>

      <template v-for="(page, index) in visiblePages" :key="index">
        <span v-if="page === 'ellipsis'" class="pagination-ellipsis">...</span>
        <button
          v-else
          class="pagination-btn"
          :class="{ active: page === meta.page }"
          @click="goToPage(page)"
        >
          {{ page }}
        </button>
      </template>

      <button
        class="pagination-btn pagination-nav"
        :disabled="!meta.has_more"
        @click="goToPage(meta.page + 1)"
      >
        <kbd>L</kbd> &rarr;
      </button>
    </div>
  </div>
</template>

<style scoped>
.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-top: 1px solid var(--border-color, #e2e8f0);
}

.pagination-info {
  font-size: 14px;
  color: var(--muted-text);
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 4px;
}

.pagination-btn {
  min-width: 32px;
  height: 32px;
  padding: 0 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-size: 14px;
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.15s;
}

.pagination-btn:hover:not(:disabled) {
  background: var(--hover-bg);
  border-color: var(--accent-color);
}

.pagination-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.pagination-btn.active {
  background: var(--accent-color);
  border-color: var(--accent-color);
  color: white;
}

.pagination-ellipsis {
  padding: 0 8px;
  color: var(--muted-text);
}

.pagination-nav {
  gap: 4px;
}

.pagination-nav kbd {
  background: var(--hover-bg);
  border-color: var(--border-color);
  color: var(--muted-text);
}

.pagination-nav:disabled kbd {
  opacity: 0.5;
}
</style>
