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

// Look up style by (property, value). The cross-property fallback that
// scanned every styled property for a value match was removed (RR-UD2D):
// it produced non-deterministic colours when a value (e.g. 'open') was
// styled under multiple properties. Audited consumers all pass an
// explicit :property=. A missing property -> the default gray; that's
// the correct "no styling configured" answer.
const badgeClass = computed(() => {
  if (!props.property) return 'badge--gray'
  // Normalize: lowercase, spaces to underscores (keep underscores as-is)
  const valueKey = props.value.toLowerCase().replace(/\s/g, '_')
  const propStyles = schemaStore.styles[props.property]
  if (propStyles && propStyles[valueKey]) {
    return badgeClassNames[propStyles[valueKey]] || 'badge--gray'
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

.badge--blue {
  background-color: color-mix(in srgb, var(--badge-blue) 18%, transparent);
  color: var(--badge-blue);
}

.badge--purple {
  background-color: color-mix(in srgb, var(--badge-purple) 18%, transparent);
  color: var(--badge-purple);
}

.badge--green {
  background-color: color-mix(in srgb, var(--badge-green) 18%, transparent);
  color: var(--badge-green);
}

.badge--gray {
  background-color: var(--hover-bg);
  color: var(--muted-text);
}

.badge--red {
  background-color: color-mix(in srgb, var(--badge-red) 18%, transparent);
  color: var(--badge-red);
}

.badge--orange {
  background-color: color-mix(in srgb, var(--badge-orange) 18%, transparent);
  color: var(--badge-orange);
}

.badge--yellow {
  background-color: color-mix(in srgb, var(--badge-yellow) 18%, transparent);
  color: var(--badge-yellow);
}
</style>

<style>
/* Shared row layout for rendering multiple badges from a list-typed value.
   Unscoped so any consumer can wrap badges in <div class="badge-row">. */
.badge-row {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
}
</style>
