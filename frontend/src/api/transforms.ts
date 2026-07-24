import { api } from './client'
import { getPlural } from './entities'

/** A registered export format from the metamodel `transforms:` registry. */
export interface TransformInfo {
  name: string
  produces: string
}

// The registry is static per server config (it changes only on a metamodel
// reload), so one fetch per SPA session is enough — ExportMenu mounts on every
// entity/list navigation and must not re-request it each time. A failed fetch
// clears the cache so the next mount retries.
let transformsCache: Promise<TransformInfo[]> | null = null

/** Fetch the export formats a client may offer (drives the "Export as" menu). */
export async function getTransforms(signal?: AbortSignal): Promise<TransformInfo[]> {
  if (!transformsCache) {
    transformsCache = api.get<TransformInfo[]>('/_transforms', undefined, signal).catch((err) => {
      transformsCache = null
      throw err
    })
  }
  return transformsCache
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

/**
 * Build the export URL for a whole list view + transform. `extraParams` carries
 * the list's current filter/sort/search query so the export matches what the
 * user sees (the backend applies the same ACL + filter pipeline). Always passes
 * the list id so the export uses the view's configured columns.
 */
export function listExportUrl(
  entityType: string,
  listId: string,
  transform: string,
  extraParams?: URLSearchParams
): string {
  const q = new URLSearchParams(extraParams)
  q.set('transform', transform)
  q.set('list', listId)
  return `/api/v1/${getPlural(entityType)}/_export?${q.toString()}`
}
