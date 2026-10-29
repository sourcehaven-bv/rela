/**
 * The token for the signed-in user (TKT-YJSRBZ), in a form field `default:`
 * and in a list or board filter value. The server resolves it in list
 * filters; the SPA resolves it in form defaults and board filters, which it
 * applies itself.
 */
export const USER_TOKEN = '$user'

/**
 * The id `GET /_me` reports for a caller the server could not identify
 * (`principal.Unknown` in internal/principal).
 */
export const UNKNOWN_USER = 'unknown'

/**
 * Resolves `$user` to the principal id from `GET /_me`. It is undefined
 * while that is unknown, so a form field stays empty and a board filter
 * matches nothing rather than comparing against the literal token. Other
 * values pass through.
 */
export function resolveUserToken(value: unknown, user: string | undefined): unknown {
  if (value !== USER_TOKEN) return value
  if (!user || user === UNKNOWN_USER) return undefined
  return user
}
