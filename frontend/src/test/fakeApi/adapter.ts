// The axios adapter that routes the SPA's requests to a FakeServer.
//
// With a fast-check scheduler, every request waits twice: once before the
// fake runs it, and once before its answer reaches the client. So the
// scheduler decides both the order in which the server applies concurrent
// writes and the order in which the client sees their answers. Without a
// scheduler the fake answers on the next microtask, in request order.
//
// A fault answers a request in place of the fake, as a proxy does when the
// server is down. The fake never sees the request, so it stays verified.

import {
  AxiosError,
  AxiosHeaders,
  CanceledError,
  type AxiosAdapter,
  type InternalAxiosRequestConfig,
} from 'axios'
import type { Scheduler } from 'fast-check'

import type { FakeRequest, FakeResponse, FakeServer } from './server'

export interface RecordedRequest extends FakeRequest {
  /** Order in which the client sent it, from 0. */
  sent: number
  /** The answer, once the fake has run the request; undefined while held or when aborted first. */
  response?: FakeResponse
}

export interface FakeAdapter {
  adapter: AxiosAdapter
  /** Every request the client sent, in send order. */
  readonly requests: readonly RecordedRequest[]
}

export interface FakeAdapterOptions {
  /** Runs as each request reaches the server; an answer here replaces the fake's. */
  fault?: (req: FakeRequest) => FakeResponse | undefined
}

const API_PREFIX = '/api/v1'

export function fakeAdapter(
  server: FakeServer,
  scheduler?: Scheduler,
  options: FakeAdapterOptions = {}
): FakeAdapter {
  const requests: RecordedRequest[] = []

  const adapter: AxiosAdapter = async (config) => {
    const req = toFakeRequest(config)
    const recorded: RecordedRequest = { ...req, sent: requests.length }
    requests.push(recorded)
    const label = `${req.method} ${req.path}`

    if (scheduler) await abortable(config, scheduler.schedule(Promise.resolve(), `run ${label}`))
    else await abortable(config, Promise.resolve())

    const res = options.fault?.(req) ?? server.handle(req)
    recorded.response = res

    if (scheduler)
      await abortable(config, scheduler.schedule(Promise.resolve(), `deliver ${label}`))

    const response = {
      data: res.data,
      status: res.status,
      statusText: String(res.status),
      headers: new AxiosHeaders({ 'content-type': 'application/json' }),
      config,
      request: {},
    }
    if (config.validateStatus ? !config.validateStatus(res.status) : res.status >= 400) {
      throw new AxiosError(
        `Request failed with status code ${res.status}`,
        res.status >= 500 ? AxiosError.ERR_BAD_RESPONSE : AxiosError.ERR_BAD_REQUEST,
        config,
        response.request,
        response
      )
    }
    return response
  }

  return { adapter, requests }
}

function toFakeRequest(config: InternalAxiosRequestConfig): FakeRequest {
  const url = new URL(
    (config.baseURL ?? '').replace(/\/$/, '') + (config.url ?? ''),
    'http://fake.invalid'
  )
  let path = url.pathname
  const at = path.indexOf(API_PREFIX)
  if (at < 0) throw new Error(`fake API: request outside ${API_PREFIX}: ${path}`)
  path = path.slice(at + API_PREFIX.length) || '/'

  const query: Record<string, string> = {}
  url.searchParams.forEach((v, k) => (query[k] = v))
  for (const [k, v] of Object.entries((config.params ?? {}) as Record<string, unknown>)) {
    if (v !== undefined && v !== null) query[k] = String(v)
  }

  // axios has already serialized the body by the time an adapter sees it.
  const body =
    typeof config.data === 'string' && config.data !== '' ? JSON.parse(config.data) : config.data
  const headers: Record<string, string> = {}
  const ifMatch = AxiosHeaders.from(config.headers).get('If-Match')
  if (typeof ifMatch === 'string') headers['If-Match'] = ifMatch

  return { method: (config.method ?? 'get').toUpperCase(), path, query, body, headers }
}

// Rejects as axios does when the request's signal aborts while it waits.
function abortable<T>(config: InternalAxiosRequestConfig, p: Promise<T>): Promise<T> {
  const signal = config.signal as AbortSignal | undefined
  if (!signal) return p
  if (signal.aborted) return Promise.reject(new CanceledError(undefined, undefined, config))
  return new Promise<T>((resolve, reject) => {
    const onAbort = () => reject(new CanceledError(undefined, undefined, config))
    signal.addEventListener('abort', onAbort, { once: true })
    p.then(
      (v) => {
        signal.removeEventListener('abort', onAbort)
        resolve(v)
      },
      (e: unknown) => {
        signal.removeEventListener('abort', onAbort)
        reject(e)
      }
    )
  })
}
