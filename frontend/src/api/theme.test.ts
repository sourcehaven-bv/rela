import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { uploadLogo, removeLogo } from './theme'

describe('theme api', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    fetchSpy = vi.spyOn(globalThis, 'fetch')
  })

  afterEach(() => {
    fetchSpy.mockRestore()
  })

  describe('uploadLogo', () => {
    it('PUTs multipart with the file in field "logo"', async () => {
      fetchSpy.mockResolvedValue(
        new Response(JSON.stringify({ ok: true, logoUrl: '/api/v1/_theme/logo?v=abc123' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

      const file = new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], 'logo.png', {
        type: 'image/png',
      })
      const result = await uploadLogo(file)

      expect(fetchSpy).toHaveBeenCalledTimes(1)
      const [url, init] = fetchSpy.mock.calls[0] as [string, RequestInit]
      expect(url).toBe('/api/v1/_theme/logo')
      expect(init.method).toBe('PUT')
      expect(init.body).toBeInstanceOf(FormData)
      expect((init.body as FormData).get('logo')).toBe(file)
      expect(result.logoUrl).toBe('/api/v1/_theme/logo?v=abc123')
    })

    it('throws with the server error message on failure', async () => {
      fetchSpy.mockResolvedValue(
        new Response(JSON.stringify({ error: 'unsupported format: image/gif' }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

      const file = new File([new Uint8Array([0])], 'broken.gif', { type: 'image/gif' })
      await expect(uploadLogo(file)).rejects.toThrow('unsupported format')
    })

    it('throws a generic message when the response has no body', async () => {
      fetchSpy.mockResolvedValue(new Response(null, { status: 500 }))
      const file = new File([new Uint8Array([0])], 'logo.png', { type: 'image/png' })
      await expect(uploadLogo(file)).rejects.toThrow(/Upload failed/)
    })
  })

  describe('removeLogo', () => {
    it('DELETEs the logo endpoint', async () => {
      fetchSpy.mockResolvedValue(new Response(null, { status: 204 }))
      await removeLogo()
      expect(fetchSpy).toHaveBeenCalledWith('/api/v1/_theme/logo', { method: 'DELETE' })
    })

    it('throws on non-2xx', async () => {
      fetchSpy.mockResolvedValue(
        new Response(JSON.stringify({ error: 'kaboom' }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      await expect(removeLogo()).rejects.toThrow('kaboom')
    })
  })
})
