<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '@/stores/schema'
import type { EntityType } from '@/types'
import { STYLE_PALETTE, styleClassFor } from '@/utils/styleColors'

const props = defineProps<{
  value: string
  property?: string
  // Accepts a type-name string OR a resolved EntityType, matching the schema
  // store's resolution API — widget call sites hold the name, list/kanban
  // call sites hold the object.
  entityType?: string | EntityType
  // Text to show instead of the value's label, keeping the value's colour: a
  // relation field shows the target's title coloured by its status category
  // (TKT-CADCFX).
  text?: string
}>()

const schemaStore = useSchemaStore()

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
  // Value keys are normalized (lowercase, spaces to underscores) in styleKey.
  const propStyles = schemaStore.stylesForProperty(props.property, props.entityType)
  const cls = styleClassFor(propStyles, props.value)
  return (cls && STYLE_PALETTE[cls]?.badgeClass) || 'badge--gray'
})

// The metamodel-authored display label for this enum value, if any. Color
// styling above still keys on the raw `value` (the wire identity); only the
// text shown changes. When no label is configured we fall back to the raw
// value and let CSS `capitalize` prettify it as before.
const label = computed(() =>
  schemaStore.getEnumLabel(props.value, props.property, props.entityType),
)
const displayText = computed(() => props.text ?? label.value ?? props.value)
</script>

<template>
  <!-- When the metamodel supplies a label the author already chose the display
       form, so we suppress CSS capitalize; label text is interpolated (escaped),
       never v-html. -->
  <span class="badge" :class="[badgeClass, { 'badge--labeled': label !== undefined || text !== undefined }]">
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

/* Badge text needs 4.5:1, which the status-dot tones (3:1, non-text) do not
   promise. The tag pairs do: each -fg clears AA on its own -bg. Orange and
   yellow share the amber pair. */
.badge--blue {
  background-color: var(--rl-tag-blue-bg);
  color: var(--rl-tag-blue-fg);
}

.badge--purple {
  background-color: var(--rl-tag-purple-bg);
  color: var(--rl-tag-purple-fg);
}

.badge--green {
  background-color: var(--rl-tag-green-bg);
  color: var(--rl-tag-green-fg);
}

.badge--gray {
  background-color: var(--rl-tag-grey-bg);
  color: var(--rl-tag-grey-fg);
}

.badge--red {
  background-color: var(--rl-tag-red-bg);
  color: var(--rl-tag-red-fg);
}

.badge--orange {
  background-color: var(--rl-tag-amber-bg);
  color: var(--rl-tag-amber-fg);
}

.badge--yellow {
  background-color: var(--rl-tag-amber-bg);
  color: var(--rl-tag-amber-fg);
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
