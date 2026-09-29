import { api } from './client'

/** The person entity the current principal resolves to, as the principal may read it. */
export interface MePerson {
  type: string
  id: string
  title: string
  /** A URL for an `<img src>`, when `account.avatar_property` is set and filled. */
  avatar?: string
}

/**
 * Pages the login proxy owns, from `account:` in data-entry.yaml. Each is a
 * host-absolute path or an https URL, opened as a full page load: the proxy,
 * not rela, handles CSRF and confirmation there.
 */
export interface MeLinks {
  sign_out?: string
  account?: string
  switch_org?: string
  admin?: string
}

/** `GET /_me`: who the request principal is, for the account menu (TKT-MJTD12). */
export interface MeResponse {
  /** The principal's user id. Always present. */
  user: string
  email?: string
  org?: { id: string; slug?: string; name?: string }
  /** Roles asserted by the login proxy's token, not rela's ACL roles. */
  roles?: string[]
  person?: MePerson
  links?: MeLinks
}

/**
 * Fetches the current principal. Display only: nothing in the SPA may decide
 * access from it, since the server authorizes every request on its own.
 */
export async function getMe(): Promise<MeResponse> {
  return api.get<MeResponse>('/_me')
}
