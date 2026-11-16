import { api } from './client'

/** `GET /_page_status`: the `done_when` verdict of each step of a steps page. */
export interface PageStatusResponse {
  /** Keyed by tab id. A step without `done_when`, or one this reader cannot see, is absent. */
  steps: Record<string, { done: boolean }>
}

/**
 * Fetches the per-principal done marks of a steps page (TKT-MWZF0UO).
 *
 * Separate from the sidebar for the reason `/_nav_status` is: the page's
 * structure is the same for every principal, and a verdict is about data,
 * so it depends on who is asking. On an entity page, `entity` is the page's
 * entity and each step is judged over the rows it shows for that entity. A
 * world-bound page passes its world, so the rows judged are the faces the
 * page shows.
 */
export async function getPageStatus(
  page: string,
  entity?: string,
  world?: string
): Promise<PageStatusResponse> {
  const params: Record<string, string> = { page }
  if (entity) params.entity = entity
  if (world) params.world = world
  return api.get<PageStatusResponse>('/_page_status', params)
}
