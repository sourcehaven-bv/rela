import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { uploadLogo, removeLogo, exportTheme, importTheme } from './theme'

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

  describe('exportTheme', () => {
    it('GETs the export endpoint and triggers a download', async () => {
      const blob = new Blob([new Uint8Array([0x50, 0x4b])], { type: 'application/zip' })
      fetchSpy.mockResolvedValue(
        new Response(blob, {
          status: 200,
          headers: {
            'Content-Type': 'application/zip',
            'Content-Disposition': 'attachment; filename="my-theme.relatheme"',
          },
        }),
      )
      const createSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock')
      const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})

      await exportTheme()

      expect(fetchSpy).toHaveBeenCalledWith('/api/v1/_theme/export', { method: 'GET' })
      expect(createSpy).toHaveBeenCalledOnce()
      // Revoke is deferred via setTimeout so Safari doesn't drop the
      // download — wait one macrotask before asserting.
      await new Promise((resolve) => setTimeout(resolve, 0))
      expect(revokeSpy).toHaveBeenCalledWith('blob:mock')

      createSpy.mockRestore()
      revokeSpy.mockRestore()
    })

    it('throws with the server error message on failure', async () => {
      fetchSpy.mockResolvedValue(
        new Response(JSON.stringify({ error: 'no palette' }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      await expect(exportTheme()).rejects.toThrow('no palette')
    })
  })

  describe('importTheme', () => {
    it('POSTs multipart with the file in the "file" field', async () => {
      const fakePalette = { accent: '#abcdef' }
      fetchSpy.mockResolvedValue(
        new Response(JSON.stringify({ palette: fakePalette, logoUrl: '/api/v1/_theme/logo?v=ab' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

      const file = new File([new Uint8Array([0x50, 0x4b])], 'theme.relatheme', { type: 'application/zip' })
      const result = await importTheme(file)

      expect(fetchSpy).toHaveBeenCalledTimes(1)
      const [url, init] = fetchSpy.mock.calls[0] as [string, RequestInit]
      expect(url).toBe('/api/v1/_theme/import')
      expect(init.method).toBe('POST')
      expect(init.body).toBeInstanceOf(FormData)
      expect((init.body as FormData).get('file')).toBe(file)
      expect(result.palette).toEqual(fakePalette)
      expect(result.logoUrl).toBe('/api/v1/_theme/logo?v=ab')
    })

    it('throws with the server error message on rejection', async () => {
      fetchSpy.mockResolvedValue(
        new Response(JSON.stringify({ error: 'invalid theme manifest: ...' }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      const file = new File([new Uint8Array([0])], 'broken.relatheme', { type: 'application/zip' })
      await expect(importTheme(file)).rejects.toThrow('invalid theme manifest')
    })
  })
})
