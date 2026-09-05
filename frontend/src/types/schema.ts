export interface Schema {
  entities: Record<string, EntityType>
  relations: Record<string, RelationType>
  types: Record<string, CustomType>
  // Every DECLARED world plus the implicit `default`, each marked with
  // whether THIS caller may select it (TKT-WRLDAPI item 1). Absent on a
  // server too old to compute it.
  //
  // The declared SET is principal-independent — world names are
  // operator-authored schema.yaml config, so their existence is already
  // disclosed and the server does not filter them per principal. Only
  // `readable` varies by caller.
  worlds?: Record<string, WorldInfo>
}

// WorldInfo mirrors v1.World. See internal/dataentry/schemaworlds.go.
export interface WorldInfo {
  select?: string[]
  overrides?: Record<string, string[]>
  otherwise?: string
  // Faces this world is the canonical home of — the operator's tie-break when
  // several worlds head the same face for a type (TKT-MFVH03). Absent for an
  // unambiguous schema, which is the normal case.
  //
  // It can only CONFIRM the chains: the server refuses at load a claim on a
  // face the world does not head, so a client may trust a claimed face is one
  // this world leads.
  primary_for?: string[]
  // Operator-authored text announcing this world ("DRAFT — not in force").
  // Empty/absent means announce nothing — a language world needs no banner.
  // Gates ONLY the announcement: a non-default world is read-only regardless.
  banner?: string
  // Whether this caller may select the world via `?world=`.
  //
  // A UI HINT about SELECTION, never a boundary: the server re-checks the
  // grant on every request, and a denial is served as an EMPTY RESULT rather
  // than a 403 — deliberately, so it stays indistinguishable from a world
  // holding nothing readable. That is exactly why a client should respect
  // this flag: ignoring it means offering a world that silently returns
  // nothing, which reads as "nothing is published yet".
  //
  // Never omitted by the server (no `omitempty`), so `false` is
  // distinguishable from a server too old to compute it.
  readable: boolean
  // Marks the implicit default world. Spelled as a flag so a client need not
  // hardcode the reserved name.
  default?: boolean
}

export interface EntityType {
  label: string
  label_plural?: string
  plural?: string
  description?: string
  id_type?: 'short' | 'sequential' | 'manual'
  id_prefix?: string
  id_prefixes?: string[]
  properties: Record<string, PropertyDef>
  default_sort?: SortSpec[]
  color?: string
  border_color?: string
  // Declared content-state coordinates for this type, keyed by the face
  // name. Absent for the common faceless type. Mirrors v1.EntityType.
  //
  // SCHEMA, not data: which faces the TYPE declares, never which faces a
  // given entity holds. `_faces` on an entity response answers that.
  faces?: Record<string, FaceInfo>
  // Which declared face the bare id addresses, mirroring `bare_face:` in
  // the schema. Empty when the type declares no faces, or names none.
  bare_face?: string
}

// FaceInfo mirrors v1.FaceDef — one declared content state of a type.
//
// Which face the bare id addresses is NOT here: it is `bare_face` on the
// entity type, because it is one fact about the type rather than a flag each
// face could contradict.
export interface FaceInfo {
  // The operator's display text for this face ("Nederlands" for `nl`).
  // Absent means fall back to the face name, which is itself
  // operator-authored config and so an honest, if terse, display string.
  label?: string
}

export interface PropertyDef {
  type: 'string' | 'date' | 'datetime' | 'integer' | 'boolean' | 'enum' | 'file' | 'rrule'
  required?: boolean
  values?: string[]
  // Optional display labels keyed by enum value. Display-only: the stored/
  // submitted value stays the raw value; labels never become wire identities.
  labels?: Record<string, string>
  default?: string
  description?: string
  format?: string
  list?: boolean
  // For `file` properties: maximum attachments (default 1). Above 1 the
  // property holds several files and the widget switches to add-mode.
  max?: number
}

export interface RelationType {
  label: string
  description?: string
  from: string[]
  to: string[]
  inverse?: InverseDef
  symmetric?: boolean
  min_outgoing?: number
  max_outgoing?: number
  min_incoming?: number
  max_incoming?: number
  properties?: Record<string, PropertyDef>
  orderable?: RelationOrderable
}

export interface RelationOrderable {
  outgoing?: boolean
  incoming?: boolean
}

// Reserved property names that hold managed ordering values on a relation.
// These match the names the backend writes; the frontend never spells
// other strings for ordering.
export const ORDER_PROPERTY_OUT = '_order_out'
export const ORDER_PROPERTY_IN = '_order_in'

export interface InverseDef {
  id: string
  label?: string
}

export interface CustomType {
  values: string[]
  // Optional display labels keyed by value. Display-only; see PropertyDef.labels.
  labels?: Record<string, string>
  default?: string
}

export interface SortSpec {
  property: string
  direction: 'asc' | 'desc'
}

export interface Template {
  name: string
  properties: Record<string, unknown>
  content: string
  relations: TemplateRelation[]
}

export interface TemplateRelation {
  relation: string
  target: string
}
