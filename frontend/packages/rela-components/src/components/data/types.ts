/**
 * Someone who can be shown or chosen.
 *
 * `id` rather than the name as the key, because two people share a name more
 * often than a product expects, and an assignment that moves when someone is
 * renamed is a bug that only appears in production.
 */
export interface Person {
  id: string
  name: string
  /** A photo. Initials are drawn when there is none. */
  avatarUrl?: string
  /**
   * A line under the name, such as a role or an email address. The thing
   * that tells two people with the same name apart.
   */
  secondary?: string
  /** Whether the person can be chosen. They still show, greyed. */
  disabled?: boolean
}
