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

// Map badge class names to CSS class names for styling
// Colors are defined in CSS variables for light/dark mode support
const badgeClassNames: Record<string, string> = {
  'badge-blue': 'badge--blue',
  'badge-purple': 'badge--purple',
  'badge-green': 'badge--green',
  'badge-gray': 'badge--gray',
  'badge-red': 'badge--red',
  'badge-orange': 'badge--orange',
  'badge-yellow': 'badge--yellow',
}

// Look up style: first try by property name, then search all properties
const badgeClass = computed(() => {
  // Normalize: lowercase, spaces to underscores (keep underscores as-is)
  const valueKey = props.value.toLowerCase().replace(/\s/g, '_')
  const styles = schemaStore.styles

  // Try looking up by property name first if provided
  if (props.property) {
    const propStyles = styles[props.property]
    if (propStyles && propStyles[valueKey]) {
      return badgeClassNames[propStyles[valueKey]] || 'badge--gray'
    }
  }

  // Search all properties for this value
  for (const propStyles of Object.values(styles)) {
    if (propStyles && propStyles[valueKey]) {
      return badgeClassNames[propStyles[valueKey]] || 'badge--gray'
    }
  }

  return 'badge--gray'
})
</script>

<template>
  <span class="badge" :class="badgeClass">
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

/* Light mode colors - darker text for better contrast */
.badge--blue {
  background-color: color-mix(in srgb, #3b82f6 18%, transparent);
  color: #1d4ed8;
}

.badge--purple {
  background-color: color-mix(in srgb, #8b5cf6 18%, transparent);
  color: #6d28d9;
}

.badge--green {
  background-color: color-mix(in srgb, #22c55e 18%, transparent);
  color: #15803d;
}

.badge--gray {
  background-color: var(--hover-bg);
  color: var(--muted-text);
}

.badge--red {
  background-color: color-mix(in srgb, #ef4444 18%, transparent);
  color: #b91c1c;
}

.badge--orange {
  background-color: color-mix(in srgb, #f97316 18%, transparent);
  color: #c2410c;
}

.badge--yellow {
  background-color: color-mix(in srgb, #eab308 18%, transparent);
  color: #a16207;
}

</style>

<!-- Dark mode: unscoped with higher specificity to override scoped [data-v-*] -->
<style>
.dark .badge.badge--blue {
  background-color: color-mix(in srgb, #3b82f6 20%, transparent);
  color: #60a5fa;
}

.dark .badge.badge--purple {
  background-color: color-mix(in srgb, #8b5cf6 22%, transparent);
  color: #c4b5fd;
}

.dark .badge.badge--green {
  background-color: color-mix(in srgb, #22c55e 20%, transparent);
  color: #4ade80;
}

.dark .badge.badge--red {
  background-color: color-mix(in srgb, #ef4444 20%, transparent);
  color: #f87171;
}

.dark .badge.badge--orange {
  background-color: color-mix(in srgb, #f97316 20%, transparent);
  color: #fb923c;
}

.dark .badge.badge--yellow {
  background-color: color-mix(in srgb, #eab308 20%, transparent);
  color: #fde047;
}
</style>
