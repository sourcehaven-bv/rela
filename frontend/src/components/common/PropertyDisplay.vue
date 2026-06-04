<script setup lang="ts">
import InaccessibleField from './InaccessibleField.vue'
import { defaultRegistry } from '@/widgets/registry'
import type { EntityType, PropertyDef } from '@/types'

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
  inaccessible?: boolean // Property exists but value is unreadable (e.g. encrypted)
  inaccessibleReason?: string // Reason marker (e.g. "git-crypt") shown in tooltip
}

defineProps<{
  properties: PropertyItem[]
  // entityType is kept on the props signature for backward compatibility
  // with CustomView, which used it for badge style lookup. The widget
  // path now uses `propType` (forwarded into the widget as
  // `propertyName`); badge styling remains correct via Badge's
  // cross-property fallback when both are absent.
  entityType?: EntityType
}>()

// Build a synthetic PropertyDef when none was pre-resolved. Lets
// PropertyDisplay still render (via the registry's text fallback) for
// callers that haven't been updated to supply propertyDef.
function defForProp(prop: PropertyItem): PropertyDef {
  if (prop.propertyDef) return prop.propertyDef
  return {
    type: ((prop.type as PropertyDef['type']) ?? 'string'),
    values: prop.values,
    list: Array.isArray(prop.value) && (prop.value as unknown[]).length > 1,
  }
}

function widgetFor(prop: PropertyItem) {
  return defaultRegistry.resolve(undefined, defForProp(prop))
}

function isLong(prop: PropertyItem): boolean {
  if (prop.isLongText) return true
  const val = String(prop.value || '')
  return val.length > 60
}
</script>

<template>
  <div class="properties-list">
    <div
      v-for="prop in properties"
      :key="prop.name"
      class="property-item"
      :class="{ 'property-long': isLong(prop) }"
    >
      <dt>{{ prop.label }}</dt>
      <dd>
        <InaccessibleField
          v-if="prop.inaccessible"
          :reason="prop.inaccessibleReason"
        />
        <component
          :is="widgetFor(prop)"
          v-else
          :model-value="prop.value"
          :mode="'display'"
          :property-def="defForProp(prop)"
          :property-name="prop.propType ?? prop.name"
        />
      </dd>
    </div>
  </div>
</template>

<style scoped>
.properties-list {
  display: flex;
  flex-wrap: wrap;
  gap: 16px 32px;
}

.property-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 120px;
}

.property-item.property-long {
  flex-basis: 100%;
  min-width: 100%;
}

.property-item dt {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--muted-text);
}

.property-item dd {
  margin: 0;
  font-size: 14px;
  color: var(--text-color);
  line-height: 1.5;
}

.property-item.property-long dd {
  white-space: pre-wrap;
  word-break: break-word;
}

.property-inaccessible {
  color: var(--muted-text);
  font-style: italic;
  cursor: help;
}
</style>
