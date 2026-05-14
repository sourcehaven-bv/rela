export interface Entity {
  id: string
  type: string
  _title?: string
  properties: Record<string, unknown>
  content?: string
  relations?: Record<string, string[]>
  included?: Record<string, Entity>
  _self?: string
  _actions?: EntityActions
  inaccessible?: InaccessibleField[]
  // Soft-validation findings on mutation responses (DEC-HWZHA).
  // Present on PATCH/POST results; absent on GETs.
  warnings?: Warning[]
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

export interface EntityActions {
  delete?: {
    allowed: boolean
    reason?: string
  }
  transitions?: string[]
}

export interface CreateEntity {
  id?: string
  prefix?: string
  properties: Record<string, unknown>
  content?: string
  relations?: Record<string, string[]>
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
