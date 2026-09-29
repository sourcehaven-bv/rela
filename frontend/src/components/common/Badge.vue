<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '@/stores/schema'
import type { EntityType } from '@/types'

const props = defineProps<{
  value: string
  property?: string
  // Accepts a type-name string OR a resolved EntityType, matching the schema
  // store's resolution API — widget call sites hold the name, list/kanban
  // call sites hold the object.
  entityType?: string | EntityType
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

// Look up style by (property, value). The server keys the styles map by
// custom-type name, so resolution goes through the schema store's
// stylesForProperty (property -> its custom type -> styles), with a
// property-name fallback. The cross-property fallback that scanned every
// styled property for a value match was removed (RR-UD2D): it produced
// non-deterministic colours when a value (e.g. 'open') was styled under
// multiple properties. Audited consumers all pass an explicit :property=.
// A missing property -> the default gray; that's the correct "no styling
// configured" answer.
const badgeClass = computed(() => {
  if (!props.property) return 'badge--gray'
  // Normalize: lowercase, spaces to underscores (keep underscores as-is)
  const valueKey = props.value.toLowerCase().replace(/\s/g, '_')
  const propStyles = schemaStore.stylesForProperty(props.property, props.entityType)
  if (propStyles && propStyles[valueKey]) {
    return badgeClassNames[propStyles[valueKey]] || 'badge--gray'
  }
  return 'badge--gray'
})

// The metamodel-authored display label for this enum value, if any. Color
// styling above still keys on the raw `value` (the wire identity); only the
// text shown changes. When no label is configured we fall back to the raw
// value and let CSS `capitalize` prettify it as before.
const label = computed(() =>
  schemaStore.getEnumLabel(props.value, props.property, props.entityType),
)
const displayText = computed(() => label.value ?? props.value)
</script>

<template>
  <!-- When the metamodel supplies a label the author already chose the display
       form, so we suppress CSS capitalize; label text is interpolated (escaped),
       never v-html. -->
  <span class="badge" :class="[badgeClass, { 'badge--labeled': label !== undefined }]">
    {{ displayText }}
    <!-- Optional trailing adornment (e.g. a dropdown caret when the badge is an
         interactive control). Empty for the common read-only badge. -->
    <slot />
  </span>
</template>

<style scoped>
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  font-weight: 500;
  text-transform: capitalize;
}

/* A metamodel-authored label is already in its intended casing; don't let
   capitalize override the author's choice (e.g. "iOS", "high priority").
   Labels can also be arbitrarily long (unlike snake_case values), so guard
   them from blowing out list columns / kanban cards — scoped to labeled
   badges only, leaving raw-value badges rendered exactly as before. */
.badge--labeled {
  text-transform: none;
  max-width: 24ch;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.badge--blue {
  background-color: color-mix(in srgb, var(--rl-color-status-blue) 18%, transparent);
  color: var(--rl-color-status-blue);
}

.badge--purple {
  background-color: color-mix(in srgb, var(--rl-color-accent) 18%, transparent);
  color: var(--rl-color-accent);
}

.badge--green {
  background-color: color-mix(in srgb, var(--rl-color-status-green) 18%, transparent);
  color: var(--rl-color-status-green);
}

.badge--gray {
  background-color: var(--rl-color-bg-hover);
  color: var(--rl-color-text-muted);
}

.badge--red {
  background-color: color-mix(in srgb, var(--rl-color-status-red) 18%, transparent);
  color: var(--rl-color-status-red);
}

.badge--orange {
  background-color: color-mix(in srgb, var(--rl-color-status-amber) 18%, transparent);
  color: var(--rl-color-status-amber);
}

.badge--yellow {
  background-color: color-mix(in srgb, var(--rl-color-status-amber) 18%, transparent);
  color: var(--rl-color-status-amber);
}
</style>

<style>
/* Shared row layout for rendering multiple badges from a list-typed value.
   Unscoped so any consumer can wrap badges in <div class="badge-row">. */
.badge-row {
  display: inline-flex;
  flex-wrap: wrap;
  gap: var(--space-2xs);
}
</style>
