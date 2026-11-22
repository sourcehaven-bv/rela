/**
 * A JSON API call that is answered with an HTML page.
 *
 * A proxy in front of rela can answer an API call with its own HTML page and
 * a 200, for example a sign-in page after the session ended. axios hands that
 * page on as a string, and a component that expects a list then fails on it
 * (`commands.value.map is not a function`). The client rejects such a
 * response instead, so a caller only ever receives the data it asked for
 * (BUG-JRJDSVJ).
 */
import type { AxiosResponse } from 'axios'
import { ApiError } from './errors'
import { t } from '@/i18n'

/**
 * Whether a successful response to a JSON call is an HTML page.
 *
 * HTML only, not every non-JSON type: an action may answer with its own
 * text, CSV or XML body (script.validActionContentTypes), which callers and
 * embedded apps read, and that allowlist has no HTML. So an HTML answer on
 * the API comes from something in front of rela. A body without a content
 * type counts when it starts with markup. An empty body (a 204) passes, as
 * does a call that asked for another response type, such as a blob.
 */
export function isHtmlResponse(response: AxiosResponse): boolean {
  const responseType = response.config?.responseType
  if (responseType && responseType !== 'json') return false
  if (typeof response.data !== 'string' || response.data === '') return false
  const contentType = String(response.headers?.['content-type'] ?? '').toLowerCase()
  if (!contentType) return response.data.trimStart().startsWith('<')
  return /^(text\/html|application\/xhtml\+xml)\b/.test(contentType)
}

/** The error an HTML answer to a JSON call rejects with. */
export function htmlResponseError(response: AxiosResponse): ApiError {
  const message = t('errors.unexpectedResponse')
  return new ApiError(message, {
    kind: 'http',
    status: response.status,
    problem: { type: 'unexpected_response', title: message, status: response.status },
    original: response,
  })
}
