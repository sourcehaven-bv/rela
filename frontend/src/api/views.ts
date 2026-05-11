import { api } from './client'
import type { Entity } from '@/types'

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

// Entity data for view sections
export interface ViewEntity {
  id: string
  title: string
  type: string
  editFormId?: string
  fields?: ViewSectionField[]
  content?: string
  hasContent: boolean
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

// Full view API response
export interface ViewResponse {
  entry: Entity
  sections: ViewSection[]
}

// Fetch executed view data for an entity. The backend looks up the
// configured ViewConfig by entry.type, or synthesizes a default when
// none is registered.
export async function fetchView(entityType: string, entityId: string): Promise<ViewResponse> {
  return api.get<ViewResponse>(`/_views/${entityType}/${entityId}`)
}
