export interface Entity {
  id: string
  type: string
  properties: Record<string, unknown>
  content?: string
  relations?: Record<string, string[]>
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
