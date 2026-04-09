import type { SortSpec } from './schema'

export interface ResolvedPalette {
  light: Record<string, string>
  dark?: Record<string, string>
  darkDisabled?: boolean
}

export interface Config {
  app: AppConfig
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
}

export interface FormConfig {
  entity: string
  title?: string
  description?: string
  mode?: 'edit' | string
  body?: boolean
  sections?: FormSection[]
  fields?: FormField[]
  relations?: FormRelation[]
}

export interface FormSection {
  title?: string
  description?: string
  fields: FormFieldOrRelation[]
}

export interface FormField {
  property?: string
  widget?: string
  label?: string
  placeholder?: string
  help?: string
  default?: unknown
  readonly?: boolean
  hidden?: boolean
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
  allow_create?: boolean
  create_form?: string
  properties?: RelationProperty[]
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
  allow_create?: boolean
  create_form?: string
  properties?: RelationProperty[]
  // Common props
  label?: string
  widget?: string
}

export interface ListConfig {
  entity: string
  title?: string
  description?: string
  columns: ListColumn[]
  filters?: ListFilter[]
  filter_controls?: FilterControl[]
  default_sort?: SortSpec[]
  create_form?: string
  edit_form?: string
  detail_view?: string
  page_size?: number
  actions?: string[]
}

// Helper to get edit form for an entity type
export function getEditFormId(schemaStore: { forms: Map<string, FormConfig> }, entityType: string): string | undefined {
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
}

export interface KanbanSwimlane {
  value: string
  label?: string
}

export interface KanbanCard {
  title: string
  subtitle?: string
  fields?: Array<{ property?: string; relation?: string }>
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
}

export interface AnalyzeIssue {
  entityId: string
  entityType: string
  message: string
  severity: 'error' | 'warning'
  checkType: string
}

export interface AnalyzeResult {
  errors: number
  warnings: number
  issues: AnalyzeIssue[]
  byCheck: Record<string, number>
}

export interface NavigationEntry {
  // Direct item fields
  label?: string
  list?: string
  dashboard?: boolean
  graph?: boolean
  kanban?: string
  action?: string
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
}

// Document config for external rendering via shell commands
export interface DocumentConfig {
  title?: string
  entity_type?: string // Entity type this document applies to (for frontend filtering)
  view?: string // View name from views.yaml for cache hashing (optional)
  command: string
  timeout?: number
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
