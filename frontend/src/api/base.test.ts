import { describe, it, expect, afterEach, vi } from 'vitest'

// base.ts reads the meta tag once at module init, so each case needs a fresh
// module registry with the tag already in place.
async function loadWithBase(content?: string) {
  document.head.querySelectorAll('meta[name="rela-base"]').forEach((m) => m.remove())
  if (content !== undefined) {
    const meta = document.createElement('meta')
    meta.name = 'rela-base'
    meta.content = content
    document.head.appendChild(meta)
  }
  vi.resetModules()
  return import('./base')
}

describe('relaBase', () => {
  afterEach(() => {
    document.head.querySelectorAll('meta[name="rela-base"]').forEach((m) => m.remove())
  })

  it('defaults to / when there is no meta tag', async () => {
    const { relaBase } = await loadWithBase(undefined)
    expect(relaBase()).toBe('/')
  })

  it('reads the tag and keeps the trailing slash', async () => {
    const { relaBase } = await loadWithBase('/p/abc123/')
    expect(relaBase()).toBe('/p/abc123/')
  })

  it('adds a missing trailing slash', async () => {
    const { relaBase } = await loadWithBase('/p/abc123')
    expect(relaBase()).toBe('/p/abc123/')
  })

  it('treats an empty tag as unprefixed', async () => {
    const { relaBase } = await loadWithBase('')
    expect(relaBase()).toBe('/')
  })
})

describe('apiUrl', () => {
  afterEach(() => {
    document.head.querySelectorAll('meta[name="rela-base"]').forEach((m) => m.remove())
  })

  it('leaves paths untouched when unprefixed', async () => {
    const { apiUrl } = await loadWithBase(undefined)
    expect(apiUrl('/api/v1/_settings')).toBe('/api/v1/_settings')
  })

  it('prefixes a root-absolute path', async () => {
    const { apiUrl } = await loadWithBase('/p/abc123/')
    expect(apiUrl('/api/v1/_settings')).toBe('/p/abc123/api/v1/_settings')
  })

  // The command, open-file and help routes are not under /api/v1, so the
  // helper must take any root-absolute path rather than assume that root.
  it('prefixes endpoints outside /api/v1', async () => {
    const { apiUrl } = await loadWithBase('/p/abc123/')
    expect(apiUrl('/api/command/x')).toBe('/p/abc123/api/command/x')
    expect(apiUrl('/api/help/ticket')).toBe('/p/abc123/api/help/ticket')
  })

  it('does not double the slash', async () => {
    const { apiUrl } = await loadWithBase('/p/abc123/')
    expect(apiUrl('/x')).not.toContain('//')
  })

  it('preserves query strings', async () => {
    const { apiUrl } = await loadWithBase('/p/abc123/')
    expect(apiUrl('/api/v1/t/_export?transform=pdf')).toBe(
      '/p/abc123/api/v1/t/_export?transform=pdf'
    )
  })
})
