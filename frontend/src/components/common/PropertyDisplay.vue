<script setup lang="ts">
import Badge from './Badge.vue'
import { asArray, formatValue, isEnumProperty } from '@/utils/format'
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
  if (!hasBadgeValue(prop)) return false
  if (isEnumProperty(prop)) return true
  // Also show badge if propType is explicitly set (CustomView API response)
  return !!prop.propType
}

function hasBadgeValue(prop: PropertyItem): boolean {
  if (Array.isArray(prop.value)) return asArray(prop.value).length > 0
  return prop.value != null && prop.value !== ''
}

function getBadgeProperty(prop: PropertyItem): string {
  // Use propType if available (from CustomView API), otherwise use property name
  return prop.propType || prop.name
}

function getBadgeValues(prop: PropertyItem): string[] {
  return asArray(prop.value)
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
        <div v-if="shouldUseBadge(prop)" class="badge-row">
          <Badge
            v-for="badgeValue in getBadgeValues(prop)"
            :key="badgeValue"
            :value="badgeValue"
            :property="getBadgeProperty(prop)"
            :entity-type="entityType"
          />
        </div>
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
