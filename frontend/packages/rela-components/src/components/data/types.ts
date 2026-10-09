import type { Component } from 'vue'

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

/** How a value in a related row is coloured. `subtle` is the quiet default. */
export type RelatedTone = 'subtle' | 'muted' | 'info' | 'success' | 'warning' | 'danger'

/**
 * One value at the end of a related row, such as a date or a person.
 *
 * `label` is what the value is ("Due", "Assigned to"). It is read to a screen
 * reader and not drawn, because a row of columns without headings reads as
 * "Aug 23 Rowdy", and only the sighted user can tell which is which from
 * where they sit.
 */
export interface RelatedMeta {
  id: string
  value: string
  label?: string
  tone?: RelatedTone
}

/**
 * One record shown in an `RlRelatedList`: another record linked to the one on
 * screen, such as a subtask, a requirement a decision answers, or a risk a
 * measure covers. The list makes no claim about what the link means.
 */
export interface RelatedItem {
  id: string
  title: string
  /**
   * A glyph before the title, usually the record's state. Taken as the
   * component itself; see `Section.icon`.
   */
  icon?: Component
  /**
   * What the icon says, such as "Done". Read to a screen reader in place of
   * the glyph. Without it the icon is treated as decorative.
   */
  iconLabel?: string
  iconTone?: RelatedTone
  meta?: RelatedMeta[]
  /**
   * What the title renders as. See `NavItem.as`. `'span'` makes the row
   * static: plain text, with no hover, pointer or focus, for a record that is
   * already shown in full where the row sits.
   */
  as?: 'button' | 'a' | 'span' | Component
  /** Attributes for the rendered element, such as `to` or `href`. */
  attrs?: Record<string, unknown>
}
