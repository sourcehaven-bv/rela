<script setup lang="ts">
import Badge from './Badge.vue'
import { formatValue, isEnumProperty } from '@/utils/format'
import type { EntityType } from '@/types'

export interface PropertyItem {
  name: string
  label: string
  value: unknown
  type?: string
  values?: string[] // For enum type detection
  propType?: string // For badge styling lookup (used by CustomView)
  isLongText?: boolean
}

defineProps<{
  properties: PropertyItem[]
  entityType?: EntityType // For badge styling lookup (used by EntityDetail)
}>()

function shouldUseBadge(prop: PropertyItem): boolean {
  // Check if it's an enum property (has values array or propType)
  if (isEnumProperty(prop)) return prop.value != null && prop.value !== ''
  // Also show badge if propType is explicitly set (CustomView API response)
  if (prop.propType && prop.value != null && prop.value !== '') return true
  return false
}

function getBadgeProperty(prop: PropertyItem): string {
  // Use propType if available (from CustomView API), otherwise use property name
  return prop.propType || prop.name
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
        <Badge
          v-if="shouldUseBadge(prop)"
          :value="String(prop.value)"
          :property="getBadgeProperty(prop)"
          :entity-type="entityType"
        />
        <span v-else>{{ formatValue(prop.value, prop.type) }}</span>
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
</style>
