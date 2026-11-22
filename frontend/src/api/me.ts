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
  /**
   * The principal's user id. Always present; `unknown` when the server
   * could not identify the caller (UNKNOWN_USER in utils/userToken).
   */
  user: string
  email?: string
  org?: { id: string; slug?: string; name?: string }
  /** Roles asserted by the login proxy's token, not rela's ACL roles. */
  roles?: string[]
  person?: MePerson
  links?: MeLinks
  /**
   * The server offers the Configure space and this principal may use it.
   * Absent means no. It only decides whether the entry is shown; the
   * Configure API checks the permission on every request itself.
   */
  can_configure?: boolean
  /**
   * This principal holds remote:sync and the server has remotes wired. Absent
   * means no. It only decides whether the Remotes link is shown.
   */
  can_remote_sync?: boolean
  /**
   * This principal may save the settings page (settings:edit, and not a
   * read-only instance). Absent means no. The page leaves out the shared
   * defaults, palette, logo and theme controls without it; every save
   * checks the permission itself.
   */
  can_edit_settings?: boolean
  /**
   * The principal is verified but has no person entity yet, and a POST to
   * `/_me` creates it (`unmatched_principal: provision`). Absent means no.
   */
  provisionable?: boolean
}

/**
 * Fetches the current principal. Display only: nothing in the SPA may decide
 * access from it, since the server authorizes every request on its own.
 *
 * When the server says the principal is `provisionable`, a POST creates its
 * person now, so a member who has only looked around is listed among the
 * people too (BUG-WS7QF2K). If the POST fails, for example in a read-only
 * workspace, the GET's answer stands.
 */
export async function getMe(): Promise<MeResponse> {
  const me = await api.get<MeResponse>('/_me')
  if (!me.provisionable) return me
  try {
    return await api.post<MeResponse>('/_me')
  } catch {
    return me
  }
}
