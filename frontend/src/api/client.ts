import axios, { type AxiosInstance, type AxiosResponse } from 'axios'
import { ApiError, toApiError } from './errors'
import { apiUrl } from './base'
import { isProvisioningResponse, startProvisioningPoll } from './tenantProvisioning'
import { trackWorkspaceReadOnly } from './workspaceReadOnly'
import { accessRefusal, probeSessionEnded, refusedRequestError, trackAccess } from './workspaceAccess'
import { carriesAsOf, currentAsOf } from './asOf'
import { htmlResponseError, isHtmlResponse } from './htmlResponse'
import { t } from '@/i18n'

class ApiClient {
  private client: AxiosInstance

  constructor() {
    this.client = axios.create({
      baseURL: apiUrl('/api/v1'),
      headers: {
        'Content-Type': 'application/json',
      },
    })

    // A page reading at a past instant sends the instant with every read
    // the server can answer at it, and writes nothing (see asOf.ts).
    this.client.interceptors.request.use((config) => {
      // After a refusal (an ended session, a suspended workspace) no request
      // can succeed until the page reloads, so none is sent.
      if (accessRefusal.value) return Promise.reject(refusedRequestError())
      const at = currentAsOf()
      if (!at) return config
      const method = (config.method ?? 'get').toLowerCase()
      if (method !== 'get' && method !== 'head') {
        const message = t('asOf.readOnly')
        return Promise.reject(
          new ApiError(message, {
            kind: 'http',
            status: 422,
            problem: { type: 'as_of_read_only', title: message, status: 422 },
            original: null,
          }),
        )
      }
      if (carriesAsOf(config.url ?? '')) {
        config.params = { ...(config.params ?? {}), as_of: at }
      }
      return config
    })

    // Every failure is normalized to a single ApiError (BUG-X9VNE1):
    // catch sites read .message / .validationErrors / getScriptError()
    // instead of branching on the rejection shape.
    this.client.interceptors.response.use(
      (response) => {
        // A 202 with status "provisioning" carries no data the caller asked
        // for (TKT-TNPRV8): show the setting-up screen and fail the call
        // rather than hand the status on as data.
        if (isProvisioningResponse(response.status, response.data)) {
          void startProvisioningPoll()
          return Promise.reject(toApiError(new Error('tenant_provisioning')))
        }
        // An HTML page in place of JSON is not data: reject it rather than
        // hand a string to the caller. It may be the edge's sign-in page,
        // reached through a redirect the browser followed, so ask once
        // whether the session ended, as for a request with no response.
        if (isHtmlResponse(response)) {
          void probeSessionEnded()
          return Promise.reject(htmlResponseError(response))
        }
        trackWorkspaceReadOnly(response.config, response.status, response.data)
        return response
      },
      (error: unknown) => {
        // A write during the create, and a create that is throttled, failed
        // or needs an operator, is a 503 provisioning problem: show the
        // same screen, which keeps polling.
        if (
          axios.isAxiosError(error) &&
          error.response &&
          isProvisioningResponse(error.response.status, error.response.data)
        ) {
          void startProvisioningPoll()
        }
        if (axios.isAxiosError(error) && error.response) {
          trackWorkspaceReadOnly(error.config ?? {}, error.response.status, error.response.data)
          trackAccess(error.response.status, error.response.data)
        }
        const normalized = toApiError(error)
        // No response at all: perhaps the edge sent an ended session to its
        // sign-in page and the browser refused the redirect. Ask once.
        if (normalized instanceof ApiError && normalized.kind === 'network') {
          void probeSessionEnded()
        }
        return Promise.reject(normalized)
      },
    )
  }

  async get<T>(
    url: string,
    params?: Record<string, unknown>,
    signal?: AbortSignal,
  ): Promise<T> {
    const response: AxiosResponse<T> = await this.client.get(url, { params, signal })
    return response.data
  }

  async post<T>(url: string, data?: unknown, opts?: { signal?: AbortSignal }): Promise<T> {
    const response: AxiosResponse<T> = await this.client.post(url, data, { signal: opts?.signal })
    return response.data
  }

  async patch<T>(url: string, data: unknown, etag?: string, signal?: AbortSignal): Promise<T> {
    const headers: Record<string, string> = {}
    if (etag) {
      headers['If-Match'] = etag
    }
    const response: AxiosResponse<T> = await this.client.patch(url, data, { headers, signal })
    return response.data
  }

  async put<T>(url: string, data: unknown): Promise<T> {
    const response: AxiosResponse<T> = await this.client.put(url, data)
    return response.data
  }

  async delete(url: string): Promise<void> {
    await this.client.delete(url)
  }

  getAxios(): AxiosInstance {
    return this.client
  }
}

export const api = new ApiClient()
