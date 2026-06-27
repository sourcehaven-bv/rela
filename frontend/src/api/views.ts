import { api } from './client'
import type { Entity, FieldAffordance } from '@/types'

// Field data for view sections
export interface ViewSectionField {
  // Raw property name (e.g. "title") — used to correlate with the entry's
  // inaccessible[] for tooltip reasons.
  property?: string
  label: string
  values?: string[]
  propType?: string
  // True when the underlying entity is git-crypt encrypted; PropertyDisplay
  // renders a lock indicator instead of the (absent) value.
  inaccessible?: boolean
}

// Entity data for view sections.
//
// `_props` and `_fields` (TKT-IHC7D) ship typed property values and a
// per-cell writability verdict for inline-edit hosts on cards/list
// view sections. Both are hidden-property-stripped; the consumer can
// assume:
//
//  - `keys(_props) ∩ hidden(e) == ∅`
//  - `keys(_fields) ∩ hidden(e) == ∅`
//  - `_fields` may have keys absent from `_props` when the property
//    has no stored value but a non-default verdict
//
// Both fields are absent on view sections that don't compute them
// (entry section, table rows).
export interface ViewEntity {
  id: string
  title: string
  type: string
  editFormId?: string
  fields?: ViewSectionField[]
  content?: string
  hasContent: boolean
  _props?: Record<string, unknown>
  _fields?: Record<string, FieldAffordance>
}

// Table cell data
export interface ViewCell {
  values: string[]
  propType?: string
  widget?: string
  link?: string
  entityId?: string
  entityType?: string
}

// Table row data
export interface ViewRow {
  entityId: string
  entityType: string
  editFormId?: string
  cells: ViewCell[]
  content?: string
}

// Column definition
export interface ViewColumn {
  property?: string
  label?: string
  relation?: string
  link?: string
}

// Group of rows/entities
export interface ViewGroup {
  groupName: string
  rows?: ViewRow[]
  entities?: ViewEntity[]
}

// View section with all display types
export interface ViewSection {
  heading: string
  sectionId: string
  display: 'properties' | 'content' | 'table' | 'cards' | 'list'
  isEmpty: boolean
  emptyMessage?: string
  fields?: ViewSectionField[]
  entities?: ViewEntity[]
  columns?: ViewColumn[]
  rows?: ViewRow[]
  groups?: ViewGroup[]
  isGrouped: boolean
  content?: string
  hasContent: boolean
}

// Mention is the resolved target of an entity-ID code span found inside
// any markdown body the response carries (entry content + section
// content). Mirrors the server-side `Mention` Go struct (TKT-747O); the
// SPA's `renderMarkdown` consumes this map to rewrite bare-ID code spans
// into titled in-app links. `inaccessible` flags targets whose display
// title is unreadable (e.g. git-crypt encrypted) so the renderer can
// show a lock affordance.
//
// `inaccessible_reason` carries the matching `entity.InaccessibleReason`
// value as a bare string. Today only `"git-crypt"` is produced; the SPA
// treats unknown reasons as opaque and falls back to a generic tooltip,
// so adding new reasons server-side never breaks the client.
export interface Mention {
  type: string
  title: string
  inaccessible?: boolean
  inaccessible_reason?: string
}

// Full view API response
export interface ViewResponse {
  entry: Entity
  sections: ViewSection[]
  mentions?: Record<string, Mention>
}

// Fetch executed view data for an entity. The backend looks up the
// configured ViewConfig by entry.type, or synthesizes a default when
// none is registered.
export async function fetchView(entityType: string, entityId: string): Promise<ViewResponse> {
  return api.get<ViewResponse>(`/_views/${entityType}/${entityId}`)
}
