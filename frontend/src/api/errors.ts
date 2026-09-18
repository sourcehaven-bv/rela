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
/**
 * The user-facing sentence in a problem document.
 *
 * `title` is the message; `detail` is either a JSON pointer naming the
 * offending field (rela's convention, see writeV1Error) or a longer
 * explanation. A pointer is useless on its own in a toast, so it is dropped;
 * anything else is appended to the title.
 */
function problemMessage(problem: ProblemDetail): string {
  const title = problem.title?.trim() ?? ''
  const detail = problem.detail?.trim() ?? ''
  // A pointer is not a sentence. Fall back to the title, which carries the
  // message in this case.
  if (detail.startsWith('/')) return title || detail
  return detail || title
}

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
    // `detail` is the message when it is prose, per RFC 9457. But rela's
    // writeV1Error also passes a JSON POINTER there (`/world`, `/face`) to say
    // WHICH field a refusal is about, putting the sentence in `title`
    // instead — and a toast reading just "/world" names the field while
    // explaining nothing. problemMessage picks whichever field holds prose.
    return new ApiError(
      problemMessage(problem) || `Request failed (${response.status})`,
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

/**
 * True when a view holding already-fetched content must drop it, because the
 * server has just answered that the content is no longer the principal's to
 * see — as opposed to the request merely failing to complete.
 *
 * The distinction matters wherever a view keeps already-fetched content on
 * screen across a refetch (DocumentView, DocumentsPanel). Holding content
 * through a timeout or a 500 is right — the user keeps their place and the
 * toast explains the hiccup. Holding it through a withdrawn permission is
 * not: the server has just said this principal may not read it, so the copy
 * still painted is content they are no longer entitled to (CONTROL-8-03).
 *
 * 404 is in the list because rela's read gate answers a denied entity with a
 * uniform not-found — whether an entity exists is itself a secret, so an
 * access-revoked render is deliberately indistinguishable from a deleted one
 * (see resolveAnchoredDocument in internal/dataentry/export_document.go, whose
 * comment mandates exactly this). 401/403 cover an expired session and a
 * refused config-declared capability.
 *
 * That 404 therefore sweeps in two non-denials, knowingly: an entity someone
 * genuinely deleted, and a document key dropped from data-entry.yaml (which
 * carries a distinct `document_not_found` code and is NOT confidential by
 * rela's config-is-not-a-secret rule). Both are content that no longer exists,
 * so clearing the view is the right answer for them anyway — the cost of the
 * ambiguity is an empty state instead of stale content, not a wrong decision.
 * What is NOT free is the 401 case: the SPA has no global re-authentication
 * handler, so an expired session yields a blank view and a toast with no route
 * back. Confidentiality still wins that trade, but see the bug entity for the
 * follow-up.
 *
 * Do NOT use this to drive a login redirect. It answers "should a view drop
 * content it is holding", not "was the user denied" — a deleted entity is a
 * true result here and must not send anyone to a login screen.
 *
 * Nil: accepted — a non-ApiError rejection is not a denial.
 */
export function shouldDropHeldContent(err: unknown): boolean {
  if (!(err instanceof ApiError)) return false
  return err.status === 401 || err.status === 403 || err.status === 404
}
