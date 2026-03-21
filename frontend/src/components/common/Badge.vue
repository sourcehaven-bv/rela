<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '@/stores/schema'
import type { EntityType } from '@/types'

const props = defineProps<{
  value: string
  property?: string
  entityType?: EntityType
}>()

const schemaStore = useSchemaStore()

// Map badge class names to inline styles
// Using colors that work in both light and dark modes
const badgeClassMap: Record<string, { backgroundColor: string; color: string }> = {
  'badge-blue': { backgroundColor: 'color-mix(in srgb, #3b82f6 20%, transparent)', color: '#60a5fa' },
  'badge-purple': { backgroundColor: 'color-mix(in srgb, #8b5cf6 20%, transparent)', color: '#a78bfa' },
  'badge-green': { backgroundColor: 'color-mix(in srgb, #22c55e 20%, transparent)', color: '#4ade80' },
  'badge-gray': { backgroundColor: 'var(--hover-bg)', color: 'var(--muted-text)' },
  'badge-red': { backgroundColor: 'color-mix(in srgb, #ef4444 20%, transparent)', color: '#f87171' },
  'badge-orange': { backgroundColor: 'color-mix(in srgb, #f97316 20%, transparent)', color: '#fb923c' },
  'badge-yellow': { backgroundColor: 'color-mix(in srgb, #eab308 20%, transparent)', color: '#facc15' },
}

const defaultStyle = { backgroundColor: 'var(--hover-bg)', color: 'var(--muted-text)' }

// Look up style: first try by property name, then search all properties
const badgeStyle = computed(() => {
  // Normalize: lowercase, spaces to underscores (keep underscores as-is)
  const valueKey = props.value.toLowerCase().replace(/\s/g, '_')
  const styles = schemaStore.styles

  // Try looking up by property name first if provided
  if (props.property) {
    const propStyles = styles[props.property]
    if (propStyles && propStyles[valueKey]) {
      return badgeClassMap[propStyles[valueKey]] || defaultStyle
    }
  }

  // Search all properties for this value
  for (const propStyles of Object.values(styles)) {
    if (propStyles && propStyles[valueKey]) {
      return badgeClassMap[propStyles[valueKey]] || defaultStyle
    }
  }

  return defaultStyle
})
</script>

<template>
  <span class="badge" :style="badgeStyle">
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
