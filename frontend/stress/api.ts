// Minimal REST client used by the runner and by workload operations that
// need to perform background reads/writes outside of the browser session
// (e.g. seeding, latency canaries).
//
// We send the Referer header explicitly so the server's same-origin
// middleware accepts the request — this client speaks straight to the
// loopback port, no Vite proxy in front of it.

import type { ApiClient } from './types.js'

export function makeApi(baseUrl: string): ApiClient {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Referer: `${baseUrl}/`,
  }
  async function call(method: string, path: string, body?: unknown): Promise<Response> {
    const url = `${baseUrl}/api/v1/${path.replace(/^\/+/, '')}`
    return fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  }
  return {
    async get(path) {
      const r = await call('GET', path)
      if (!r.ok) throw new Error(`GET ${path} → ${r.status}`)
      return r.json()
    },
    async post(path, body) {
      const r = await call('POST', path, body)
      if (!r.ok) throw new Error(`POST ${path} → ${r.status}: ${await r.text()}`)
      return r.json()
    },
    async patch(path, body) {
      const r = await call('PATCH', path, body)
      if (!r.ok) throw new Error(`PATCH ${path} → ${r.status}: ${await r.text()}`)
      return r.json()
    },
    async delete(path) {
      const r = await call('DELETE', path)
      if (!r.ok && r.status !== 404) throw new Error(`DELETE ${path} → ${r.status}`)
    },
    async timed(method, path, body) {
      const start = performance.now()
      const r = await call(method, path, body)
      // Drain body so we measure full request time, not just headers.
      try {
        await r.text()
      } catch {
        /* ignore */
      }
      return { status: r.status, ms: performance.now() - start }
    },
  }
}
