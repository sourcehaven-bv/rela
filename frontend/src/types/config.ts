import type { SortSpec } from './schema'

export interface Config {
  app: AppConfig
  forms: Record<string, FormConfig>
  lists: Record<string, ListConfig>
  views: Record<string, ViewConfig>
  kanbans: Record<string, KanbanConfig>
  navigation: NavigationEntry[]
}

export interface AppConfig {
  name: string
  description?: string
}

export interface FormConfig {
  entity: string
  title?: string
  sections?: FormSection[]
  fields?: FormField[]
}

export interface FormSection {
  title?: string
  description?: string
  fields: FormField[]
}

export interface FormField {
  property?: string
  relation?: string
  widget?: string
  label?: string
  placeholder?: string
  help?: string
  default?: unknown
  readonly?: boolean
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
  group_by: string
  columns: KanbanColumn[]
  card: KanbanCard
}

export interface KanbanColumn {
  value: string
  label?: string
  color?: string
}

export interface KanbanCard {
  title: string
  subtitle?: string
  badges?: string[]
}

export interface NavigationEntry {
  // Direct item fields
  label?: string
  list?: string
  dashboard?: boolean
  graph?: boolean
  kanban?: string
  icon?: string
  // Group fields
  group?: string
  collapsed?: boolean
  items?: NavigationEntry[]
}
