export interface GraphNode {
  id: string
  type: string
  title: string
  properties: Record<string, string>
}

export interface GraphEdge {
  source: string
  target: string
  type: string
}

export interface GraphEntityType {
  type: string
  label: string
  color: string
  count: number
}

export interface GraphRelationType {
  type: string
  label: string
  count: number
}

export interface GraphData {
  nodes: GraphNode[]
  edges: GraphEdge[]
  entityTypes: GraphEntityType[]
  relationTypes: GraphRelationType[]
}

export async function getGraphData(mode: 'content' | 'metamodel' = 'content'): Promise<GraphData> {
  // Note: graph-data endpoint is at /api/graph-data, not /api/v1/graph-data
  const response = await fetch(`/api/graph-data?mode=${mode}`)
  if (!response.ok) {
    throw new Error('Failed to load graph data')
  }
  return response.json()
}
