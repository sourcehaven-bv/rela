import { api } from './client'

/** SankeyNode is one gated, redacted entity of the diagram. */
interface SankeyNode {
  id: string
  type: string
  /** The face the node was read at; absent for a faceless type. */
  face?: string
  title?: string
  /** Value of the type's `color_by` property. */
  tone?: string
  /** Value of the type's `order` property. */
  sort_key?: string
  /** Related entities the principal may read, drawn as dots beside the node. */
  pips?: SankeyPip[]
  /** Readable related entities beyond `pips`. */
  pips_more?: number
}

/** SankeyPip is one related entity drawn as a dot; it is not a diagram node. */
export interface SankeyPip {
  id: string
  type: string
  face?: string
  title?: string
  /** Value of the pips' `color_by` property. */
  tone?: string
}

/** SankeyLink is a relation between two nodes of the response. */
interface SankeyLink {
  source: string
  target: string
  relation: string
}

export interface SankeyResponse {
  nodes: SankeyNode[]
  links: SankeyLink[]
  /** True when the node cap cut the (ACL-filtered) list short. */
  truncated?: boolean
}

/** getSankey fetches the nodes and links of a configured sankey diagram. */
export async function getSankey(id: string): Promise<SankeyResponse> {
  return api.get<SankeyResponse>(`/_sankeys/${encodeURIComponent(id)}`)
}
