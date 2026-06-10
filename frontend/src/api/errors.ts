// The API error boundary (BUG-X9VNE1).
//
// The axios response interceptor in client.ts normalizes every failure
// into one ApiError, so catch sites never branch on rejection shape.
// Before this existed, rejections arrived as three different shapes
// (plain script_error envelope, plain ProblemDetail, raw AxiosError)
// and the common `err instanceof Error ? err.message : fallback` idiom
// silently discarded the server's message in every branch.
//
// Conventions for catch sites:
//   - user-facing message      → getErrorMessage(err, 'context fallback')
//   - script-error routing     → getScriptError(err) → scriptErrorStore
//   - suppress cancellations   → isCancelledFetch (usePageData) delegates here
//   - field-level validation   → (err as ApiError).validationErrors
import axios, { type AxiosError } from 'axios'
import type { ScriptError } from '@/types/scriptError'
import { isScriptError } from '@/types/scriptError'

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

export type ApiErrorKind = 'script' | 'http' | 'cancelled' | 'network'

export class ApiError extends Error {
  readonly kind: ApiErrorKind
  /** HTTP status, when a response was received. */
  readonly status?: number
  /** The server's ProblemDetail body, when it sent one. */
  readonly problem?: ProblemDetail
  /** Field-level validation errors from the ProblemDetail, [] otherwise. */
  readonly validationErrors: ValidationError[]
  /** The Lua script-failure envelope, for kind === 'script'. */
  readonly scriptError?: ScriptError
  readonly correlationId?: string
  /** The original rejection, for debugging. */
  readonly original: unknown

  constructor(
    message: string,
    opts: {
      kind: ApiErrorKind
      status?: number
      problem?: ProblemDetail
      scriptError?: ScriptError
      correlationId?: string
      original: unknown
    }
  ) {
    super(message)
    this.name = 'ApiError'
    this.kind = opts.kind
    this.status = opts.status
    this.problem = opts.problem
    this.validationErrors = opts.problem?.errors ?? []
    this.scriptError = opts.scriptError
    this.correlationId = opts.correlationId
    this.original = opts.original
  }
}

function isAxiosCancellation(error: AxiosError): boolean {
  // axios surfaces aborts as ECONNABORTED (legacy) or ERR_CANCELED (new);
  // fetch-style paths as DOMException name=AbortError.
  return (
    error.code === 'ECONNABORTED' ||
    error.code === 'ERR_CANCELED' ||
    error.name === 'CanceledError' ||
    error.name === 'AbortError'
  )
}

/**
 * Normalize an axios rejection into an ApiError. Pure — exported for
 * tests, which pin the boundary contract for all four failure shapes.
 */
export function normalizeApiError(error: AxiosError): ApiError {
  if (isAxiosCancellation(error)) {
    return new ApiError('Request cancelled', { kind: 'cancelled', original: error })
  }

  const response = error.response
  if (!response) {
    return new ApiError('Network error — server unreachable', {
      kind: 'network',
      original: error,
    })
  }

  const data = response.data
  if (isScriptError(data)) {
    return new ApiError(data.lua?.message || 'Script failed', {
      kind: 'script',
      status: response.status,
      scriptError: data,
      correlationId: data.correlation_id,
      original: error,
    })
  }

  if (data && typeof data === 'object' && 'type' in data && (data as ProblemDetail).type) {
    const problem = data as ProblemDetail & { correlation_id?: string }
    return new ApiError(
      problem.detail || problem.title || `Request failed (${response.status})`,
      {
        kind: 'http',
        status: problem.status ?? response.status,
        problem,
        correlationId: problem.correlation_id,
        original: error,
      }
    )
  }

  // Response without a structured body (proxy error page, empty body, …).
  return new ApiError(`Request failed (${response.status})`, {
    kind: 'http',
    status: response.status,
    original: error,
  })
}

/**
 * Interceptor entry point: wrap axios rejections, pass everything else
 * (programming errors, already-normalized ApiErrors) through unchanged.
 */
export function toApiError(error: unknown): unknown {
  if (error instanceof ApiError) return error
  if (axios.isAxiosError(error)) return normalizeApiError(error)
  return error
}

/** The one way to turn a caught error into a user-facing message. */
export function getErrorMessage(err: unknown, fallback = 'Something went wrong'): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error && err.message) return err.message
  if (typeof err === 'string' && err) return err
  return fallback
}

/**
 * Extract the script-failure envelope from a caught rejection, for
 * routing to ScriptErrorPanel. Accepts both the normalized ApiError and
 * a bare envelope (e.g. from code paths outside the shared client).
 */
export function getScriptError(err: unknown): ScriptError | null {
  if (err instanceof ApiError) return err.scriptError ?? null
  if (isScriptError(err)) return err
  return null
}
