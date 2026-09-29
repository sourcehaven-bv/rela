/**
 * The shapes the comment components share.
 *
 * A comment is anchored: it points at something on the page rather than
 * floating beside it. The library does not know what an anchor refers to — it
 * carries a kind, a reference and a human label, and the app resolves them.
 */

/**
 * What a comment is attached to.
 *
 * `property` and `section` name a part of the record; `text` quotes a passage
 * of prose; `block` points at something that has no text to quote, such as an
 * image or a diagram.
 */
export type CommentAnchorKind = 'property' | 'section' | 'text' | 'block'

export interface CommentAnchor {
  kind: CommentAnchorKind
  /** The property name, section id, or block key this points at. */
  ref: string
  /** Shown instead of `ref` when the raw reference is not readable. */
  label?: string
  /** The quoted passage, for a `text` anchor. */
  quote?: string
}

/**
 * One anchored comment.
 *
 * `editable` and `deletable` describe what the UI offers, not what the user is
 * allowed to do: the server re-authorises every call, so a control may be
 * present for an action that still comes back refused.
 */
export interface AnchoredComment {
  id: string
  author: string
  /** Preformatted by the caller; the library does no date handling. */
  timestamp: string
  body: string
  anchor: CommentAnchor
  resolved?: boolean
  /**
   * The thing this points at is gone — a deleted property, or a passage that
   * no longer appears in the text. The comment survives; its anchor does not.
   */
  detached?: boolean
  editable?: boolean
  deletable?: boolean
}

/** An anchor a new comment may be attached to, for the anchor picker. */
export interface CommentAnchorOption {
  /** Unique across kinds, since a property and a section may share a name. */
  key: string
  label: string
  anchor: CommentAnchor
}

/** A new comment, as emitted by any of the composers. */
export interface NewComment {
  anchor: CommentAnchor
  body: string
}
