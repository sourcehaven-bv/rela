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
import { computed, type Component } from 'vue'
import RlMetaItem from 'rela-components/components/common/RlMetaItem.vue'
import type { IconName } from 'rela-components/components/common/icons'
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
  /** Set for a count field (comments, `display: count`); rendered as an icon
   * and number in one row under the other fields. */
  count?: { value: number; icon: IconName }
}

const props = defineProps<{
  fields: ResolvedCardField[]
  /** Forwarded to widgets that need it for enum styling. */
  entityType?: string
}>()

const schemaStore = useSchemaStore()

const lines = computed(() => props.fields.filter((f) => !f.count))
const counts = computed(() => props.fields.filter((f) => f.count))

/** A relation's authored label from the metamodel, so `belongs-to` renders as
 * "belongs to" rather than looking like a raw field name. */
function relationLabel(relation: string): string | undefined {
  return schemaStore.getRelationType(relation)?.label
}

/** A stable key per count field: a card may count a relation both ways. */
function countKey(field: KanbanCardField): string {
  return field.comments ? 'comments' : `${field.relation}:${field.direction ?? 'outgoing'}`
}
</script>

<template>
  <div v-if="fields.length" class="card-fields">
    <div
      v-for="(resolved, i) in lines"
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
    <div v-if="counts.length" class="card-counts">
      <!-- The label is visible unless show_label is false: two relation counts
           share an icon, so the number alone cannot say which is which. -->
      <RlMetaItem
        v-for="resolved in counts"
        :key="countKey(resolved.field)"
        :icon="resolved.count!.icon"
        :label="
          cardFieldLabelShown(resolved.field)
            ? `${resolved.count!.value} ${cardFieldLabel(resolved.field, relationLabel)}`
            : resolved.count!.value
        "
        :count-label="
          cardFieldLabelShown(resolved.field)
            ? undefined
            : cardFieldLabel(resolved.field, relationLabel)
        "
        :title="cardFieldLabel(resolved.field, relationLabel)"
      />
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

.card-counts {
  display: flex;
  gap: var(--space-sm);
  margin-top: 2px;
}

.field-label {
  flex: none;
  color: var(--rl-color-text-muted);
}

.field-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
