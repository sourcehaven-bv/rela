<script setup lang="ts">
/**
 * The label-and-value detail lines shown on a kanban card or a calendar event
 * chip.
 *
 * Both surfaces render the same thing — a derived label, then a value routed
 * through the dense widget registry — so they share this rather than keeping
 * two implementations that drift. Before it existed, kanban always printed a
 * label and the calendar never did; neither was configurable.
 *
 * # What the caller supplies
 *
 * Resolution happens in the caller, ONCE per configured field, and arrives here
 * already done. That ordering is load-bearing: `registry.resolve` walks a Map
 * and can warn, so resolving per card (or per chip) repeats that work for every
 * row on screen — 200 rows meaning 200 warnings per render (RR-UD2A).
 *
 * Fields whose value is empty must be filtered out by the caller too, matching
 * the dense-surface rule that an empty value renders as nothing rather than a
 * placeholder.
 */
import type { Component } from 'vue'
import { cardFieldLabel, cardFieldLabelShown, type KanbanCardField } from '@/types/config'
import { useSchemaStore } from '@/stores/schema'

/** A field with its value already resolved by the caller. */
export interface ResolvedCardField {
  field: KanbanCardField
  /** Widget to render the value with; absent means render `text`. */
  component?: Component
  propertyName?: string
  modelValue?: unknown
  /** Formatted value, used when no widget applies (relations, plain values). */
  text: string
}

defineProps<{
  fields: ResolvedCardField[]
  /** Forwarded to widgets that need it for enum styling. */
  entityType?: string
}>()

const schemaStore = useSchemaStore()

/** A relation's authored label from the metamodel, so `belongs-to` renders as
 * "belongs to" rather than looking like a raw field name. */
function relationLabel(relation: string): string | undefined {
  return schemaStore.getRelationType(relation)?.label
}
</script>

<template>
  <div v-if="fields.length" class="card-fields">
    <div
      v-for="(resolved, i) in fields"
      :key="resolved.field.relation || resolved.field.property || i"
      class="card-field"
    >
      <span v-if="cardFieldLabelShown(resolved.field)" class="field-label">
        {{ cardFieldLabel(resolved.field, relationLabel) }}:
      </span>
      <component
        :is="resolved.component"
        v-if="resolved.component"
        class="field-value"
        :model-value="resolved.modelValue"
        mode="display"
        :property-name="resolved.propertyName"
        :entity-type="entityType"
      />
      <span v-else class="field-value">{{ resolved.text }}</span>
    </div>
  </div>
</template>

<style scoped>
.card-fields {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

/* One field per line, label and value on the same row. Stacked rather than
   flowing: a run of values with no line breaks reads as a sentence, and two
   person fields become indistinguishable. */
.card-field {
  display: flex;
  align-items: baseline;
  gap: var(--space-xs);
  min-width: 0;
  font-size: var(--font-size-sm);
}

.field-label {
  flex: none;
  color: var(--muted-text);
}

.field-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
