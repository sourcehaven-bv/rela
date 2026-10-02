import type { Component } from 'vue'

export type StatusColor = 'green' | 'amber' | 'red' | 'grey' | 'blue'

export type TagColor = 'grey' | 'blue' | 'green' | 'amber' | 'red' | 'purple'

export interface Tag {
  id: string
  label: string
  color?: TagColor
}

export interface Task {
  id: string
  title: string
  tags?: Tag[]
  assignee?: string
  dueDate?: string
  commentCount?: number
  subtaskCount?: number
  status?: string
  priority?: Tag
}

/**
 * The minimum a collection component needs from a row: something to key it by
 * and something to name it. Everything else a row carries is the consumer's,
 * reached through a cell or card slot.
 */
export interface CollectionItem {
  id: string
  title: string
}

/**
 * One titled group of rows. Generic over the row, so a table or a board works
 * on whatever record the consumer holds; `Task` is only this library's own
 * demo row.
 */
export interface Section<T extends CollectionItem = CollectionItem> {
  id: string
  title: string
  color?: StatusColor
  count?: number
  collapsed?: boolean
  /**
   * A glyph drawn before the title, carried as the component itself rather
   * than a name to look up.
   *
   * A name would have to resolve through this library's icon registry, which
   * makes a section unable to use an icon the registry does not have and
   * forces any app with its own registry to map between the two vocabularies.
   * Taking the component means an app passes whatever its own resolver
   * returns, and `RlIcon` is just one thing that can be passed. This is the
   * same choice `NavItem.as` makes, for the same reason.
   */
  icon?: Component
  items: T[]
}

/**
 * One horizontal band of a swimlane board: a title, and that lane's items
 * already split across the board's columns.
 *
 * A lane holds sections rather than items because the split is the caller's
 * decision. The same item can only appear in one column, so the caller that
 * knows what a column means is the one that can place it; the board reads
 * `sections` in the order the lane gives them.
 */
export interface Swimlane<T extends CollectionItem = CollectionItem> {
  id: string
  title: string
  color?: StatusColor
  /** Total across the lane's sections. Absent means count the items. */
  count?: number
  collapsed?: boolean
  /** A glyph drawn before the lane's title. See `Section.icon`. */
  icon?: Component
  sections: Section<T>[]
}

export interface NavItem {
  id: string
  label: string
  /** Single-letter badge shown in the sidebar, e.g. "R", "J", "T". */
  initial?: string
  icon?: string
  children?: NavItem[]
  expanded?: boolean
  /**
   * What this row renders as, overriding the sidebar's own default. One list
   * can hold both kinds: an entry that goes to a URL is a link, and an entry
   * that runs an action is a button.
   *
   * Carried as the component itself, never as a name to look up: a name
   * resolved at render time would make a config string able to choose what
   * gets mounted, which is the boundary `resolveIcon` exists to hold.
   */
  as?: 'button' | 'a' | Component
  /**
   * Extra attributes for the rendered element, such as `to` for a router
   * link or `href` and `target` for a plain anchor.
   */
  attrs?: Record<string, unknown>
  /**
   * Which subheading this item falls under once its group is long enough to
   * need them, matched against `NavGroup.subgroups`.
   *
   * This is not a second grouping system. `NavGroup` is a decision the caller
   * made about what belongs together, and it holds at every size; this is
   * presentation the sidebar applies inside one group and drops again when
   * the group is short enough to read without it.
   */
  groupId?: string
  /**
   * Whether this item opens a panel over the content rather than navigating.
   *
   * Changes what the row claims about itself: it reports `aria-expanded`
   * against the sidebar's `flyoutId`, so a screen reader announces both that
   * the row opens something and whether that thing is open now. An item that
   * navigates must not set this, because `aria-expanded` on a link would
   * describe something the link does not do.
   *
   * Only the claim lives here. Which panel is open is the app's, passed to
   * `RlSidebar` as `flyoutId`, because that is usually part of the route.
   */
  opensFlyout?: boolean
  /**
   * A state worth flagging on the row: unread items, a failing sync, a
   * section still being set up.
   *
   * Says what is true rather than what to draw, so the sidebar picks the
   * glyph and colour and every row flagged the same way looks the same. A
   * caller that hard-coded an icon here would be deciding presentation one
   * row at a time.
   */
  status?: NavItemStatus
}

/**
 * The states a nav item can flag, and what each is for.
 *
 * Kept to five, because this is read at a glance down the side of a window:
 * a vocabulary the user has to learn is not glanceable. Each maps to one
 * glyph and one colour, both decided by the sidebar.
 */
export type NavItemTone = 'new' | 'info' | 'warning' | 'error' | 'success'

export interface NavItemStatus {
  tone: NavItemTone
  /**
   * What the indicator means, for a screen reader and for the row's tooltip.
   * Required, because an icon alone is not a message: "3 unread" and "sync
   * failed" are both a red glyph to someone who cannot see the colour.
   */
  label: string
  /**
   * A number shown beside the glyph, such as an unread count. Left out for a
   * state that has no quantity, which is most of them.
   */
  count?: number
}

/** A subheading offered inside a group, used only once the group is long. */
export interface NavSubgroup {
  id: string
  label: string
}

export interface NavGroup {
  id: string
  label?: string
  items: NavItem[]
  showMenu?: boolean
  /**
   * Gives the heading an add control, named by this label ("New topic").
   * Pressing it emits the sidebar's `groupAdd`; what gets added is the app's
   * business.
   */
  addLabel?: string
  /**
   * Subheadings for this group's items, in the order they should appear.
   *
   * Applied only past `RlSidebarGroup`'s threshold, so the same data reads as
   * a plain list while it is short and gains its subheadings when it grows.
   * Items with no `groupId`, or one that matches nothing here, collect under
   * a trailing fallback rather than disappearing.
   */
  subgroups?: NavSubgroup[]
}

export type AttachmentKind = 'pdf' | 'doc' | 'sheet' | 'image' | 'other'

export interface Attachment {
  id: string
  name: string
  kind: AttachmentKind
  /** The word after the kind, such as `Download`. */
  action?: string
  /** Where the file downloads from. With one, the card is a download link. */
  href?: string
  /** An image shown in place of the kind icon, such as a thumbnail. */
  preview?: string
}

export interface Comment {
  id: string
  author: string
  timestamp: string
  body: string
}

export interface DetailField {
  id: string
  label: string
  type: 'status' | 'text' | 'tag' | 'tags' | 'date'
  value?: string
  status?: StatusColor
  tags?: Tag[]
}

/**
 * One choice for an editable status field: the label the user picks and the
 * colour its dot takes once picked.
 */
export interface StatusOption {
  value: string
  label: string
  status?: StatusColor
}
