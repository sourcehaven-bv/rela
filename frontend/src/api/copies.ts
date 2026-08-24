import { api } from './client'

/**
 * The copy surface (TKT-WRLDAPI item 5).
 *
 * ## There is no list call here, deliberately
 *
 * Offers ride the ENTITY response as `_copies`, alongside `_actions` — the way
 * every other affordance in this app is delivered. An earlier revision shipped
 * a `GET /_copies` and it was removed: a second endpoint for one affordance is
 * the odd shape, and it made the client construct a lookup key for data it had
 * already fetched. So this module is invoke-only; to READ the offers, read
 * `entity._copies`.
 */

// CopyInvokeResult reports what an invoked copy produced. Mirrors
// v1.CopyResult.
export interface CopyInvokeResult {
  definition: string
  entityId: string
  // The coordinate written — `published`, `nl`. Empty means the default state.
  pointer: string
  // True when the copy brought the target face into existence rather than
  // overwriting one — the difference between "published for the first time"
  // and "re-published".
  created: boolean
}

/**
 * invokeCopy runs a declared copy definition BY NAME.
 *
 * The request names a definition and the entities it applies to; it never
 * supplies a definition's contents. That is the transforms-registry precedent:
 * if a caller could describe a copy, they could describe one whose guard is
 * convenient, and the guard system would be decorative.
 *
 * `targetId` is required for a CROSS-ENTITY copy and must be omitted for a
 * same-entity one, whose target is the source by construction. Read
 * `offer.sameEntity` to know which you have — a client cannot build a valid
 * invoke without it.
 *
 * Authorization happens server-side. `offer.allowed` is a hint for rendering,
 * so calling this on a disallowed offer is not a bug in the caller — it
 * returns the same 403 the kernel would return anyway.
 */
export async function invokeCopy(
  name: string,
  sourceId: string,
  targetId?: string,
): Promise<CopyInvokeResult> {
  return api.post<CopyInvokeResult>(`/_copies/${encodeURIComponent(name)}`, {
    source_id: sourceId,
    ...(targetId ? { target_id: targetId } : {}),
  })
}
