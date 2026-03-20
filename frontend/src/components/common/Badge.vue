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
const badgeClassMap: Record<string, { backgroundColor: string; color: string }> = {
  'badge-blue': { backgroundColor: '#dbeafe', color: '#1e40af' },
  'badge-purple': { backgroundColor: '#e9d5ff', color: '#6b21a8' },
  'badge-green': { backgroundColor: '#dcfce7', color: '#166534' },
  'badge-gray': { backgroundColor: '#e2e8f0', color: '#475569' },
  'badge-red': { backgroundColor: '#fee2e2', color: '#991b1b' },
  'badge-orange': { backgroundColor: '#fed7aa', color: '#9a3412' },
  'badge-yellow': { backgroundColor: '#fef08a', color: '#854d0e' },
}

const defaultStyle = { backgroundColor: '#e2e8f0', color: '#475569' }

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
