import { api } from './client'
import { getPlural } from './entities'

/** One entity whose body mentions the addressed one, at the face the world serves. */
export interface MentionSource {
  id: string
  /** The served face; absent for a type without faces. */
  face?: string
  type: string
  /** Display title; the id when the title property is hidden from the reader. */
  title: string
  inaccessible?: boolean
  inaccessible_reason?: string
}

export interface MentionedByPage {
  items: MentionSource[]
  /** Pass back as `cursor` for the next page. Absent on the last page. */
  next_cursor?: string
  /** The server stopped scanning before reaching every source. */
  truncated?: boolean
}

/**
 * getMentionedBy pages the entities whose body mentions `entityId` as an
 * `` `id` `` code span (TKT-Q6AKUN). `entityId` is the address the page shows;
 * `world` is the page's world, so each source is listed at the face it serves.
 */
export async function getMentionedBy(
  type: string,
  entityId: string,
  opts: { cursor?: string; limit?: number; world?: string } = {},
  signal?: AbortSignal
): Promise<MentionedByPage> {
  const params: Record<string, unknown> = {}
  if (opts.cursor) params.cursor = opts.cursor
  if (opts.limit) params.limit = opts.limit
  if (opts.world) params.world = opts.world
  return api.get<MentionedByPage>(
    `/${getPlural(type)}/${encodeURIComponent(entityId)}/mentioned-by`,
    params,
    signal
  )
}
