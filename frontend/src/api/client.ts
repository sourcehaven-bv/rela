import axios, { AxiosError, type AxiosInstance, type AxiosResponse } from 'axios'

export interface ProblemDetail {
  type: string
  title: string
  status: number
  detail?: string
  instance?: string
  errors?: ValidationError[]
}

export interface ValidationError {
  source?: { pointer: string }
  code?: string
  field?: string
  message?: string
  detail?: string
}

class ApiClient {
  private client: AxiosInstance

  constructor() {
    this.client = axios.create({
      baseURL: '/api/v1',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    this.client.interceptors.response.use(
      (response) => response,
      (error: AxiosError<ProblemDetail | { error?: string }>) => {
        const data = error.response?.data
        // Script-failure envelope (HTTP 422 from Lua surfaces) is shaped
        // like { error: "script_error", ... }. Pass it through as-is so
        // catch handlers can recognise and route it to <ScriptErrorPanel>.
        if (data && typeof data === 'object' && 'error' in data && data.error === 'script_error') {
          return Promise.reject(data)
        }
        if (data && typeof data === 'object' && 'type' in data && data.type) {
          return Promise.reject(data)
        }
        return Promise.reject(error)
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

  async post<T>(url: string, data?: unknown): Promise<T> {
    const response: AxiosResponse<T> = await this.client.post(url, data)
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
