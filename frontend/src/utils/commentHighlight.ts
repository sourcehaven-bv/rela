/**
 * Marking commented text ranges in an entity body (TKT-FIO205 stage 2).
 *
 * # Why the marking happens in SOURCE, before rendering
 *
 * The server resolves each text anchor against the stored markdown and returns
 * byte offsets into it. Those coordinates are only meaningful in the source, so
 * the marks have to be inserted there and carried THROUGH the markdown render —
 * trying to re-find the text in the rendered DOM would mean re-implementing the
 * matcher in the browser, against a document the renderer has already
 * transformed (and that mermaid/PlantUML mutate again afterwards).
 *
 * Marks are emitted as inline HTML because markdown renderers pass raw HTML
 * through. They are stripped by DOMPurify unless the tag and its attributes are
 * allowlisted — see `markdown.ts`.
 */

/** The tag used for a highlight. Must be allowlisted in the sanitiser config. */
export const HIGHLIGHT_TAG = 'mark'

/** Attribute carrying the comment id, so a click can open the right thread. */
export const HIGHLIGHT_ID_ATTR = 'data-comment-id'

/** Attribute set when the anchor resolved below the exact-match band. */
export const HIGHLIGHT_UNCERTAIN_ATTR = 'data-comment-uncertain'

/** A resolved range to mark. Offsets are BYTES into the source body. */
export interface HighlightRange {
  id: string
  start: number
  end: number
  uncertain?: boolean
}

/**
 * Wraps each range in the source with a highlight tag.
 *
 * Ranges are applied back-to-front so each insertion cannot shift the offsets
 * of the ones not yet applied — the standard reason to iterate in reverse here,
 * and the bug that appears immediately if you don't.
 *
 * Overlapping ranges are dropped rather than nested: markdown renderers do not
 * reliably handle interleaved inline HTML, and a half-open tag would corrupt
 * the rest of the document. The dropped comment still appears in the panel, so
 * nothing is lost — it just isn't highlighted.
 *
 * Offsets are byte-based (Go's coordinates); JavaScript strings are UTF-16, so
 * the body is converted to bytes and back rather than sliced directly. Slicing
 * a JS string with a Go byte offset silently mis-cuts any body containing a
 * multi-byte character.
 */
export function applyHighlights(body: string, ranges: HighlightRange[]): string {
  if (!body || ranges.length === 0) return body

  const bytes = new TextEncoder().encode(body)
  const usable = selectNonOverlapping(ranges, bytes.length)
  if (usable.length === 0) return body

  const decoder = new TextDecoder()
  let out = ''
  let cursor = bytes.length

  // Back-to-front: later insertions never disturb earlier offsets.
  for (let i = usable.length - 1; i >= 0; i--) {
    const r = usable[i]
    out =
      openTag(r) +
      decoder.decode(bytes.slice(r.start, r.end)) +
      `</${HIGHLIGHT_TAG}>` +
      decoder.decode(bytes.slice(r.end, cursor)) +
      out
    cursor = r.start
  }
  return decoder.decode(bytes.slice(0, cursor)) + out
}

function openTag(r: HighlightRange): string {
  const uncertain = r.uncertain ? ` ${HIGHLIGHT_UNCERTAIN_ATTR}="true"` : ''
  // The id is server-minted (an opaque token), but it still lands in an HTML
  // attribute, so quote-escape it rather than trusting the generator's alphabet.
  return `<${HIGHLIGHT_TAG} ${HIGHLIGHT_ID_ATTR}="${escapeAttr(r.id)}"${uncertain}>`
}

function escapeAttr(v: string): string {
  return v.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')
}

/**
 * Sorts ranges and drops any that overlap one already kept, or that fall
 * outside the body.
 *
 * An out-of-range offset means the body changed between the read that resolved
 * it and this render — dropping it is right, since marking an arbitrary span
 * would attach a remark to text nobody selected.
 */
function selectNonOverlapping(ranges: HighlightRange[], size: number): HighlightRange[] {
  const sorted = [...ranges]
    .filter((r) => r.start >= 0 && r.end <= size && r.start < r.end)
    .sort((a, b) => a.start - b.start || a.end - b.end)

  const kept: HighlightRange[] = []
  let lastEnd = -1
  for (const r of sorted) {
    if (r.start < lastEnd) continue // overlaps the previous keeper
    kept.push(r)
    lastEnd = r.end
  }
  return kept
}
