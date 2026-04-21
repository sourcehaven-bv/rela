import { api } from './client'
import type { Entity } from '@/types'

// Field data for view sections
export interface ViewSectionField {
  label: string
  values?: string[]
  propType?: string
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

// Add button target
export interface ViewAddTarget {
  entityType: string
  formId: string
  label: string
}

// Add button info
export interface ViewAddInfo {
  relation: string
  linkAs: 'from' | 'to'
  peerId: string
  targets: ViewAddTarget[]
}

// Link existing button info
export interface ViewLinkInfo {
  relation: string
  linkAs: 'from' | 'to'
  peerId: string
  entityTypes: string[]
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
  addInfo?: ViewAddInfo
  linkInfo?: ViewLinkInfo
}

// Full view API response
export interface ViewResponse {
  entry: Entity
  sections: ViewSection[]
}

// Fetch executed view data
export async function fetchView(viewId: string, entityId: string): Promise<ViewResponse> {
  return api.get<ViewResponse>(`/_views/${viewId}/${entityId}`)
}
