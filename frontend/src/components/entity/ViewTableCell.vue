<script setup lang="ts">
/**
 * One cell of a `display: table` view section.
 *
 * A table is a dense surface, so it routes like a list cell
 * (densePropertyRoutingHint), not like a detail row: a title renders as text,
 * a date as a date, a boolean as Yes/No text, and only an enum as a badge.
 * An empty cell stays blank. A relation column has no property and keeps the
 * server's joined string.
 */
import { computed, type Component } from 'vue'
import { useSchemaStore } from '@/stores'
import { defaultRegistry } from '@/widgets/registry'
import {
  densePropertyRoutingHint,
  isDenseEmpty,
  type DenseRoutingHint,
} from '@/widgets/viewRouting'
import { formatCellValue } from '@/utils/format'
import type { PropertyDef } from '@/types'

const props = defineProps<{
  values?: string[]
  property?: string
  entityType: string
}>()

const schemaStore = useSchemaStore()

const resolved = computed(() => {
  const values = props.values ?? []
  if (!props.property || isDenseEmpty(values)) return undefined
  const typeDef = schemaStore.getEntityType(props.entityType)
  const propertyDef = typeDef?.properties?.[props.property]
  const { widget, hint } = routeFor(propertyDef, props.property)
  // An undeclared property may still hold a list; keep every value.
  const raw = propertyDef?.list === true || values.length > 1 ? values : values[0]
  return {
    widget,
    value: hint.preformatted ? formatCellValue(raw, props.property, typeDef) : raw,
  }
})
</script>

<script lang="ts">
interface Route {
  widget: Component
  hint: DenseRoutingHint
}

// One widget resolution per property, not per cell: resolveFromHint walks a
// Map and can console.warn, which in a 200-row table would be 200 lookups per
// render. Keyed by the PropertyDef object, so a schema reload (new objects)
// resolves afresh. An undeclared property always routes to text.
const routes = new WeakMap<PropertyDef, Route>()
let textRoute: Route | undefined

function routeFor(propertyDef: PropertyDef | undefined, property: string): Route {
  if (!propertyDef) {
    textRoute ??= resolveRoute(undefined, property)
    return textRoute
  }
  let route = routes.get(propertyDef)
  if (!route) {
    route = resolveRoute(propertyDef, property)
    routes.set(propertyDef, route)
  }
  return route
}

function resolveRoute(propertyDef: PropertyDef | undefined, property: string): Route {
  const hint = densePropertyRoutingHint(propertyDef, property)
  return { widget: defaultRegistry.resolveFromHint(hint), hint }
}
</script>

<template>
  <component
    :is="resolved.widget"
    v-if="resolved"
    :model-value="resolved.value"
    mode="display"
    :property-name="property"
    :entity-type="entityType"
  />
  <template v-else>{{ (values ?? []).join(', ') }}</template>
</template>
