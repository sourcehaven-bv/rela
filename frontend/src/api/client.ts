import axios, { type AxiosInstance, type AxiosResponse } from 'axios'
import { toApiError } from './errors'
import { apiUrl } from './base'
import { isProvisioningResponse, startProvisioningPoll } from './tenantProvisioning'

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

    // Every failure is normalized to a single ApiError (BUG-X9VNE1):
    // catch sites read .message / .validationErrors / getScriptError()
    // instead of branching on the rejection shape.
    this.client.interceptors.response.use(
      (response) => {
        // A 202 tenant_provisioning carries a problem, not the data the
        // caller asked for (TKT-TNPRV8): show the setting-up screen and
        // fail the call rather than hand the problem on as data.
        if (isProvisioningResponse(response.status, response.data)) {
          void startProvisioningPoll()
          return Promise.reject(toApiError(new Error('tenant_provisioning')))
        }
        return response
      },
      (error: unknown) => {
        // A create that is throttled or failed is a 503 provisioning
        // problem: show the same screen, which keeps polling.
        if (
          axios.isAxiosError(error) &&
          error.response &&
          isProvisioningResponse(error.response.status, error.response.data)
        ) {
          void startProvisioningPoll()
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
