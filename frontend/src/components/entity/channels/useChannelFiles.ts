import { onScopeDispose, shallowReactive, toValue, type MaybeRefOrGetter } from 'vue'
import { fetchMessageFile, type ChannelTarget } from '@/api/channels'
import type { Message } from '@/types/channels'

/**
 * The files of an open channel's messages: image previews and downloads.
 *
 * Every byte is read through the API client (fetchMessageFile), so the
 * request carries the reader's credentials, and the server gates it like
 * the message. Nothing is ever pointed at the file URL directly.
 *
 * A preview is fetched only for the raster types in PREVIEW_MIME_TYPES,
 * which a browser draws as an image and never runs, and only for a file of
 * at most MAX_PREVIEW_BYTES; a larger image is a chip with a download.
 * At most MAX_PREVIEW_FETCHES previews load at once; the rest wait in
 * order. Any other type, SVG
 * included, is a chip with a download and is never rendered inline. The
 * preview's object URL is made from a Blob typed with the stored MIME, so
 * the bytes are only ever shown as that image type.
 *
 * The previews follow the held messages: `sync` fetches the ones a held
 * message has and revokes the URLs of messages no longer held (removed,
 * refused on a refetch, or let go by the held-list cap). `clear` drops all
 * of them, for another channel; unmounting does the same.
 */

/** The image types shown as a thumbnail. */
export const PREVIEW_MIME_TYPES: ReadonlySet<string> = new Set([
  'image/png',
  'image/jpeg',
  'image/gif',
  'image/webp',
])

/** The largest file, in bytes, that gets a thumbnail. */
export const MAX_PREVIEW_BYTES = 2 * 1024 * 1024

/** The most preview fetches in flight at once. */
export const MAX_PREVIEW_FETCHES = 4

/** The MIME type without parameters, lower case. */
function baseType(mime: string): string {
  return (mime.split(';')[0] ?? '').trim().toLowerCase()
}

/** Whether a file of this MIME type gets a thumbnail. */
export function isPreviewable(mime: string): boolean {
  return PREVIEW_MIME_TYPES.has(baseType(mime))
}

/** Whether this file gets a thumbnail: a previewable type and small enough. */
function wantsPreview(file: { mime: string; size: number }): boolean {
  return isPreviewable(file.mime) && file.size <= MAX_PREVIEW_BYTES
}

/**
 * Hands a Blob to the browser as a download named `name`, through an
 * object URL that is revoked once the click has started the save.
 */
export function saveBlob(blob: Blob, name: string) {
  // Typed as plain bytes, so nothing about the blob can make the browser
  // open it in place instead of saving it.
  const url = URL.createObjectURL(new Blob([blob], { type: 'application/octet-stream' }))
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.rel = 'noopener'
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  a.remove()
  // Revoked after the click's task: some browsers drop the save when the
  // URL goes away in the same task (see api/theme.ts exportTheme).
  setTimeout(() => URL.revokeObjectURL(url), 0)
}

function fileKey(seq: number, id: string): string {
  return `${seq}:${id}`
}

export function useChannelFiles(target: MaybeRefOrGetter<ChannelTarget>) {
  /** Object URLs of loaded previews, by fileKey. */
  const previews = shallowReactive(new Map<string, string>())
  /**
   * Requests made, by fileKey, queued or in flight; kept after a failure
   * so it is not retried in a loop.
   */
  const requested = new Map<string, AbortController>()
  /** Previews waiting for a fetch slot, in the order they were wanted. */
  const queue: { seq: number; file: { id: string; mime: string }; key: string }[] = []
  let active = 0

  function revoke(key: string) {
    const url = previews.get(key)
    if (url) URL.revokeObjectURL(url)
    previews.delete(key)
  }

  /** Starts queued fetches while a slot is free; skips the ones dropped meanwhile. */
  function pump() {
    while (active < MAX_PREVIEW_FETCHES) {
      const next = queue.shift()
      if (!next) return
      const ctrl = requested.get(next.key)
      if (!ctrl || ctrl.signal.aborted) continue
      active++
      void load(next.seq, next.file, next.key, ctrl).finally(() => {
        active--
        pump()
      })
    }
  }

  async function load(
    seq: number,
    file: { id: string; mime: string },
    key: string,
    ctrl: AbortController
  ) {
    try {
      const blob = await fetchMessageFile(toValue(target), seq, file.id, ctrl.signal)
      if (ctrl.signal.aborted || requested.get(key) !== ctrl) return
      previews.set(key, URL.createObjectURL(new Blob([blob], { type: baseType(file.mime) })))
    } catch {
      // No preview; the chip still offers the download, which reports
      // its own failure.
    }
  }

  /** Fetches the previews of the held messages and drops the rest. */
  function sync(messages: readonly Message[]) {
    const wanted = new Set<string>()
    for (const m of messages) {
      if (m.removed) continue
      for (const f of m.files ?? []) {
        if (!wantsPreview(f)) continue
        const key = fileKey(m.seq, f.id)
        wanted.add(key)
        if (requested.has(key)) continue
        requested.set(key, new AbortController())
        queue.push({ seq: m.seq, file: f, key })
      }
    }
    for (const [key, ctrl] of [...requested]) {
      if (wanted.has(key)) continue
      ctrl.abort()
      requested.delete(key)
      revoke(key)
    }
    pump()
  }

  /** Drops every preview and request in flight. */
  function clear() {
    for (const ctrl of requested.values()) ctrl.abort()
    requested.clear()
    queue.length = 0
    for (const key of [...previews.keys()]) revoke(key)
  }

  /** The preview URL of one file, once loaded. */
  function previewUrl(seq: number, id: string): string | undefined {
    return previews.get(fileKey(seq, id))
  }

  /** Fetches one file and saves it under its name. Rejects when the read fails. */
  async function download(seq: number, file: { id: string; name: string }) {
    const blob = await fetchMessageFile(toValue(target), seq, file.id)
    saveBlob(blob, file.name)
  }

  onScopeDispose(clear)

  return { sync, clear, previewUrl, download }
}
