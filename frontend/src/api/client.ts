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
      (error: AxiosError<ProblemDetail>) => {
        if (error.response?.data?.type) {
          return Promise.reject(error.response.data)
        }
        return Promise.reject(error)
      }
    )
  }

  async get<T>(url: string, params?: Record<string, unknown>): Promise<T> {
    const response: AxiosResponse<T> = await this.client.get(url, { params })
    return response.data
  }

  async post<T>(url: string, data?: unknown): Promise<T> {
    const response: AxiosResponse<T> = await this.client.post(url, data)
    return response.data
  }

  async patch<T>(url: string, data: unknown, etag?: string): Promise<T> {
    const headers: Record<string, string> = {}
    if (etag) {
      headers['If-Match'] = etag
    }
    const response: AxiosResponse<T> = await this.client.patch(url, data, { headers })
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
