// Attachment API: upload / delete the files bound to a `file`-type entity
// property. The bytes are served from, and written to, an ACL-gated
// endpoint that inherits the owning entity's permission. Uses axios
// directly (the shared client is JSON-oriented) so multipart uploads can
// report progress via onUploadProgress.
import axios from 'axios'
import { getPlural } from './entities'
import type { Entity } from '@/types'

function propertyUrl(entityType: string, entityId: string, property: string): string {
  return `/api/v1/${getPlural(entityType)}/${encodeURIComponent(entityId)}/_attachments/${encodeURIComponent(property)}`
}

// AttachmentError carries the HTTP status so callers can distinguish a
// 413 (too large) / 409 (at max) from a 403/422 and render the right
// inline message.
export class AttachmentError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.name = 'AttachmentError'
    this.status = status
  }
}

interface ProblemBody {
  detail?: string
  title?: string
}

function toAttachmentError(err: unknown): AttachmentError {
  if (axios.isAxiosError(err) && err.response) {
    const body = err.response.data as ProblemBody | undefined
    const detail = body?.detail || body?.title || err.response.statusText
    return new AttachmentError(detail, err.response.status)
  }
  return new AttachmentError('Request failed', 0)
}

/** The human reason an attachment write was refused, as a bare noun phrase
 *  ("too large") so a caller can frame it either as a sentence or inside a
 *  list of filenames. Hoisted here (RR-1556G1 M4) because both the file widget
 *  and the create form map these statuses, and they had already drifted — one
 *  lacked a 403 branch. Falls back to the server's problem+json detail, which
 *  is written for client display. */
export function attachmentErrorReason(err: unknown): string {
  // Duck-typed on `status` rather than `instanceof AttachmentError`: the class
  // identity is not stable across module boundaries (a vi.mock factory, or two
  // copies of the module in a bundle), and silently degrading a 413 to a
  // generic "upload failed" because the prototype chain differs is exactly the
  // failure this helper exists to prevent.
  const status = typeof err === 'object' && err !== null ? (err as { status?: unknown }).status : undefined
  const message = typeof err === 'object' && err !== null ? (err as { message?: unknown }).message : undefined
  if (typeof status === 'number') {
    if (status === 413) return 'too large'
    if (status === 409) return 'at the maximum number of files'
    if (status === 403) return 'not permitted'
  }
  return typeof message === 'string' && message ? message : 'upload failed'
}

/** Upload (append, or replace at max:1) a file on a property. `onProgress`
 *  receives a 0..1 fraction. Returns the updated entity (its
 *  `_attachments` reflects the new file). Throws AttachmentError on a
 *  non-2xx response. */
export async function uploadAttachment(
  entityType: string,
  entityId: string,
  property: string,
  file: File,
  onProgress?: (fraction: number) => void
): Promise<Entity> {
  const form = new FormData()
  form.append('file', file)
  try {
    const res = await axios.put<Entity>(propertyUrl(entityType, entityId, property), form, {
      onUploadProgress: (e) => {
        if (onProgress && e.total) onProgress(e.loaded / e.total)
      },
    })
    return res.data
  } catch (err) {
    throw toAttachmentError(err)
  }
}

/** Remove one file from a property, given the server-provided per-file
 *  `href` (the same URL serves GET and DELETE). Using the href verbatim
 *  avoids re-escaping the filename, so there's a single escaper. Idempotent.
 *  Throws AttachmentError on a non-2xx (non-204) response. */
export async function deleteAttachment(href: string): Promise<void> {
  try {
    await axios.delete(href)
  } catch (err) {
    throw toAttachmentError(err)
  }
}
