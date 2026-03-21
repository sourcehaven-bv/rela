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
  properties: Record<string, unknown>
  content?: string
  relations?: Record<string, string[]>
}

export interface ListResponse<T> {
  data: T[]
  meta: ListMeta
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
  value: string
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
