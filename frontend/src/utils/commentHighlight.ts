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
 *
 * # Code is skipped, not marked
 *
 * Inside a code span or fence, markdown renders HTML LITERALLY — so a mark
 * inserted there shows the user `<mark data-comment-id="...">` as text instead
 * of a highlight. Any range touching code is therefore left unmarked; the
 * comment still lists in the panel, it just gets no highlight. Rendering the
 * markup raw would be strictly worse than rendering nothing.
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
  const codeSpans = findCodeSpans(body)
  const usable = selectNonOverlapping(ranges, bytes.length).filter(
    (r) => !overlapsCode(r, codeSpans)
  )
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

/** A byte range of the source that markdown renders verbatim. */
interface ByteSpan {
  start: number
  end: number
}

/**
 * Finds the byte ranges of every construct markdown renders verbatim: fenced
 * blocks, INDENTED (4-space) blocks, and inline code spans.
 *
 * Deliberately conservative — anything it over-claims merely loses a highlight,
 * which is the safe direction, while anything it MISSES renders raw `<mark…>`
 * markup at the user. Indented blocks were missed at first and did exactly
 * that.
 */
function findCodeSpans(body: string): ByteSpan[] {
  const enc = new TextEncoder()
  const spans: ByteSpan[] = []
  const covered = (from: number, to: number) =>
    spans.some((f) => {
      const s = byteSpanOf(enc, body, from, to)
      return s.start >= f.start && s.end <= f.end
    })

  // Fenced blocks first, so their contents are not re-read as anything else.
  const fence = /^[ \t]*(`{3,}|~{3,})[^\n]*\n[\s\S]*?(?:^[ \t]*\1[ \t]*$|$)/gm
  let m: RegExpExecArray | null
  while ((m = fence.exec(body)) !== null) {
    spans.push(byteSpanOf(enc, body, m.index, m.index + m[0].length))
  }

  // Indented code blocks: runs of lines starting with 4 spaces or a tab.
  //
  // This over-claims a deeply nested list item, which markdown may render as a
  // list rather than code. That costs a highlight on a nested bullet and is the
  // trade this function is explicitly biased toward — the alternative is
  // guessing list context here, and guessing WRONG shows raw markup.
  const indented = /^(?:(?: {4}|\t)[^\n]*\n?)+/gm
  while ((m = indented.exec(body)) !== null) {
    if (!covered(m.index, m.index + m[0].length)) {
      spans.push(byteSpanOf(enc, body, m.index, m.index + m[0].length))
    }
  }

  // Inline spans, skipping anything already inside a block.
  const inline = /(`+)(?:[\s\S]*?)\1/g
  while ((m = inline.exec(body)) !== null) {
    if (!covered(m.index, m.index + m[0].length)) {
      spans.push(byteSpanOf(enc, body, m.index, m.index + m[0].length))
    }
  }
  return spans
}

/** Converts UTF-16 string indexes to the byte offsets the server speaks. */
function byteSpanOf(enc: TextEncoder, body: string, from: number, to: number): ByteSpan {
  return {
    start: enc.encode(body.slice(0, from)).length,
    end: enc.encode(body.slice(0, to)).length,
  }
}

/** True when the range touches any code span at all — even partially. */
function overlapsCode(r: HighlightRange, spans: ByteSpan[]): boolean {
  return spans.some((s) => r.start < s.end && s.start < r.end)
}
