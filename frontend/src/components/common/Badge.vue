<script setup lang="ts">
import { computed } from 'vue'
import type { EntityType } from '@/types'

const props = defineProps<{
  value: string
  property?: string
  entityType?: EntityType
}>()

// Map common status/priority values to colors
const colorMap: Record<string, string> = {
  // Status colors
  open: '#3b82f6',
  'in-progress': '#f59e0b',
  done: '#10b981',
  closed: '#6b7280',
  draft: '#94a3b8',
  pending: '#f59e0b',
  approved: '#10b981',
  rejected: '#ef4444',
  blocked: '#ef4444',
  ready: '#3b82f6',
  accepted: '#10b981',
  deprecated: '#6b7280',

  // Priority colors
  low: '#94a3b8',
  medium: '#3b82f6',
  high: '#f59e0b',
  critical: '#ef4444',
  urgent: '#ef4444',

  // Boolean-like
  yes: '#10b981',
  no: '#ef4444',
  true: '#10b981',
  false: '#ef4444',
}

const badgeColor = computed(() => {
  const key = props.value.toLowerCase().replace(/[_\s]/g, '-')
  return colorMap[key] || '#6b7280'
})

const textColor = computed(() => {
  // Determine if we need light or dark text based on background
  const hex = badgeColor.value.replace('#', '')
  const r = parseInt(hex.slice(0, 2), 16)
  const g = parseInt(hex.slice(2, 4), 16)
  const b = parseInt(hex.slice(4, 6), 16)
  const brightness = (r * 299 + g * 587 + b * 114) / 1000
  return brightness > 128 ? '#1e293b' : '#ffffff'
})
</script>

<template>
  <span
    class="badge"
    :style="{
      backgroundColor: badgeColor,
      color: textColor,
    }"
  >
    {{ value }}
  </span>
</template>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  text-transform: capitalize;
}
</style>
