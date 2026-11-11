import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'
import type { Message } from '@/types/channels'

const fetchMessageFile = vi.hoisted(() => vi.fn())
vi.mock('@/api/channels', () => ({ fetchMessageFile }))

import {
  MAX_PREVIEW_BYTES,
  MAX_PREVIEW_FETCHES,
  isPreviewable,
  saveBlob,
  useChannelFiles,
} from './useChannelFiles'

const target = { type: 'feature', address: 'F-1', channelType: 'room' }

function msg(seq: number, mimes: string[], removed = false, size = 1): Message {
  const m: Message = {
    seq,
    rev: seq,
    type: 'card',
    files: mimes.map((mime, i) => ({
      id: `f${i}`,
      property: 'attach',
      name: `n${i}`,
      mime,
      size,
    })),
  }
  if (removed) m.removed = { kind: 'author', at: '2026-01-01T00:00:00Z' }
  return m
}

let urls = 0
const create = vi.fn(() => `blob:u${++urls}`)
const revoke = vi.fn()

beforeEach(() => {
  urls = 0
  create.mockClear()
  revoke.mockClear()
  fetchMessageFile.mockReset()
  fetchMessageFile.mockResolvedValue(new Blob(['x']))
  vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: create, revokeObjectURL: revoke }))
})
afterEach(() => vi.unstubAllGlobals())

const flush = () => new Promise((r) => setTimeout(r, 0))

describe('isPreviewable', () => {
  it.each([
    ['image/png', true],
    ['image/jpeg', true],
    ['image/gif', true],
    ['IMAGE/WEBP; q=1', true],
    ['image/svg+xml', false],
    ['image/bmp', false],
    ['text/html', false],
    ['application/pdf', false],
  ])('%s', (mime, want) => expect(isPreviewable(mime)).toBe(want))
})

describe('useChannelFiles', () => {
  it('fetches only raster previews of held messages and revokes on prune and dispose', async () => {
    const scope = effectScope()
    const files = scope.run(() => useChannelFiles(target))!
    files.sync([msg(1, ['image/png', 'image/svg+xml', 'text/plain']), msg(2, ['image/gif'], true)])
    await flush()
    expect(fetchMessageFile).toHaveBeenCalledTimes(1)
    expect(fetchMessageFile).toHaveBeenCalledWith(target, 1, 'f0', expect.any(AbortSignal))
    expect(files.previewUrl(1, 'f0')).toBe('blob:u1')
    expect(files.previewUrl(1, 'f1')).toBeUndefined()
    // The object URL is typed with the stored image type.
    expect((create.mock.calls[0] as unknown as [Blob])[0].type).toBe('image/png')

    // Synced again: no second fetch for what is held.
    files.sync([msg(1, ['image/png']), msg(3, ['image/webp'])])
    await flush()
    expect(fetchMessageFile).toHaveBeenCalledTimes(2)

    // Message 1 is no longer held: its URL is revoked.
    files.sync([msg(3, ['image/webp'])])
    expect(revoke).toHaveBeenCalledWith('blob:u1')
    expect(files.previewUrl(1, 'f0')).toBeUndefined()

    scope.stop()
    expect(revoke).toHaveBeenCalledWith('blob:u2')
  })

  it('does not retry a failed preview on every sync', async () => {
    fetchMessageFile.mockRejectedValue(new Error('nope'))
    const scope = effectScope()
    const files = scope.run(() => useChannelFiles(target))!
    files.sync([msg(1, ['image/png'])])
    await flush()
    files.sync([msg(1, ['image/png'])])
    await flush()
    expect(fetchMessageFile).toHaveBeenCalledTimes(1)
    expect(files.previewUrl(1, 'f0')).toBeUndefined()
    scope.stop()
  })

  it('gives no preview to an image over MAX_PREVIEW_BYTES', async () => {
    const scope = effectScope()
    const files = scope.run(() => useChannelFiles(target))!
    files.sync([
      msg(1, ['image/png'], false, MAX_PREVIEW_BYTES),
      msg(2, ['image/png'], false, MAX_PREVIEW_BYTES + 1),
    ])
    await flush()
    expect(fetchMessageFile).toHaveBeenCalledTimes(1)
    expect(fetchMessageFile).toHaveBeenCalledWith(target, 1, 'f0', expect.any(AbortSignal))
    expect(files.previewUrl(2, 'f0')).toBeUndefined()
    scope.stop()
  })

  it('runs at most MAX_PREVIEW_FETCHES fetches at once and queues the rest', async () => {
    const pending: ((b: Blob) => void)[] = []
    fetchMessageFile.mockImplementation(
      () => new Promise<Blob>((resolve) => pending.push(resolve))
    )
    const scope = effectScope()
    const files = scope.run(() => useChannelFiles(target))!
    const held = Array.from({ length: MAX_PREVIEW_FETCHES + 2 }, (_, i) => msg(i + 1, ['image/png']))
    files.sync(held)
    await flush()
    expect(fetchMessageFile).toHaveBeenCalledTimes(MAX_PREVIEW_FETCHES)

    // A queued preview whose message is dropped is never fetched.
    files.sync(held.filter((m) => m.seq !== MAX_PREVIEW_FETCHES + 1))
    pending[0]!(new Blob(['x']))
    await flush()
    expect(fetchMessageFile).toHaveBeenCalledTimes(MAX_PREVIEW_FETCHES + 1)
    expect(fetchMessageFile).toHaveBeenLastCalledWith(
      target,
      MAX_PREVIEW_FETCHES + 2,
      'f0',
      expect.any(AbortSignal)
    )
    expect(files.previewUrl(1, 'f0')).toBe('blob:u1')

    for (const resolve of pending.slice(1)) resolve(new Blob(['x']))
    await flush()
    expect(fetchMessageFile).toHaveBeenCalledTimes(MAX_PREVIEW_FETCHES + 1)
    expect(files.previewUrl(MAX_PREVIEW_FETCHES + 2, 'f0')).toBeDefined()
    scope.stop()
  })

  it('drops queued previews on clear', async () => {
    fetchMessageFile.mockImplementation(() => new Promise<Blob>(() => {}))
    const scope = effectScope()
    const files = scope.run(() => useChannelFiles(target))!
    files.sync(Array.from({ length: MAX_PREVIEW_FETCHES + 1 }, (_, i) => msg(i + 1, ['image/png'])))
    await flush()
    files.clear()
    files.sync([])
    await flush()
    expect(fetchMessageFile).toHaveBeenCalledTimes(MAX_PREVIEW_FETCHES)
    scope.stop()
  })

  it('downloads through an object URL that is revoked afterwards', async () => {
    vi.useFakeTimers()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    try {
      const scope = effectScope()
      const files = scope.run(() => useChannelFiles(target))!
      await files.download(4, { id: 'f9', name: 'report.html' })
      expect(fetchMessageFile).toHaveBeenCalledWith(target, 4, 'f9')
      expect(click).toHaveBeenCalledTimes(1)
      const a = click.mock.contexts[0] as HTMLAnchorElement
      expect(a.download).toBe('report.html')
      // Saved as plain bytes, never as the stored type.
      expect((create.mock.calls[0] as unknown as [Blob])[0].type).toBe('application/octet-stream')
      expect(revoke).not.toHaveBeenCalled()
      vi.runAllTimers()
      expect(revoke).toHaveBeenCalledWith('blob:u1')
      expect(document.querySelector('a[download]')).toBeNull()
      scope.stop()
    } finally {
      click.mockRestore()
      vi.useRealTimers()
    }
  })

  it('saveBlob leaves no anchor in the document', () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    saveBlob(new Blob(['a']), 'a.txt')
    expect(document.body.querySelector('a')).toBeNull()
    click.mockRestore()
  })
})
