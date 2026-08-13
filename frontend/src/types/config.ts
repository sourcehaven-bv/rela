import type { SortSpec } from './schema'
import type { ScriptError } from './scriptError'

export interface ResolvedPalette {
  light: Record<string, string>
  dark?: Record<string, string>
  darkDisabled?: boolean
}

export interface Config {
  app: AppConfig
  /** Deployment description for the global "About" help (TKT-DUQBD0). */
  about_description?: string
  styles?: Record<string, Record<string, string>>
  palette?: ResolvedPalette
  forms: Record<string, FormConfig>
  lists: Record<string, ListConfig>
  views: Record<string, ViewConfig>
  kanbans: Record<string, KanbanConfig>
  dashboard?: DashboardConfig
  actions?: Record<string, ActionConfig>
  navigation: NavigationEntry[]
  documents?: Record<string, DocumentConfig>
  apps?: Record<string, AppEntry>
  /**
   * Operator-declared priority tiers for next-action suggestions, so the UI
   * can label a band rather than echo a raw id. The SOURCES are deliberately
   * absent: a suggestion arrives fully resolved from /_next_action, and
   * serving the rules would invite a client-side re-implementation of the
   * engine (same reasoning as "no useACL() composable").
   */
  next_action_bands?: NextActionBand[]
}

/** A custom app surfaced in the SPA (HTML fetched from /api/v1/_apps/{id}). */
export interface AppEntry {
  title?: string
  label?: string
  description?: string
}

export interface ActionConfig {
  label?: string
  key?: string
  confirm?: boolean
  set?: Record<string, string>
  script?: string
  description?: string
  params?: Record<string, string>
}

export interface AppConfig {
  name: string
  description?: string
  /**
   * Base URL of the configured PlantUML rendering server, or absent/empty
   * when PlantUML rendering is disabled. A non-empty value is the on switch
   * for ```plantuml diagram rendering (see renderPlantUMLDiagrams). The key
   * is snake_case to match the server's JSON tag verbatim — there is no
   * client-side casing transform.
   */
  plantuml_server_url?: string
}

export interface FormConfig {
  entity: string
  title?: string
  description?: string
  mode?: 'edit' | string
  body?: boolean
  /**
   * Wizard layout. Mutually exclusive with fields/relations (the server
   * rejects both). When present the form renders one step at a time with
   * next/back navigation instead of a single page.
   */
  steps?: FormStep[]
  fields?: FormField[]
  relations?: FormRelation[]
}

export interface FormStep {
  title: string
  description?: string
  /** Condition expression; the step is hidden/skipped when it evaluates false. */
  visible_when?: string
  fields?: FormFieldOrRelation[]
  relations?: FormFieldOrRelation[]
}

/**
 * What happens to a field's STORED value when its `visible_when` turns false
 * (BUG-FB0LN8). Hiding is presentation; the default does not touch data.
 *
 * - `no` (default) — keep the value; hide/reveal is lossless.
 * - `yes`          — clear it when the branch hides.
 *
 * A third value, `confirm` (ask before clearing, undo the triggering change on
 * decline), is deliberately absent. It needs the form to separate "proposed"
 * from "committed" — today an edit mutates form state and arms the autosave in
 * one step, so a decline must reconstruct state after the fact. The backend
 * rejects `confirm` at config-validation time until that refactor lands.
 */
export type ClearWhenHidden = 'no' | 'yes'

export interface FormField {
  property?: string
  widget?: string
  label?: string
  placeholder?: string
  help?: string
  default?: unknown
  readonly?: boolean
  hidden?: boolean
  /** Condition expression; the field is hidden when it evaluates false. */
  visible_when?: string
  /** Condition expression; the field is required only when it evaluates true. */
  required_when?: string
  /** Fate of the stored value when `visible_when` turns false. Default `no`. */
  clear_when_hidden?: ClearWhenHidden
  /**
   * Width on the 12-column form grid (TKT-5V8704). Absent means full width —
   * one field per row, the default. Same semantics as a view section field's
   * span, so an author learns the model once.
   */
  span?: number
}

export interface RelationProperty {
  property: string
  label?: string
  required?: boolean
}

export interface FormRelation {
  relation: string
  direction?: 'outgoing' | 'incoming'
  target_type?: string
  label?: string
  required?: boolean
  widget?: string
  properties?: RelationProperty[]
  /** Condition expression; the relation widget is hidden when it evaluates false. */
  visible_when?: string
}

// Unified type for form fields that can be either property fields or relation fields
export interface FormFieldOrRelation {
  // Property field props
  property?: string
  placeholder?: string
  help?: string
  default?: unknown
  readonly?: boolean
  hidden?: boolean
  transitions?: Record<string, string[]>
  // Relation field props
  relation?: string
  direction?: 'outgoing' | 'incoming'
  target_type?: string
  required?: boolean
  properties?: RelationProperty[]
  // Common props
  label?: string
  widget?: string
  /**
   * Width on the 12-column form grid; absent = full width. See FormField.
   *
   * Only meaningful on the FIELD half of this union. A relation entry renders
   * via RelationCards / RelationPicker, which do not read it — those widgets
   * (a card list, a searchable multi-select) have a natural minimum width that
   * a narrow column would break. The config validator rejects a span on a
   * relation so an author is told rather than left wondering.
   */
  span?: number
  // Wizard conditions (see FormField / FormRelation)
  visible_when?: string
  required_when?: string
  /**
   * Fate of the stored value when `visible_when` turns false. Default `no`.
   * Property fields only — a relation's hidden-branch handling is the separate
   * `pruneWizardHiddenRelations` mechanism, and steps have no such key (a step
   * hiding is just "all its fields hid", each honoring its own setting).
   */
  clear_when_hidden?: ClearWhenHidden
}

export interface ListConfig {
  entity: string
  title?: string
  /** Markdown rendered above the list. `description` is a fallback alias. */
  header?: string
  /** Markdown rendered below the list. */
  footer?: string
  /** Fallback for `header`; the previously-unused field, used when `header` is unset. */
  description?: string
  columns: ListColumn[]
  filters?: ListFilter[]
  filter_controls?: FilterControl[]
  default_sort?: SortSpec[]
  create_form?: string
  edit_form?: string
  page_size?: number
  actions?: string[]
}

/**
 * Resolve the raw markdown for a view's top info region. `header` is canonical.
 * Returns '' when unset (a blank/whitespace-only value counts as unset). The
 * caller passes the result through renderMarkdown.
 *
 * `description` is an OPT-IN legacy alias, enabled per call site via
 * `allowDescriptionAlias`. Only lists pass it: there the field predated this
 * feature and was never rendered, so adopting it as an alias avoided rewriting
 * every existing config. Kanban deliberately has no such fallback (TKT-6S331G).
 *
 * The opt-in is a flag rather than a type constraint because TypeScript erases
 * at runtime: a structural parameter would happily read `description` off any
 * object carrying one, and these configs are parsed from the /_config response.
 * Relying on `KanbanConfig` merely lacking the field would make the policy a
 * comment that a later Go `Kanban.Description` field could silently void.
 */
export function viewHeaderMarkdown(
  view: { header?: string; description?: string } | undefined,
  opts: { allowDescriptionAlias?: boolean } = {}
): string {
  const alias = opts.allowDescriptionAlias ? view?.description?.trim() : ''
  return view?.header?.trim() || alias || ''
}

/** Resolve the raw markdown for a view's bottom info region ('' when unset). */
export function viewFooterMarkdown(view: { footer?: string } | undefined): string {
  return view?.footer?.trim() || ''
}

// Helper to get edit form for an entity type
export function getEditFormId(
  schemaStore: { forms: Map<string, FormConfig> },
  entityType: string
): string | undefined {
  for (const [formId, config] of schemaStore.forms) {
    if (config.entity === entityType && config.mode === 'edit') {
      return formId
    }
  }
  // Fallback to any form for this entity type
  for (const [formId, config] of schemaStore.forms) {
    if (config.entity === entityType) {
      return formId
    }
  }
  return undefined
}

export interface FilterControl {
  property?: string
  relation?: string
  // For relation filters: which edge direction the filter follows.
  // `outgoing` (default) pulls option candidates from the relation's `to[*]`
  // types; `incoming` from `from[*]`. Mirrors ListColumn.direction.
  direction?: 'outgoing' | 'incoming'
  label?: string
}

export interface ListColumn {
  property?: string
  relation?: string
  direction?: 'outgoing' | 'incoming'
  label?: string
  sortable?: boolean
  link?: string
  width?: string
}

export interface ListFilter {
  property: string
  operator?: string
  value?: string
  label?: string
  operators?: string[]
}

// SortSpec is imported from schema.ts

export interface ViewConfig {
  entity: string
  title?: string
  sections: ViewSection[]
}

export interface ViewSection {
  type: 'properties' | 'relations' | 'content' | 'custom'
  title?: string
  properties?: string[]
  relations?: string[]
}

export interface KanbanConfig {
  entity: string
  title?: string
  /** Markdown rendered above the board. */
  header?: string
  /** Markdown rendered below the board. */
  footer?: string
  // Deliberately no `description`: unlike ListConfig, kanban has no legacy field
  // to adopt as a header alias. Enforced at the call site, which does NOT pass
  // viewHeaderMarkdown's `allowDescriptionAlias` — not by this field's absence,
  // which types alone cannot guarantee at runtime.
  column_property: string
  columns?: KanbanColumn[]
  swimlane_property?: string
  swimlanes?: KanbanSwimlane[]
  card: KanbanCard
  edit_form?: string
  create_form?: string
  filters?: Array<{ property: string; operator: string; value: string }>
  filter_controls?: FilterControl[]
}

export interface KanbanColumn {
  value: string
  label?: string
  color?: string
  /**
   * Name of an icon from the shared registry (see utils/icons.ts), rendered
   * beside the label. A NAME, never a glyph: an emoji written into `label`
   * renders verbatim, and the SPA never parses one back out of label text.
   */
  icon?: string
}

export interface KanbanSwimlane {
  value: string
  label?: string
  /** Icon name; see KanbanColumn.icon. */
  icon?: string
}

export interface KanbanCardField {
  property?: string
  relation?: string
  direction?: 'outgoing' | 'incoming'
  label?: string
}

export interface KanbanCard {
  title: string
  subtitle?: string
  fields?: KanbanCardField[]
}

export interface DashboardConfig {
  title?: string
  description?: string
  cards: DashboardCard[]
}

export interface DashboardCard {
  title: string
  query: string
  display: 'count' | 'breakdown' | 'table'
  group_by?: string
  columns?: Array<{ property?: string; relation?: string; label?: string; link?: string }>
  sort?: Array<{ property: string; direction: 'asc' | 'desc' }>
  limit?: number
  /**
   * Optional ACL permission gating this card's visibility (TKT-53KICM).
   *
   * The SPA does not act on this: the server already omits cards the principal
   * cannot use from `/_dashboard`. It appears here only because `/_config`
   * serves the `dashboard:` block verbatim to everyone, so the type has to
   * describe what arrives. Never filter on it client-side — the SPA reads
   * booleans the server computed, it does not evaluate ACL.
   */
  permission?: string
}

/**
 * The per-principal dashboard payload from `/_dashboard`.
 *
 * Distinct from {@link DashboardConfig}, which is the verbatim config block on
 * `/_config`. `cards` is always an array — a project with no dashboard, an
 * empty `cards:`, and an all-filtered dashboard all arrive as `[]`.
 */
export interface DashboardResponse {
  title?: string
  description?: string
  cards: DashboardCard[]
}

/**
 * One priority tier, declared by the operator. List order IS priority order.
 * Served by /_config so the UI can label a band rather than echo a raw id.
 */
/**
 * How loudly a band interrupts. A closed vocabulary rather than styling
 * knobs: the operator declares the volume, the UI decides what that looks
 * like. Ordered by how much they interrupt.
 */
export type NextActionProminence = 'banner' | 'card' | 'inline' | 'whisper'

export interface NextActionBand {
  id: string
  label?: string
  /** Defaults to 'card' when unset. */
  prominence?: NextActionProminence
}

/**
 * One affordance on a suggestion: a discriminated union where exactly one of
 * action/set/navigate/snooze/dismiss/acknowledge is set. The server validates
 * that invariant, so the UI can switch on whichever field is present.
 */
export interface NextActionOffer {
  label?: string
  action?: string
  set?: Record<string, string>
  confirm?: boolean
  navigate?: string
  snooze?: string[]
  dismiss?: boolean
  acknowledge?: boolean
}

/**
 * The one suggestion to show. Arrives FULLY RESOLVED — message already
 * interpolated, affordances attached — so the UI renders it rather than
 * re-deriving anything. The rules themselves are deliberately not served.
 */
export interface NextActionSuggestion {
  source: string
  band: string
  entity_id?: string
  /**
   * Opaque key component from the source's key_props. Echo it back verbatim
   * on feedback — it is part of the suggestion key, so omitting it stores a
   * snooze under a key the server never checks.
   */
  variant?: string
  message: string
  actions?: NextActionOffer[]
}

export interface NextActionResponse {
  suggestion: NextActionSuggestion | null
}

/** How a user answered a suggestion. */
export type NextActionFeedbackKind = 'snooze' | 'dismiss' | 'mute' | 'unmute' | 'shown'

export interface AnalyzeIssue {
  entityId: string
  entityType: string
  /** Optional headline shown when the row has no entity (e.g. validation rule name). */
  title?: string
  message: string
  severity: 'error' | 'warning'
  checkType: string
  /**
   * Optional structured specifics about why the issue fired, beyond the
   * flat message. For content required-headers violations it lists the
   * missing exact headers. Absent on rows with no structured detail;
   * the message cell reveals it in an expandable detail row.
   */
  detail?: string[]
  /**
   * Present only on validation script-error rows. Carries the same
   * envelope as the action surface so the UI can branch: rows with
   * scriptError open ScriptErrorDialog instead of navigating.
   */
  scriptError?: ScriptError
}

export interface AnalyzeResult {
  errors: number
  warnings: number
  issues: AnalyzeIssue[]
  byCheck: Record<string, number>

  /**
   * Names of checks that found MORE issues than they returned. Each check
   * is capped server-side (TKT-1ESTYJ), so `byCheck` for a listed check is
   * the cap, not the true total.
   *
   * Omitted by the server when nothing was truncated, so it is optional —
   * and older servers never send it at all.
   */
  truncatedChecks?: string[]
}

export interface NavigationEntry {
  // Direct item fields
  label?: string
  list?: string
  dashboard?: boolean
  kanban?: string
  search?: boolean
  settings?: boolean
  action?: string
  /** Names a standalone document (one configured without an entity_type). */
  document?: string
  /** Global named permission required for this entry to appear in the sidebar.
   *  The SPA does not act on it — the server already omits filtered entries
   *  from /_sidebar, which is what the menu is built from. Present here only
   *  because /_config serves the navigation tree verbatim. */
  permission?: string
  icon?: string
  // Group fields
  group?: string
  collapsed?: boolean
  items?: NavigationEntry[]
}

// Sidebar API types (denormalized navigation with counts)
export interface SidebarItem {
  label: string
  href?: string
  icon?: string
  count?: number
  action?: string
}

export interface SidebarGroup {
  group?: string
  collapsed?: boolean
  items: SidebarItem[]
}

export interface SidebarData {
  app: AppConfig
  navigation: SidebarGroup[]
  /** URL of the user-uploaded sidebar logo (with cache-busting query
   *  parameter), or null/undefined when no logo is set. */
  logoUrl?: string | null
  /**
   * Entity type → form id for inline creation from a relation field.
   * A type is present only when the principal may create it AND a create
   * form resolves for it, so presence alone is the affordance — no
   * client-side permission arithmetic, and no reimplementation of the
   * server's form-resolution ordering.
   *
   * Rides on the sidebar because it is the one boot-time payload that is
   * principal-scoped (`_config` is pinned principal-independent;
   * `_schema` is a pure metamodel projection).
   *
   * A UI hint only: the create POST re-authorizes.
   */
  inline_create?: Record<string, string>
}

// Document config, mirroring the Go `dataentryconfig.DocumentConfig` that
// /_config serves verbatim. Exactly one of `command` / `script` is set
// (enforced server-side).
export interface DocumentConfig {
  title?: string
  /** Entity type this document applies to (for frontend filtering).
   *  ABSENT means a standalone document: it renders at /document/:name with
   *  no entry entity, rather than being attached to one. */
  entity_type?: string
  command?: string
  script?: string
  timeout?: number
  /** Global named permission required to render this document, if any.
   *  Present here because the config is not secret; the SPA does not act on
   *  it — the render endpoint enforces it and returns 403. */
  permission?: string
  edit?: DocumentEdit
}

// DocumentEdit configures the Edit button on the full-page document view.
// Both fields are required when the parent block is present (validated server-side).
export interface DocumentEdit {
  form: string
  label: string
}

// Response from document render API
export interface DocumentRenderResponse {
  html: string
  cached: boolean
  entity_ids: string[] // IDs of entities involved in this document (for SSE filtering)
}

// Command available for a page context
export interface Command {
  id: string
  label: string
  confirm?: string
  context: 'entity' | 'list' | 'view' | 'global'
  auto_open?: boolean
}
