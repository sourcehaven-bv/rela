import axios, { type AxiosInstance, type AxiosResponse } from 'axios'
import { ApiError, toApiError } from './errors'
import { apiUrl } from './base'
import { isProvisioningResponse, startProvisioningPoll } from './tenantProvisioning'
import { trackWorkspaceReadOnly } from './workspaceReadOnly'
import { carriesAsOf, currentAsOf } from './asOf'
import { t } from '@/i18n'

// The error types and helpers live in errors.ts; re-export so existing
// `from '@/api'` / `from '@/api/client'` importers keep working.
export type { ProblemDetail, ValidationError } from './errors'

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
        }
        return Promise.reject(toApiError(error))
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
