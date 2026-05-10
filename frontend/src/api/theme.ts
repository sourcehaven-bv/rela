// Theme asset API: user-uploaded sidebar logo + portable theme
// packages. The backend persists logo bytes under `.rela/theme/logo`
// and serves them with a content-hash query param so any update
// produces a fresh URL the browser will fetch. Theme packages are
// .relatheme zips containing a manifest plus the optional logo.

import type { PaletteConfig } from './settings'

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

export interface ImportThemeResponse {
  /** Parsed palette from the manifest. The frontend stages this into
   *  the palette editor; the user clicks Save palette to persist. */
  palette: PaletteConfig
  /** Cache-busted URL for the freshly-imported logo, when one was
   *  bundled. Absent when the package didn't include a logo. */
  logoUrl?: string
}

/** Download the current palette + logo as a `.relatheme` zip via an
 *  invisible anchor click. The browser handles the actual save. */
export async function exportTheme(): Promise<void> {
  const response = await fetch('/api/v1/_theme/export', { method: 'GET' })
  if (!response.ok) {
    const data = await response.json().catch(() => ({ error: 'Export failed' }))
    throw new Error(data.error || `Export failed (${response.status})`)
  }
  // Pull the filename out of Content-Disposition when present so we
  // honor the server's safe-name derivation; fall back to a generic
  // default otherwise.
  const cd = response.headers.get('Content-Disposition') ?? ''
  const match = /filename="([^"]+)"/.exec(cd)
  const filename = match?.[1] ?? 'theme.relatheme'

  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  // Defer revoke: Safari (and historically older Chrome) drops the
  // download silently if the URL is revoked synchronously after click.
  // The microtask queue is empty by the time the browser starts the
  // actual save, so a 0ms timeout is enough headroom.
  setTimeout(() => URL.revokeObjectURL(url), 0)
}

/** Install a `.relatheme` zip. The backend persists the bundled logo
 *  immediately (matching the direct logo PUT path) and returns the
 *  manifest's palette JSON for the frontend to stage in the editor. */
export async function importTheme(file: File): Promise<ImportThemeResponse> {
  const form = new FormData()
  form.append('file', file)
  const response = await fetch('/api/v1/_theme/import', {
    method: 'POST',
    body: form,
  })
  if (!response.ok) {
    const data = await response.json().catch(() => ({ error: 'Install failed' }))
    throw new Error(data.error || `Install failed (${response.status})`)
  }
  return response.json()
}
