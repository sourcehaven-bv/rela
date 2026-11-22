import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AxiosAdapter, AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import { api } from './client'
import { ApiError } from './errors'
import { runAction } from './actions'
import { REFUSAL, accessRefusal, resetAccessForTest } from './workspaceAccess'

const LOGIN_PAGE = '<!doctype html><html><body><form>Sign in</form></body></html>'

const axiosInstance = api.getAxios()
const originalAdapter = axiosInstance.defaults.adapter

/** Answers every request with this status, content type and raw body. */
function answer(status: number, contentType: string | undefined, body: string): void {
  const adapter: AxiosAdapter = (config: InternalAxiosRequestConfig) =>
    Promise.resolve({
      data: body,
      status,
      statusText: 'OK',
      headers: contentType ? { 'content-type': contentType } : {},
      config,
    } as AxiosResponse)
  axiosInstance.defaults.adapter = adapter
}

// Every HTML answer starts a status probe; keep it off the network.
beforeEach(() => {
  probeAnswers(false)
})

afterEach(() => {
  axiosInstance.defaults.adapter = originalAdapter
  vi.unstubAllGlobals()
  resetAccessForTest()
})

/** The status probe's answer: a redirect (the edge's sign-in page) or a 200. */
function probeAnswers(redirect: boolean): ReturnType<typeof vi.fn> {
  const fetchFn = vi.fn(() =>
    Promise.resolve({
      type: redirect ? 'opaqueredirect' : 'basic',
      status: redirect ? 0 : 200,
      json: () => Promise.resolve({ status: 'active' }),
    } as Response)
  )
  vi.stubGlobal('fetch', fetchFn)
  return fetchFn
}

describe('a JSON call answered with an HTML page', () => {
  it('rejects a 200 sign-in page instead of returning it as data', async () => {
    answer(200, 'text/html; charset=utf-8', LOGIN_PAGE)
    const err = await api.get('/commands').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).problem?.type).toBe('unexpected_response')
    expect((err as ApiError).status).toBe(200)
  })

  it('rejects markup that comes without a content type', async () => {
    answer(200, undefined, `\n  ${LOGIN_PAGE}`)
    await expect(api.get('/commands')).rejects.toBeInstanceOf(ApiError)
  })

  it('rejects an XHTML page', async () => {
    answer(200, 'application/xhtml+xml', LOGIN_PAGE)
    await expect(api.get('/commands')).rejects.toBeInstanceOf(ApiError)
  })

  it('shows the session ended when the status probe is redirected', async () => {
    const fetchFn = probeAnswers(true)
    answer(200, 'text/html', LOGIN_PAGE)
    await expect(api.get('/commands')).rejects.toBeInstanceOf(ApiError)
    await vi.waitFor(() => expect(accessRefusal.value).toBe(REFUSAL.sessionEnded))
    expect(fetchFn).toHaveBeenCalledTimes(1)
  })

  it('leaves the session alone when the status probe is answered', async () => {
    const fetchFn = probeAnswers(false)
    answer(200, 'text/html', LOGIN_PAGE)
    await expect(api.get('/commands')).rejects.toBeInstanceOf(ApiError)
    await vi.waitFor(() => expect(fetchFn).toHaveBeenCalledTimes(1))
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(accessRefusal.value).toBeNull()
  })

  it('rejects an HTML page on a write too', async () => {
    answer(200, 'text/html', LOGIN_PAGE)
    await expect(api.post('/entities/task', {})).rejects.toBeInstanceOf(ApiError)
  })
})

describe('answers the client keeps returning', () => {
  it('returns parsed JSON', async () => {
    answer(200, 'application/json', '[{"name":"build"}]')
    await expect(api.get('/commands')).resolves.toEqual([{ name: 'build' }])
  })

  it('passes an empty body', async () => {
    answer(204, undefined, '')
    await expect(api.delete('/entities/task/T-1')).resolves.toBeUndefined()
  })

  // An action may answer with its own body in an allowlisted type that is not
  // JSON; callers and embedded apps read it.
  it.each([
    ['text/plain; charset=utf-8', 'ok'],
    ['text/csv', 'id,title\nT-1,Build'],
    ['application/xml', '<result>ok</result>'],
  ])('returns an action body of type %s', async (contentType, body) => {
    answer(200, contentType, body)
    await expect(runAction('export', 'T-1', 'task')).resolves.toBe(body)
  })

  it('returns HTML to a call that asked for text', async () => {
    answer(200, 'text/html', LOGIN_PAGE)
    const res = await axiosInstance.get<string>('/page', { responseType: 'text' })
    expect(res.data).toBe(LOGIN_PAGE)
  })
})
