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
  columns: ListColumn[]
  filters?: ListFilter[]
  default_sort?: SortSpec[]
  page_size?: number
}

export interface ListColumn {
  property?: string
  relation?: string
  direction?: 'outgoing' | 'incoming'
  label?: string
  sortable?: boolean
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
  type: 'link' | 'section' | 'divider'
  label?: string
  icon?: string
  href?: string
  items?: NavigationEntry[]
}
