/**
 * Values for kanban card fields that show a property of the related entities:
 * `{relation, direction, property}` (TKT-8Z2LT3).
 *
 * # Gating
 *
 * Reads only what the board fetch already carries: the ids in the gated
 * `relations` map and the related entities in `included`, which the server
 * row-gates and field-redacts. A related entity the reader cannot see is not
 * there, and a property hidden by `visible:` is absent from its properties, so
 * either case adds nothing. Unlike a title field there is no raw-id fallback:
 * an id is not a value of the property.
 *
 * # Values
 *
 * Each distinct value shows once, in relation order. A list-valued property
 * contributes its elements, so two organisations tagged `[a, b]` and `[a]`
 * show `a` and `b`. Values are compared by value alone: the same enum value
 * reached through two related types is one badge, styled by the first type.
 */
import { computed, type Component } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import type { Entity, KanbanCardField, PropertyDef } from '@/types'
import type { RelatedCardValue } from '@/components/common/CardFieldList.vue'
import { formatCellValue } from '@/utils/format'
import { defaultRegistry } from '@/widgets/registry'
import {
  densePropertyRoutingHint,
  isDenseEmpty,
  type DenseRoutingHint,
} from '@/widgets/viewRouting'

interface Options {
  /** The board's configured card fields. */
  fields: () => KanbanCardField[]
  /** The board fetch's `included` map, keyed by id. */
  included: () => Record<string, Entity>
  /** The key under which a field's related ids sit in an entity's `relations`. */
  relationKey: (field: KanbanCardField) => string
}

function widgetKey(type: string, property: string): string {
  return `${type}\u0000${property}`
}

/** The element shape of a list property: each element renders as one value. */
function elementDef(def: PropertyDef | undefined): PropertyDef | undefined {
  return def?.list ? { ...def, list: false } : def
}

export function useRelatedCardValues(options: Options) {
  const schemaStore = useSchemaStore()
  const uiStore = useUIStore()

  // The types at the other end of a field's relation: the relation's `from`
  // types for an incoming field, its `to` types otherwise. The server
  // materializes `direction`, so it is set on the wire.
  function relatedTypes(field: KanbanCardField): string[] {
    const rel = schemaStore.getRelationType(field.relation ?? '')
    if (!rel) return []
    return field.direction === 'incoming' ? rel.from : rel.to
  }

  // Widgets resolved once per field and related type, as the board does for its
  // own properties (RR-UD2A). Keyed by type as well: the same property name may
  // be an enum on one related type and a string on another.
  const widgets = computed(() => {
    const byKey = new Map<string, { component: Component; hint: DenseRoutingHint }>()
    for (const field of options.fields()) {
      if (!field.relation || !field.property) continue
      for (const type of relatedTypes(field)) {
        const key = widgetKey(type, field.property)
        if (byKey.has(key)) continue
        const def = elementDef(schemaStore.getEntityType(type)?.properties[field.property])
        const hint = densePropertyRoutingHint(def, field.property)
        byKey.set(key, { component: defaultRegistry.resolveFromHint(hint), hint })
      }
    }
    return byKey
  })

  /** The distinct values of `field.property` on the entity's related entities. */
  function relatedCardValues(entity: Entity, field: KanbanCardField): RelatedCardValue[] {
    const property = field.property ?? ''
    const included = options.included()
    const out: RelatedCardValue[] = []
    const seen = new Set<string>()
    for (const id of entity.relations?.[options.relationKey(field)] ?? []) {
      const related = included[id]
      if (!related) continue
      const raw = related.properties[property]
      for (const value of Array.isArray(raw) ? raw : [raw]) {
        if (isDenseEmpty(value)) continue
        const key = JSON.stringify(value)
        if (seen.has(key)) continue
        seen.add(key)
        const entry = widgets.value.get(widgetKey(related.type, property))
        const text = formatCellValue(
          value,
          property,
          schemaStore.getEntityType(related.type),
          uiStore.effectiveTimezone
        )
        // A preformatted kind (text, integer) renders its text and nothing
        // else, so it goes without a widget; CardFieldList then joins such
        // values with commas instead of setting them side by side.
        const widget = entry && !entry.hint.preformatted ? entry.component : undefined
        out.push({
          key,
          component: widget,
          propertyName: property,
          entityType: related.type,
          modelValue: value,
          text,
        })
      }
    }
    return out
  }

  return { relatedCardValues }
}
