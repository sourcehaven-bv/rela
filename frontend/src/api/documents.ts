import { api } from './client'
import { isSafeReturnPath } from '@/utils/returnPath'
import type { DocumentRenderResponse } from '@/types'

/**
 * Renders a document.
 *
 * `entityId` is omitted for a STANDALONE document — one configured without an
 * `entity_type:`, whose content is company-wide rather than about one entity.
 * That request goes to `/_documents/{name}` with no id segment, and the server
 * rejects the mismatched shape either way (an entity-anchored document cannot
 * be fetched without an id, or vice versa).
 */
export async function renderDocument(
  docName: string,
  entityId?: string,
  opts: { refresh?: boolean; returnTo?: string } = {},
): Promise<DocumentRenderResponse> {
  const params: Record<string, string> = {}
  if (opts.refresh) params.refresh = 'true'
  // The server uses return_to to inject a matching query param into any
  // form link inside the rendered HTML, so submitting a form redirects
  // back to the page currently rendering the document. isSafeReturnPath
  // enforces the open-redirect guard — the server applies the same
  // check, this one only prevents wasted round-trips on obvious bad input.
  const safe = isSafeReturnPath(opts.returnTo)
  if (safe) params.return_to = safe
  const path = entityId ? `/_documents/${docName}/${entityId}` : `/_documents/${docName}`
  return api.get<DocumentRenderResponse>(path, Object.keys(params).length ? params : undefined)
}
