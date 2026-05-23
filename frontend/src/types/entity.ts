export interface Entity {
  id: string
  type: string
  _title?: string
  properties: Record<string, unknown>
  content?: string
  relations?: Record<string, string[]>
  included?: Record<string, Entity>
  _self?: string
  // Per-resource verb-verdict map driven by the backend ACL. Keys are
  // verbs (phase 1: `update`, `delete`, `rename` per-item; `create`
  // on collection responses); values are booleans. Always present
  // on responses from the data-entry server. An empty map means the
  // principal has every verb denied — UI should hide all affordances.
  // See .ignored/action-affordances-design.md for the full contract.
  _actions?: Record<string, boolean>
  // Per-field write affordances on per-entity GET responses.
  // Sparse: only fields whose verdict deviates from default appear.
  // Hidden fields are omitted from `properties` AND from `_fields`.
  // Absent on list / mutation responses; present (possibly empty) on
  // per-entity GET (closed-world signal — empty means "evaluated, no
  // deviations"). See docs/data-entry/api-reference.md.
  _fields?: Record<string, FieldAffordance>
  // Per-relation-type affordances on per-entity GET responses. Same
  // sparse / closed-world semantics as _fields. Per-relation-type
  // uniform — per-link verdicts are predicate territory (deferred).
  _relations?: Record<string, RelationAffordance>
  inaccessible?: InaccessibleField[]
  // Soft-validation findings on mutation responses (DEC-HWZHA).
  // Present on PATCH/POST results; absent on GETs.
  warnings?: Warning[]
}

// FieldAffordance carries per-field write / option affordances on
// the wire. Sparse: `writable` undefined means default (writable);
// `options` lists only the false entries (allowed options are
// implicit via the metamodel).
export interface FieldAffordance {
  writable?: boolean
  options?: Record<string, boolean>
}

// RelationAffordance carries per-relation-type affordances on the
// wire. Same sparse semantics as FieldAffordance: `creatable` /
// `removable` undefined means default (true). `fields` is the
// per-meta-field writability map, also sparse.
export interface RelationAffordance {
  creatable?: boolean
  removable?: boolean
  fields?: Record<string, FieldAffordance>
}

// Warning is a soft validation finding returned alongside a successful
// mutation. Code matches the analyze_* finding code so UIs can
// de-duplicate. See docs/data-entry/api-reference.md for stable codes.
export interface Warning {
  code: string
  path: string
  detail: string
}

// JSON:API §9 resource identifier — the per-edge shape inside the
// unified PATCH's modern relations field. Used by the patch builder
// to emit edges with explicit type, meta, and (future) content.
export interface ResourceIdentifier {
  type: string
  id: string
  meta?: Record<string, unknown>
  meta_unset?: string[]
  content?: string
}

// Modern relations field shape for the unified PATCH body. Keys are
// relation names; each value's `data` is the desired set of edges.
// Sending `data: []` clears all edges of that type — see the
// data-loss footgun docs in docs/data-entry/api-reference.md.
export interface ModernRelationsField {
  [relationName: string]: { data: ResourceIdentifier[] }
}

// InaccessibleField marks a property whose value is known to exist but is
// unreadable by the holder of the entity (e.g. the file is git-crypt
// encrypted and the key is not present locally). The SPA renders such
// fields with a lock indicator instead of an editable widget.
export interface InaccessibleField {
  name: string
  reason: string
}

export interface CreateEntity {
  id?: string
  prefix?: string
  properties: Record<string, unknown>
  content?: string
  // Modern JSON:API §9 wrapper shape only. The legacy IDs-only form
  // (`Record<string, string[]>`) is no longer accepted on the wire.
  relations?: ModernRelationsField
}

export interface RelationEntry {
  id: string
  // type of the peer entity on the other end of the edge. Required for
  // the unified PATCH builder to emit JSON:API §9 resource identifiers
  // without consulting the schema. Backend started emitting this in
  // TKT-ZEKO4; older servers omit it.
  type: string
  direction?: 'outgoing' | 'incoming'
  meta?: Record<string, unknown>
  // Plumbing-only — no widget exposes per-edge body editing yet, but
  // the wire shape carries it so a future ticket can wire UI without
  // touching types again.
  content?: string
}

export interface ListResponse<T> {
  data: T[]
  meta: ListMeta
  included?: Record<string, T>
  // Collection-scope verb verdicts (phase 1: just `create`). Same
  // semantics as Entity._actions: absent = anonymous/pre-rollout
  // fallback; empty {} = all denied.
  _actions?: Record<string, boolean>
}

export interface ListMeta {
  total: number
  page: number
  per_page: number
  has_more: boolean
  next_cursor?: string
}

export interface ListParams {
  page?: number
  per_page?: number
  cursor?: string
  sort?: string
  fields?: string
  include?: string
  [key: `filter[${string}]`]: string | undefined
}

// Side panel types
export interface SidePanelField {
  label: string
  values?: string[]
  propType?: string
}

export interface SidePanelEntity {
  id: string
  title: string
  type: string
  editFormId?: string
  fields?: SidePanelField[]
  content?: string
  hasContent: boolean
}

// Add target for side panel (reusable)
export interface SidePanelAddTarget {
  entityType: string
  formId: string
  label: string
}

// Add button info for side panel
export interface SidePanelAddInfo {
  relation: string
  linkAs: 'from' | 'to'
  peerId: string
  targets: SidePanelAddTarget[]
}

// Link existing button info for side panel
export interface SidePanelLinkInfo {
  relation: string
  linkAs: 'from' | 'to'
  peerId: string
  entityTypes: string[]
}

export interface SidePanelSection {
  heading: string
  sectionId: string
  display: 'cards' | 'list' | 'properties'
  isEmpty: boolean
  emptyMessage?: string
  fields?: SidePanelField[]
  entities?: SidePanelEntity[]
  addInfo?: SidePanelAddInfo
  linkInfo?: SidePanelLinkInfo
}
