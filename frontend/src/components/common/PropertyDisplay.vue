<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import InaccessibleField from './InaccessibleField.vue'
import { defaultRegistry } from '@/widgets/registry'
import { fieldSpanStyle } from '@/utils/fieldSpan'
import type { AttachmentInfo, EntityType, PropertyDef } from '@/types'

export interface PropertyItem {
  name: string
  label: string
  value: unknown
  type?: string
  values?: string[] // For enum type detection
  propType?: string // For badge styling lookup (used by CustomView)
  // Pre-resolved schema def for the property -- supplied by callers
  // (EntityDetail's mapFieldsToProperties) so PropertyDisplay does not
  // do a schema lookup per row (RR-UD1H).
  propertyDef?: PropertyDef
  isLongText?: boolean
  // Authored width on the 12-column property grid (TKT-5V8704). Absent or 0
  // means full width. Ignored when the value is long-form: a paragraph
  // squeezed into a third of the row is unreadable whatever the author said.
  span?: number
  inaccessible?: boolean // Property exists but value is unreadable (e.g. encrypted)
  inaccessibleReason?: string // Reason marker (e.g. "git-crypt") shown in tooltip
  // Attachment LIST for a `file`-type property, supplied by callers from
  // the entity's `_attachments` map, plus the property's `max`. Forwarded
  // to the file widget.
  attachments?: AttachmentInfo[]
  max?: number
}

const props = defineProps<{
  properties: PropertyItem[]
  // entityType is kept on the props signature for backward compatibility
  // with CustomView, which used it for badge style lookup.
  entityType?: EntityType
}>()

interface PropertyRow {
  prop: PropertyItem
  widget: Component
  propertyName: string
  propertyDef?: PropertyDef
  // Position in the rendered list, exposed via the label-affordance slot so a
  // consumer can reason about a field's place in the grid row (the comment
  // popover flips its alignment near the right edge).
  index: number
}

// Precompute rows once per properties array change instead of recomputing
// widget+def inline per render (RR-UD2A). When prop.propertyDef is
// present, use the form-side resolve(); otherwise fall back to a
// WidgetRoutingHint derived from the wire-level shape (RR-UD2B).
const rows = computed<PropertyRow[]>(() =>
  props.properties.map((prop, index) => {
    const propertyName = prop.propType ?? prop.name
    if (prop.propertyDef) {
      return {
        prop,
        widget: defaultRegistry.resolve(undefined, prop.propertyDef),
        propertyName,
        propertyDef: prop.propertyDef,
        index,
      }
    }
    // No schema def -- use a routing hint. Mirrors EntityDetail's
    // cards/list heuristic: any propType triggers enum-list, multi-
    // value text triggers text-list, everything else is plain text.
    const isMulti = Array.isArray(prop.value) && (prop.value as unknown[]).length > 1
    const kind = prop.propType ? 'enum-list' : isMulti ? 'text-list' : 'text'
    return {
      prop,
      widget: defaultRegistry.resolveFromHint({ kind, propertyName }),
      propertyName,
      index,
    }
  })
)

function isLong(prop: PropertyItem): boolean {
  if (prop.isLongText) return true
  const val = String(prop.value || '')
  return val.length > 60
}
</script>

<template>
  <div class="properties-list">
    <div
      v-for="row in rows"
      :key="row.prop.name"
      class="property-item"
      :class="{ 'property-long': isLong(row.prop) }"
      :style="isLong(row.prop) ? undefined : fieldSpanStyle(row.prop.span)"
    >
      <dt>
        {{ row.prop.label }}
        <!-- Per-field affordances (the comment indicator, TKT-FIO205). A slot
             rather than a prop because PropertyDisplay is shared with dense
             surfaces — list cells, kanban cards — where a comment control
             would be noise. Only the detail view fills it. -->
        <slot name="label-affordance" :property="row.prop" :index="row.index" />
      </dt>
      <dd>
        <InaccessibleField
          v-if="row.prop.inaccessible"
          :reason="row.prop.inaccessibleReason"
        />
        <component
          :is="row.widget"
          v-else
          :model-value="row.prop.value"
          :mode="'display'"
          :property-def="row.propertyDef"
          :property-name="row.propertyName"
          :attachments="row.prop.attachments"
          :max="row.prop.max"
        />
      </dd>
    </div>
  </div>
</template>

<style scoped>
/* .properties-list / .property-item / .property-long live in
 * styles/properties-list.css, shared with SectionEditForm and SidePanel.
 * Do not redefine them here. */

.property-item.property-long dd {
  white-space: pre-wrap;
  /* Force-wrap unbreakable strings (URLs, no-space identifiers).
     overflow-wrap: anywhere is in src/styles/text-utilities.css as
     .text-wrap-anywhere — we keep it inline here because dd is rendered
     via v-for and we don't want to thread a class through PropertyItem. */
  overflow-wrap: anywhere;
}

.property-inaccessible {
  color: var(--muted-text);
  font-style: italic;
  cursor: help;
}
</style>
