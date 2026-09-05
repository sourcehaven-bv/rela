import { api } from './client'
import type { Entity, EntityWorld, FieldAffordance } from '@/types'

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
  // Authored width on the 12-column property grid (TKT-5V8704), carried from
  // the view/form config. Absent or 0 means full width — the default, and what
  // every auto-generated view emits. See utils/fieldSpan.ts.
  span?: number
  // Server-resolved render mode (TKT-HOIX1). 'display' (the default) renders a
  // view-oriented value; 'input' opts the field into inline edit. The
  // section→field inheritance is resolved server-side, so this is already the
  // effective value — never re-derive it here.
  //
  // Opting in to 'input' does NOT grant editability: the ACL verdict in
  // `_fields` still decides, so 'input' on a read-only field renders display.
  render?: 'input' | 'display'
  // Config's widget override (TKT-3R7RF3): the registered widget name to use
  // instead of the type-derived default. Absent means "use the default",
  // i.e. defaultWidgetFor's dispatch — the server does NOT resolve it.
  //
  // Honoured only when the property is in the schema (the 'schema' arm of
  // SectionEditField). A field the metamodel doesn't declare routes through
  // resolveFromHint, which takes no name, and the server could not have
  // type-checked the override for it either (RR-2GBB0V) — config load emits a
  // warning for that case rather than silently doing nothing.
  widget?: string
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
  // Which face this world served for THIS entity, and which rule chose it
  // (TKT-WRLDAPI item 4b). Per-neighbour: each collection entity resolves
  // through the world independently, so one section can mix `chain` and
  // `fallback-default` entries.
  //
  // Absent under the default world — there is no provenance to report when no
  // resolution was applied. Under a non-default world it is present, and
  // `via: 'fallback-default'` is the case worth surfacing: the reader is
  // seeing a SUBSTITUTE face, not the one the world asked for.
  _world?: EntityWorld
  // The address of THIS row, face included — the same contract as
  // `Entity._self`. A row's Edit button and inline edit write to it, never to
  // the bare id: under a world the row is a neighbour's RESOLVED face, and the
  // bare id would edit a state the page is not showing. See utils/entityRef.
  _self?: string
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
  // The row's address, face included. See ViewEntity._self.
  _self?: string
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
  /**
   * The entity EXISTS but has no face in the requested world — the ordinary
   * state of an unpublished draft under a filtering world, not an error.
   *
   * The server answers 200 rather than a 4xx precisely so this page has
   * something to offer: `entry` carries the face that DOES exist (the default
   * face, which is where writes land), with its `_faces` and copy offers. A
   * 4xx would be swallowed by the error path and the user who just created
   * something would be told it does not exist — which is what happened, and
   * why the demo accumulated duplicate policies.
   *
   * `sections` is empty here: a section is a traversal through the requested
   * world, and that world resolves nothing for this entity.
   */
  _world_absent?: boolean
}

// Fetch executed view data for an entity. The backend looks up the
// configured ViewConfig by entry.type, or synthesizes a default when
// none is registered.
/**
 * fetchView reads the entity view — the detail page's data.
 *
 * `world` selects which FACE the entry resolves to, and resolves every
 * collection entity through the same world (TKT-WRLDAPI item 4b). Pass
 * `undefined` for the default world so the param is omitted rather than sent
 * empty; `useWorld().worldParam` is already shaped for that.
 *
 * Each collection entity carries `_world` provenance under a non-default
 * world — which face was served and which rule chose it. That distinction is
 * not decoration: "the Dutch page" and "the English page, because no Dutch
 * page exists" arrive byte-identically, and only `_world.via` separates them.
 */
export async function fetchView(
  entityType: string,
  entityId: string,
  world?: string,
): Promise<ViewResponse> {
  const params = world ? { world } : undefined
  return api.get<ViewResponse>(`/_views/${entityType}/${entityId}`, params)
}
