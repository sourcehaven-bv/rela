// Theme asset API: user-uploaded sidebar logo. The backend persists
// bytes under `.rela/theme/logo` and serves them with a content-hash
// query param so any update produces a fresh URL the browser will fetch.

export interface UploadLogoResponse {
  ok: boolean
  logoUrl: string
}

export interface LogoUploadError extends Error {
  /** Server-reported maximum byte size, populated on 413 responses. */
  maxBytes?: number
}

/** Upload a logo image. The backend validates mime + size; on rejection
 *  the returned promise rejects with the server's error message. */
export async function uploadLogo(file: File): Promise<UploadLogoResponse> {
  const form = new FormData()
  form.append('logo', file)
  const response = await fetch('/api/v1/_theme/logo', {
    method: 'PUT',
    body: form,
  })
  if (!response.ok) {
    const data = await response.json().catch(() => ({ error: 'Upload failed' }))
    const err: LogoUploadError = new Error(data.error || `Upload failed (${response.status})`)
    if (typeof data.maxBytes === 'number') err.maxBytes = data.maxBytes
    throw err
  }
  return response.json()
}

/** Remove the current logo. Idempotent: succeeds even when no logo is
 *  currently set. */
export async function removeLogo(): Promise<void> {
  const response = await fetch('/api/v1/_theme/logo', { method: 'DELETE' })
  if (!response.ok && response.status !== 204) {
    const data = await response.json().catch(() => ({ error: 'Remove failed' }))
    throw new Error(data.error || `Remove failed (${response.status})`)
  }
}
