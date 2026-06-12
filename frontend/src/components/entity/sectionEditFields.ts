// Helpers for routing properties sections in EntityDetail between
// SectionEditForm (inline edit) and PropertyDisplay (read-only).
//
// Extracted from EntityDetail.vue so the routing decisions and the
// stale-response guard are testable as pure functions, decoupled from
// the SFC's router / pinia / schema-store wiring (TKT-IHC7B).

import type { ViewEntity, ViewSectionField } from '@/api'
import type { Entity, FieldAffordance, PropertyDef } from '@/types'
import { defaultRegistry } from '@/widgets/registry'
import { viewFieldRoutingHint } from '@/widgets/viewRouting'
import { isFieldWritable } from '@/utils/affordances'
import type { SectionEditField } from '@/components/forms/SectionEditForm.vue'

// Implementor's note: `defaultRegistry` is imported only to keep
// downstream Vue scaffolding in scope when this module is consumed by
// `SectionEditForm`; the helpers below don't invoke it.
void defaultRegistry

// FieldVerdictSource (TKT-IHC7C / RR-FC1A) is the minimal shape the
// per-field verdict helpers read. Both the entry's `Entity` (entry
// section) and a cards/list row's `ViewEntity` satisfy it — they
// expose `type` and the sparse `_fields` map. One helper handles
// both call sites.
export interface FieldVerdictSource {
  type: string
  _fields?: Record<string, FieldAffordance>
}

// buildSectionEditFields shapes a properties section's fields for
// SectionEditForm: filters out fields without a wire-level property
// name (RR-FB1J), resolves each field's PropertyDef when the entry's
// type is in the schema, falls back to a WidgetRoutingHint otherwise,
// and attaches the per-field `_fields` verdict.
//
// Parameterized over FieldVerdictSource (TKT-IHC7C) so the same helper
// serves both the entry section (Entity) and per-row inline edit
// (ViewEntity).
export function buildSectionEditFields(
  fields: ViewSectionField[] | undefined,
  source: FieldVerdictSource,
  getPropertyDef: (entityType: string, propertyName: string) => PropertyDef | undefined,
): SectionEditField[] {
  if (!fields) return []
  const out: SectionEditField[] = []
  for (const f of fields) {
    if (!f.property) continue
    const def = getPropertyDef(source.type, f.property)
    const verdict = source._fields?.[f.property]
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

// sectionShouldRouteToInlineEdit: properties section gets routed to
// SectionEditForm only when (a) at least one field is writable per
// `_fields` AND (b) no field on the section is inaccessible (e.g.
// git-crypt encrypted). The inaccessible affordance — a per-cell lock
// placeholder — is rendered by PropertyDisplay's `<InaccessibleField>`
// path; SectionEditForm doesn't model it (TKT-IHC7B explicitly scopes
// to writability gating, not inaccessibility).
//
// Parameterized over FieldVerdictSource (TKT-IHC7C) — same as
// buildSectionEditFields. Section fields come from `section.fields`
// for the entry section, `row.fields` for cards/list rows.
export function sectionShouldRouteToInlineEdit(
  fields: ViewSectionField[] | undefined,
  source: FieldVerdictSource,
  getPropertyDef: (entityType: string, propertyName: string) => PropertyDef | undefined,
): boolean {
  const fs = fields ?? []
  if (fs.some((f) => f.inaccessible)) return false
  return buildSectionEditFields(fs, source, getPropertyDef).some((f) =>
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

// rowShouldRouteToInlineEdit: decide per-row whether to mount a
// SectionEditForm for a cards/list item. Bails when:
//   - `_props` is absent (defensive — post-IHC7D server sends them; this
//     branch handles legacy / shape drift)
//   - the host's section size exceeds the soft cap (RR-FC1D)
//   - any field is inaccessible (the lock placeholder is rendered by
//     the display path, not SectionEditForm)
//   - no field in the row is writable per `_fields`
//
// (TKT-IHC7C). Separated from EntityDetail.vue so the cap-behaviour test
// can exercise it without mounting the full SFC.
export function rowShouldRouteToInlineEdit(
  row: ViewEntity,
  rowCount: number,
  rowCap: number,
  getPropertyDef: (entityType: string, propertyName: string) => PropertyDef | undefined,
): boolean {
  if (!row._props) return false
  if (rowCount > rowCap) return false
  return sectionShouldRouteToInlineEdit(row.fields, row, getPropertyDef)
}

// applyPropertyToRow (TKT-IHC7C) is the per-row equivalent of
// applyPropertyToEntry: takes a ViewEntity and produces the next
// shape after a confirmed PATCH. ViewEntity stores typed properties
// in `_props` (not `properties` like Entity), so this is a different
// helper despite the analogous owner-identity guard.
//
// String mirror (RR-FC1C): the row's `fields[i].values` display-
// stringified array is NOT updated here. Display-mode rendering
// reads `_props` first, falling back to `values` only when `_props`
// is absent (legacy server / shape drift). Keeping the string mirror
// stale-but-untouched eliminates a class of race conditions where
// the verdict flips back to display-mode mid-edit.
export function applyPropertyToRow(
  row: ViewEntity | null | undefined,
  prop: string,
  value: unknown,
  applyOwner: { type: string; id: string },
): ViewEntity | null {
  if (!row) return null
  if (row.type !== applyOwner.type || row.id !== applyOwner.id) return null
  const nextProps = { ...(row._props ?? {}) }
  if (value === undefined) delete nextProps[prop]
  else nextProps[prop] = value
  return { ...row, _props: nextProps }
}
