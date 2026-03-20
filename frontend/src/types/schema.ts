export interface Schema {
  entities: Record<string, EntityType>
  relations: Record<string, RelationType>
  types: Record<string, CustomType>
}

export interface EntityType {
  label: string
  label_plural?: string
  plural?: string
  description?: string
  id_type?: 'short' | 'sequential' | 'manual'
  id_prefix?: string
  properties: Record<string, PropertyDef>
  default_sort?: SortSpec[]
  color?: string
  border_color?: string
}

export interface PropertyDef {
  type: 'string' | 'date' | 'integer' | 'boolean' | 'enum' | 'file'
  required?: boolean
  values?: string[]
  default?: string
  description?: string
  format?: string
  list?: boolean
}

export interface RelationType {
  label: string
  description?: string
  from: string[]
  to: string[]
  inverse?: InverseDef
  symmetric?: boolean
  min_outgoing?: number
  max_outgoing?: number
  min_incoming?: number
  max_incoming?: number
}

export interface InverseDef {
  id: string
  label?: string
}

export interface CustomType {
  values: string[]
  default?: string
}

export interface SortSpec {
  property: string
  direction: 'asc' | 'desc'
}

export interface Template {
  name: string
  properties: Record<string, unknown>
  content: string
  relations: TemplateRelation[]
}

export interface TemplateRelation {
  relation: string
  target: string
}
