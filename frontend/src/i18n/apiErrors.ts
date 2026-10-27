/**
 * Translated messages for API error codes.
 *
 * The server writes its messages in English. Under English the SPA shows them
 * as they are, because they are more specific than a generic line ("Entity
 * TKT-1 not found" against "Not found"). In another language a known code is
 * replaced by its translation, and an unknown code keeps the server's message:
 * English text beats no text.
 *
 * Only codes whose meaning is fixed belong here. `conflict` is left out: the
 * server uses it both for a concurrent edit and for an id that is already
 * taken. `forbidden` is left out: its detail names the missing permission,
 * which a generic line would hide.
 *
 * The record is literal on purpose: the unused-key check finds keys by their
 * text, so `errors.${code}` would hide them.
 */
import type { MessageSchema } from './index'

type ErrorKey = `errors.${keyof MessageSchema['errors']}`

export const API_ERROR_KEYS: Readonly<Record<string, ErrorKey>> = {
  not_found: 'errors.notFound',
  unauthenticated: 'errors.unauthenticated',
  precondition_failed: 'errors.preconditionFailed',
  body_too_large: 'errors.bodyTooLarge',
  schema_changed: 'errors.schemaChanged',
  state_changed: 'errors.stateChanged',
  encrypted_inaccessible: 'errors.encryptedInaccessible',
  history_unsupported: 'errors.historyUnsupported',
}

/**
 * The code in a problem document's `type`, which rela writes as
 * `https://rela.dev/errors/<code>`. Undefined for any other shape.
 */
/** The translation key for an API error code, if the code has one. */
export function apiErrorKey(code: string | undefined): ErrorKey | undefined {
  // An own-property check, so a code such as `constructor` cannot reach the
  // object prototype.
  return code && Object.prototype.hasOwnProperty.call(API_ERROR_KEYS, code) ? API_ERROR_KEYS[code] : undefined
}

export function problemCode(type: string | undefined): string | undefined {
  if (!type) return undefined
  const m = /\/errors\/([a-z0-9_]+)$/.exec(type)
  return m?.[1]
}
