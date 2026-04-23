import { api } from './client'
import { isSafeReturnPath } from '@/utils/returnPath'
import type { DocumentRenderResponse } from '@/types'

export async function renderDocument(
  docName: string,
  entityId: string,
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
  return api.get<DocumentRenderResponse>(
    `/_documents/${docName}/${entityId}`,
    Object.keys(params).length ? params : undefined,
  )
}
