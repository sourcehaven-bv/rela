// Helpers for routing properties sections in EntityDetail between
// SectionEditForm (inline edit) and PropertyDisplay (read-only).
//
// Extracted from EntityDetail.vue so the routing decisions and the
// stale-response guard are testable as pure functions, decoupled from
// the SFC's router / pinia / schema-store wiring (TKT-IHC7B).

import type { ViewSection, ViewSectionField } from '@/api'
import type { Entity, PropertyDef } from '@/types'
import { defaultRegistry } from '@/widgets/registry'
import { viewFieldRoutingHint } from '@/widgets/viewRouting'
import { isFieldWritable } from '@/utils/affordances'
import type { SectionEditField } from '@/components/forms/SectionEditForm.vue'

// Implementor's note: `defaultRegistry` is imported only to keep
// downstream Vue scaffolding in scope when this module is consumed by
// `SectionEditForm`; the helpers below don't invoke it.
void defaultRegistry

// buildSectionEditFields shapes a properties section's fields for
// SectionEditForm: filters out fields without a wire-level property
// name (RR-FB1J), resolves each field's PropertyDef when the entry's
// type is in the schema, falls back to a WidgetRoutingHint otherwise,
// and attaches the per-field `_fields` verdict.
export function buildSectionEditFields(
  fields: ViewSectionField[] | undefined,
  ent: Entity,
  getPropertyDef: (entityType: string, propertyName: string) => PropertyDef | undefined,
): SectionEditField[] {
  if (!fields) return []
  const out: SectionEditField[] = []
  for (const f of fields) {
    if (!f.property) continue
    const def = getPropertyDef(ent.type, f.property)
    const verdict = ent._fields?.[f.property]
    if (def) {
      out.push({
        property: f.property,
        label: f.label,
        verdict,
        kind: 'schema',
        propertyDef: def,
      })
    } else {
      out.push({
        property: f.property,
        label: f.label,
        verdict,
        kind: 'hint',
        routingHint: viewFieldRoutingHint(f),
      })
    }
  }
  return out
}

// sectionHasAnyWritable: properties section gets routed to
// SectionEditForm only when at least one field is writable per
// `_fields`. Otherwise PropertyDisplay handles it unchanged.
export function sectionHasAnyWritable(
  section: ViewSection,
  ent: Entity,
  getPropertyDef: (entityType: string, propertyName: string) => PropertyDef | undefined,
): boolean {
  return buildSectionEditFields(section.fields, ent, getPropertyDef).some((f) =>
    isFieldWritable(f.verdict),
  )
}

// applyPropertyToEntry returns the next viewData entry shape after a
// confirmed server PATCH on (prop, value). Returns null when the
// guard rejects the apply — either because there's no current entry
// or the apply is from a stale previous-entity instance whose PATCH
// resolved after :key-driven remount (RR-FB2A).
//
// Caller assigns the result to `viewData.value.entry` when non-null.
export function applyPropertyToEntry(
  entry: Entity | null | undefined,
  prop: string,
  value: unknown,
  applyOwner: { type: string; id: string },
): Entity | null {
  if (!entry) return null
  if (entry.type !== applyOwner.type || entry.id !== applyOwner.id) return null
  const nextProps = { ...entry.properties }
  if (value === undefined) delete nextProps[prop]
  else nextProps[prop] = value
  return { ...entry, properties: nextProps }
}
