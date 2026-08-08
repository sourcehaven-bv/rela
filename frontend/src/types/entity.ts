// Minimal shape a single-value entity-target selector needs: an id and an
// optional backend-supplied display title. Entity satisfies it structurally,
// so list rows can be passed straight through. Lives here so both the
// component and its callers/tests import it from one place.
export interface TargetCandidate {
  id: string
  _title?: string
}

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
  // A hidden field's VALUE is omitted from `properties`, and the field
  // carries a `{visible: false}` tombstone here so a client can tell
  // "redacted" from "unset" (BUG-FB0LN8). Read-only per-row surfaces
  // (view sections) omit hidden fields from both maps instead.
  // Absent on list / mutation responses; present (possibly empty) on
  // per-entity GET (closed-world signal — empty means "evaluated, no
  // deviations"). See docs/data-entry/api-reference.md.
  _fields?: Record<string, FieldAffordance>
  // Per-relation-type affordances on per-entity GET responses. Same
  // sparse / closed-world semantics as _fields. Per-relation-type
  // uniform — per-link verdicts are predicate territory (deferred).
  _relations?: Record<string, RelationAffordance>
  // Per-`file`-property attachment metadata on per-entity responses.
  // Keyed by property name; the value is always a LIST (a property may
  // hold several files when its metamodel `max` > 1, and a single file is
  // a 1-element list). Only properties that carry a file appear. Same
  // closed-world semantics as _fields/_relations: present (possibly empty)
  // on every per-entity response (GET, PATCH, POST, clone), absent on list
  // rows. The file widget reads this to render download links / previews.
  _attachments?: Record<string, AttachmentInfo[]>
  // Per-state-machine-field transition verdicts on per-entity GET
  // responses (TKT-3G93B8). Keyed by property name; the value lists the
  // outgoing transitions resolved for the requesting principal on this
  // entity. Only state-machine-typed fields appear — a plain enum field
  // has no key, and the SPA falls back to the ordinary enum control.
  // Absent entirely when the server wires no state machines (older server
  // or no policy). A UI hint, never authorization — the write path
  // re-enforces every transition (attempt-and-recover). See
  // docs/data-entry/api-reference.md.
  _transitions?: Record<string, TransitionOption[]>
  inaccessible?: InaccessibleField[]
  // Soft-validation findings on mutation responses (DEC-HWZHA).
  // Present on PATCH/POST results; absent on GETs.
  warnings?: Warning[]
}

// FieldAffordance carries per-field write / option affordances on
// the wire. Sparse: `writable` / `visible` undefined means the
// permissive default (writable, visible); `options` lists only the
// false entries (allowed options are implicit via the metamodel).
//
// `visible: false` is an explicit redaction tombstone (BUG-FB0LN8): the
// field is declared by the metamodel but its VALUE is withheld by
// field-level ACL, so it is also absent from `properties`. Render it
// read-only/redacted — never as an empty input, which would invite a
// write the server rejects. Do NOT infer redaction from a key's mere
// absence: absence is ambiguous (redacted, never set, or locally
// cleared), and guessing is what caused BUG-FB0LN8.
//
// A hidden field carries ONLY this tombstone; writability and option
// verdicts are suppressed server-side for values the caller can't read.
export interface FieldAffordance {
  writable?: boolean
  visible?: boolean
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

// TransitionOption is one resolved outgoing move of a state-machine field
// (TKT-3G93B8), the wire projection of the backend's TransitionVerdict.
// `to` is the target value; `label` is optional display text for the MOVE
// (the action, e.g. "Start progress") — when absent the UI falls back to the
// target value's display label. `guard` names the permission the move
// requires (empty when unguarded). `allowed` reports whether the requesting
// principal may perform it now. `reason` names the blocking gate when not
// allowed (`guard` | `precondition`); empty when allowed. The status control
// shows only allowed moves, so `reason` is advisory (tooltip), not a gate.
export interface TransitionOption {
  to: string
  label?: string
  guard?: string
  allowed: boolean
  reason?: string
}

// AttachmentInfo describes one file attached to a `file`-type property,
// as surfaced in Entity._attachments. `id` identifies the file within the
// property (its normalized name) and is used to build the per-file
// download/delete URL. `href` is the ACL-gated download URL (inherits the
// entity's read permission). `contentType` is inferred from the filename
// by the backend.
export interface AttachmentInfo {
  id: string
  filename: string
  size: number
  contentType: string
  href: string
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
