import { api } from './client'
import { getPlural } from './entities'

/** A registered export format from the metamodel `transforms:` registry. */
export interface TransformInfo {
  name: string
  produces: string
}

/** Fetch the export formats a client may offer (drives the "Export as" menu). */
export async function getTransforms(signal?: AbortSignal): Promise<TransformInfo[]> {
  return api.get<TransformInfo[]>('/_transforms', undefined, signal)
}

/**
 * Build the export URL for a single entity + transform. The browser navigates
 * to it so the hardened forced-download response (Content-Disposition:
 * attachment) drives a file save. Plural is resolved the same way attachment
 * URLs are, from the schema-store-populated plural registry.
 */
export function entityExportUrl(entityType: string, id: string, transform: string): string {
  const q = new URLSearchParams({ transform })
  return `/api/v1/${getPlural(entityType)}/${encodeURIComponent(id)}/_export?${q.toString()}`
}
