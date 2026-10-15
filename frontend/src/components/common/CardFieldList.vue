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

/**
 * One distinct value of a related-property field (TKT-8Z2LT3). It carries the
 * related entity's type, because the property's enum styles and display
 * format belong to that type, not to the card's.
 */
export interface RelatedCardValue {
  /** Unique within the field; the related type plus the value. */
  key: string
  /** Widget for this type's property; absent means render `text`. When no
   * value of a field has one, the field renders the field's joined `text`. */
  component?: Component
  propertyName: string
  entityType: string
  modelValue: unknown
  text: string
}

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
  /** Set for a related-property field: one entry per distinct value, in the
   * order the related entities came. Rendered side by side. */
  values?: RelatedCardValue[]
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

/** A stable key per line. A card may show a relation's titles and one of its
 * properties, or two properties through the same relation. */
function lineKey(field: KanbanCardField, i: number): string {
  return `${field.relation ?? ''}:${field.direction ?? ''}:${field.property ?? ''}:${i}`
}

/** A stable key per count field: a card may count a relation both ways. */
function countKey(field: KanbanCardField): string {
  return field.comments ? 'comments' : `${field.relation}:${field.direction ?? 'outgoing'}`
}
</script>

<template>
  <div v-if="fields.length" class="card-fields">
    <div v-for="(resolved, i) in lines" :key="lineKey(resolved.field, i)" class="card-field">
      <span v-if="cardFieldLabelShown(resolved.field)" class="field-label">
        {{ cardFieldLabel(resolved.field, relationLabel) }}:
      </span>
      <!-- Related values: widgets (badges, dates) side by side; plain text
           values joined with commas so two names do not read as one. -->
      <span
        v-if="resolved.values && resolved.values.every((v) => !v.component)"
        class="field-value"
        >{{ resolved.text }}</span
      >
      <span v-else-if="resolved.values" class="field-value related-values">
        <template v-for="value in resolved.values" :key="value.key">
          <component
            :is="value.component"
            v-if="value.component"
            :model-value="value.modelValue"
            mode="display"
            :property-name="value.propertyName"
            :entity-type="value.entityType"
          />
          <span v-else>{{ value.text }}</span>
        </template>
      </span>
      <component
        :is="resolved.component"
        v-else-if="resolved.component"
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

/* The distinct values of a related property, such as two segments, side by
   side like the badges of a list enum. */
.related-values {
  display: inline-flex;
  gap: var(--space-xs);
  overflow: hidden;
}

.field-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
