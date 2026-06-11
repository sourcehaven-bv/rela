import axios, { type AxiosInstance, type AxiosResponse } from 'axios'
import { toApiError } from './errors'

// The error types and helpers live in errors.ts; re-export so existing
// `from '@/api'` / `from '@/api/client'` importers keep working.
export type { ProblemDetail, ValidationError } from './errors'

class ApiClient {
  private client: AxiosInstance

  constructor() {
    this.client = axios.create({
      baseURL: '/api/v1',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    // Every failure is normalized to a single ApiError (BUG-X9VNE1):
    // catch sites read .message / .validationErrors / getScriptError()
    // instead of branching on the rejection shape.
    this.client.interceptors.response.use(
      (response) => response,
      (error: unknown) => Promise.reject(toApiError(error)),
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

  async delete(url: string): Promise<void> {
    await this.client.delete(url)
  }

  getAxios(): AxiosInstance {
    return this.client
  }
}

export const api = new ApiClient()
