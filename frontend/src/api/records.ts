import { api } from './client'

/**
 * Records (docs/records.md): append-only rows owned by an entity and stored
 * outside the graph. A row is readable when its owner is.
 */

/** A declared record type, as `GET /_records` lists it. */
export interface RecordTypeInfo {
  key: string
  label: string
  /** The entity type every row attaches to. */
  owner: string
}

/**
 * One folded bucket of `GET /_records/{type}/_aggregate`. The statistics are
 * decimal strings, so a decimal field never passes through a float on the
 * server; `start` is absent when the query has no bucket.
 */
export interface RecordPoint {
  owner?: string
  start?: string
  count: number
  sum?: string
  min?: string
  max?: string
  avg?: string
  latest?: string
}

/** Lists the declared record types. */
export async function getRecordTypes(): Promise<RecordTypeInfo[]> {
  const res = await api.get<{ types: RecordTypeInfo[] }>('/_records')
  return res.types ?? []
}

export interface AggregateRecordsQuery {
  owners: string[]
  field?: string
  bucket: 'hour' | 'day' | 'week' | 'month'
  /** RFC 3339; absent means unbounded. */
  from?: string
  perOwner?: boolean
}

/**
 * Folds the rows of `type` into points. The server folds only the owners this
 * reader may read: a hidden owner among several is left out of the points,
 * and a single hidden owner rejects with a 404, as a missing one does.
 */
export async function aggregateRecords(
  type: string,
  q: AggregateRecordsQuery,
  signal?: AbortSignal
): Promise<RecordPoint[]> {
  // `owner` repeats, which the client's default array format would write as
  // `owner[]`.
  const params = new URLSearchParams()
  for (const o of q.owners) params.append('owner', o)
  if (q.field) params.set('field', q.field)
  params.set('bucket', q.bucket)
  if (q.from) params.set('from', q.from)
  if (q.perOwner) params.set('per_owner', 'true')
  const res = await api.get<{ points: RecordPoint[] }>(
    `/_records/${encodeURIComponent(type)}/_aggregate?${params.toString()}`,
    undefined,
    signal
  )
  return res.points ?? []
}
