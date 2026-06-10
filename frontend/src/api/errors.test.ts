import { describe, it, expect } from 'vitest'
import { AxiosError, AxiosHeaders, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import {
  ApiError,
  normalizeApiError,
  toApiError,
  getErrorMessage,
  getScriptError,
  type ProblemDetail,
} from './errors'
import type { ScriptError } from '@/types/scriptError'

function axiosErrorWith(data: unknown, status = 422, code?: string): AxiosError {
  const config: InternalAxiosRequestConfig = { headers: new AxiosHeaders() }
  const response: AxiosResponse | undefined =
    data === undefined
      ? undefined
      : { data, status, statusText: '', headers: new AxiosHeaders(), config }
  return new AxiosError(`Request failed with status code ${status}`, code, config, {}, response)
}

const problem: ProblemDetail = {
  type: 'about:blank',
  title: 'Validation failed',
  status: 422,
  detail: "relation 'blocks' rejects type 'concept'",
  errors: [{ field: 'status', message: 'unknown value' }],
}

const scriptEnvelope: ScriptError = {
  error: 'script_error',
  correlation_id: 'abc-123',
  script: { surface: 'action', path: 'scripts/x.lua' },
  lua: { message: 'attempt to index a nil value' },
}

// These tests pin the boundary contract (BUG-X9VNE1 why4): what a catch
// site receives for each failure class of the shared client.
describe('normalizeApiError', () => {
  it('wraps a ProblemDetail response with the server message', () => {
    const err = normalizeApiError(axiosErrorWith(problem))
    expect(err).toBeInstanceOf(ApiError)
    expect(err.kind).toBe('http')
    expect(err.status).toBe(422)
    expect(err.message).toBe("relation 'blocks' rejects type 'concept'")
    expect(err.problem).toEqual(problem)
    expect(err.validationErrors).toEqual([{ field: 'status', message: 'unknown value' }])
  })

  it('falls back to title when detail is absent', () => {
    const err = normalizeApiError(axiosErrorWith({ ...problem, detail: undefined }))
    expect(err.message).toBe('Validation failed')
  })

  it('wraps a script_error envelope and keeps it extractable', () => {
    const err = normalizeApiError(axiosErrorWith(scriptEnvelope))
    expect(err.kind).toBe('script')
    expect(err.message).toBe('attempt to index a nil value')
    expect(err.scriptError).toEqual(scriptEnvelope)
    expect(err.correlationId).toBe('abc-123')
  })

  it.each(['ERR_CANCELED', 'ECONNABORTED'])('maps code %s to kind cancelled', (code) => {
    const err = normalizeApiError(axiosErrorWith(undefined, 0, code))
    expect(err.kind).toBe('cancelled')
  })

  it.each(['AbortError', 'CanceledError'])('maps name %s to kind cancelled', (name) => {
    const axiosErr = axiosErrorWith(undefined)
    axiosErr.name = name
    expect(normalizeApiError(axiosErr).kind).toBe('cancelled')
  })

  it('extracts correlation_id from a ProblemDetail body', () => {
    const err = normalizeApiError(axiosErrorWith({ ...problem, correlation_id: 'corr-9' }))
    expect(err.kind).toBe('http')
    expect(err.correlationId).toBe('corr-9')
  })

  it('falls back to the response status when the ProblemDetail omits its own', () => {
    const err = normalizeApiError(axiosErrorWith({ ...problem, status: undefined }, 403))
    expect(err.status).toBe(403)
  })

  it('maps a missing response to kind network', () => {
    const err = normalizeApiError(axiosErrorWith(undefined))
    expect(err.kind).toBe('network')
    expect(err.message).toContain('Network error')
  })

  it('handles an unstructured error body', () => {
    const err = normalizeApiError(axiosErrorWith('<html>bad gateway</html>', 502))
    expect(err.kind).toBe('http')
    expect(err.status).toBe(502)
    expect(err.message).toBe('Request failed (502)')
    expect(err.validationErrors).toEqual([])
  })
})

describe('toApiError', () => {
  it('passes ApiError and non-axios errors through unchanged', () => {
    const already = new ApiError('x', { kind: 'http', original: null })
    expect(toApiError(already)).toBe(already)
    const plain = new TypeError('boom')
    expect(toApiError(plain)).toBe(plain)
  })

  it('normalizes axios errors', () => {
    expect(toApiError(axiosErrorWith(problem))).toBeInstanceOf(ApiError)
  })
})

describe('getErrorMessage', () => {
  it.each<[unknown, string]>([
    [normalizeApiError(axiosErrorWith(problem)), "relation 'blocks' rejects type 'concept'"],
    [new Error('plain failure'), 'plain failure'],
    ['string error', 'string error'],
    [undefined, 'fallback'],
    [{ random: 'object' }, 'fallback'],
    [new Error(''), 'fallback'],
  ])('extracts the best message (%#)', (input, expected) => {
    expect(getErrorMessage(input, 'fallback')).toBe(expected)
  })
})

describe('getScriptError', () => {
  it('unwraps the envelope from a normalized ApiError', () => {
    const err = normalizeApiError(axiosErrorWith(scriptEnvelope))
    expect(getScriptError(err)).toEqual(scriptEnvelope)
  })

  it('accepts a bare envelope', () => {
    expect(getScriptError(scriptEnvelope)).toEqual(scriptEnvelope)
  })

  it('returns null for everything else', () => {
    expect(getScriptError(normalizeApiError(axiosErrorWith(problem)))).toBeNull()
    expect(getScriptError(new Error('x'))).toBeNull()
    expect(getScriptError(null)).toBeNull()
  })
})
