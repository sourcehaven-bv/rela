/**
 * The `relaComment` ProseMirror node: an HTML comment shown as a muted chip.
 *
 * Templates use HTML comments as authoring guidance — every file under
 * `tickets/templates/entities/` opens its sections with one, and the generated
 * checklists carry them into real entities. On disk that is just markdown, and
 * the rendered view drops them (DOMPurify strips comment nodes), so a reader
 * never sees them. The editor was the one surface that did: the commonmark
 * preset's `html` node renders its value as a bare `<span>` of text, so
 * `<!-- Document what IS and IS NOT in scope -->` sat in the document looking
 * exactly like prose the author was supposed to have written.
 *
 * This node claims those `html` mdast nodes and renders them as a chip
 * carrying only the comment's inner text, so the guidance reads as guidance.
 * The stored markdown does not change: `toMarkdown` writes the original bytes
 * back.
 *
 * ## Why a separate node rather than CSS on the preset's `html` node
 *
 * The preset already emits `data-type="html"`, so a CSS rule could style every
 * raw-HTML node. That would also style `<img>`, `<div>` and `<script>` — the
 * raw markup `rawHtmlPassthrough.test.ts` deliberately leaves visible as text.
 * A comment is the case where hiding the delimiters helps; literal markup is
 * the case where showing them is the entire point. Splitting the node is what
 * lets the two be styled differently.
 *
 * ## Priority
 *
 * Milkdown resolves a markdown node against `{...schema.nodes, ...schema.marks}`
 * and takes the first match. Both this node and the preset's `html` node match
 * mdast `type === 'html'`, so which one wins depends on key order in that
 * spread — insertion order for string keys. The preset is registered before
 * this node in both editors, so `html` would be seen first. `parseMarkdown` is
 * therefore NOT enough on its own; the editors must register this node and the
 * remark plugin below strips comments out of the `html` stream before the
 * preset sees them. See `relaCommentRemarkPlugin`.
 */
import { $node, $remark } from '@milkdown/kit/utils'

/**
 * Matches a string that is exactly one HTML comment and nothing else.
 *
 * Anchored at both ends deliberately. CommonMark lets an HTML block hold a
 * comment followed by other markup (`<!-- note --><div>`), and that node is not
 * a comment chip — it is raw markup that must stay visible as text. `[\s\S]` in
 * place of `.` so a multi-line comment matches: the `Options` block in
 * `research.md` spans five lines inside one mdast node.
 *
 * The body must not itself contain `-->`, which is what stops
 * `<!-- a --> text <!-- b -->` matching as one comment. A lazy quantifier does
 * NOT achieve this: anchored at both ends it simply backtracks until the whole
 * string fits, so the two-comment string matched with a body of
 * ` a --> text <!-- b `. Excluding the terminator explicitly is the fix.
 */
const COMMENT_PATTERN = /^<!--((?:(?!-->)[\s\S])*)-->$/

/**
 * Label length beyond which a `title` tooltip is worth setting.
 *
 * Below it the chip shows the whole comment, so a tooltip would merely repeat
 * text already on screen.
 */
const TITLE_THRESHOLD = 60

/**
 * Extracts the text inside an HTML comment, or null when the value is not
 * exactly one comment.
 *
 * Nil: accepted — a non-string value returns null, since mdast `value` is
 * typed `unknown` at this boundary.
 */
export function commentBody(value: unknown): string | null {
  if (typeof value !== 'string') return null
  const match = COMMENT_PATTERN.exec(value)
  if (!match) return null
  // A matched group is always a string, so `<!---->` yields '' — a FALSY
  // success. Callers must test `=== null`, never truthiness, or an empty
  // comment is misread as "not a comment".
  return match[1]
}

/**
 * True when an mdast node is an HTML comment this node should claim.
 */
export function isCommentNode(node: { type?: string; value?: unknown }): boolean {
  return node.type === 'html' && commentBody(node.value) !== null
}

/**
 * Renames comment-only `html` mdast nodes to `relaComment` before Milkdown
 * matches them against the schema.
 *
 * This is what makes the node win over the preset's `html` node without
 * depending on registration order (see the priority note above). By the time
 * the schema sees the tree, a comment is no longer typed `html`, so the two
 * matchers are disjoint rather than competing.
 *
 * The original bytes are carried on `value` untouched, which is what
 * `toMarkdown` writes back.
 *
 * Two things a reader will reasonably worry about, both checked:
 *
 *  - *Does it reach comments nested in tables, blockquotes and list items?*
 *    Yes — remark emits a flat `html` node for each, and the walk recurses
 *    `children`, so they are claimed like any other.
 *  - *Is mutating the tree in place safe?* Yes. `$remark` plugins are attached
 *    to the shared inbound processor, but `SerializerState.toString` calls
 *    `remark.stringify()` directly and never runs transformers, so the
 *    outbound path cannot re-enter this plugin and re-type its own output.
 */
/** The shape this plugin walks. Declared locally rather than pulled from an
 * mdast types package, since only two fields are touched. */
interface MdastNode {
  type?: string
  value?: unknown
  children?: MdastNode[]
}

export const relaCommentRemarkPlugin = $remark('relaComment', () => () => (tree: unknown) => {
  retypeComments(tree as MdastNode)
})

/**
 * Walks the tree depth-first, retyping every comment-only `html` node.
 *
 * Hand-rolled rather than using `unist-util-visit`: that package is present
 * only as a transitive dependency of remark, so importing it would bind this
 * file to a version nothing declares. The traversal is four lines.
 */
function retypeComments(node: MdastNode): void {
  if (node.type === 'html' && commentBody(node.value) !== null) {
    node.type = 'relaComment'
  }
  for (const child of node.children ?? []) retypeComments(child)
}

/**
 * The `relaComment` node schema.
 *
 * Inline and atomic, matching the preset's `html` node it replaces. Inline is
 * not a style choice: a trailing comment such as
 * `**Research Doc:** <!-- Link RES-xxxx -->` parses as an `html` child of a
 * paragraph, so a block node there would split the paragraph and change the
 * document's shape. A comment that stands alone in the document is an `html`
 * child of `root`, which remark-stringify writes back as its own block — the
 * `data-block` attribute below lets CSS style that case differently without
 * the schema needing two node types.
 *
 * Atomic because a comment is one unit: putting a cursor inside the delimiters
 * and deleting a character would produce markup that is no longer a comment,
 * silently turning guidance into visible raw text.
 */
export const relaCommentNode = $node('relaComment', () => ({
  group: 'inline',
  inline: true,
  atom: true,
  // Selectable so the chip can be selected and deleted as a unit, which is the
  // gesture for clearing a prompt once the section is written.
  selectable: true,
  draggable: false,
  attrs: {
    /** The comment's full source text, delimiters included. Round-tripped verbatim. */
    value: { default: '' },
  },
  parseDOM: [
    {
      tag: 'span[data-type="rela-comment"]',
      getAttrs: (dom) => ({ value: (dom as HTMLElement).dataset.value ?? '' }),
    },
  ],
  toDOM: (node) => {
    const value = node.attrs.value as string
    const body = commentBody(value) ?? value
    // Trimmed for display only; `value` keeps the original spacing so the
    // serializer writes back byte-identical markdown.
    const label = body.trim()
    // A multi-line comment is block-level guidance (the `Options` scaffold in
    // research.md); a single-line one may be either. The distinction drives
    // layout only, so it is computed here rather than stored.
    //
    // Tested against `body`, NOT `label`: trimming removes the edge newlines
    // first, so a comment opened on its own line (`<!--\nfoo\n-->` — the most
    // common template shape) would be misread as inline and lose both the
    // block layout and the `pre-wrap` that preserves its newlines.
    const isBlock = body.includes('\n')
    const attrs: Record<string, string> = {
      'data-type': 'rela-comment',
      'data-value': value,
      class: isBlock ? 'rela-comment rela-comment-block' : 'rela-comment',
      // Only when the chip may actually clip: a `title` duplicating fully
      // visible text produces a redundant hover tooltip.
      ...(isBlock || label.length > TITLE_THRESHOLD ? { title: label } : {}),
    }
    // The third element is a CHILD, so ProseMirror builds it with
    // `createTextNode` rather than assigning innerHTML. A comment body is
    // author-controlled but still untrusted, and Milkdown has shipped this
    // vulnerability class twice (CVE-2026-57530, CVE-2026-57531), so the text
    // sink here is load-bearing. `rawHtmlPassthrough.test.ts` pins it.
    return ['span', attrs, label]
  },
  parseMarkdown: {
    match: ({ type }) => type === 'relaComment',
    runner: (state, node, type) => {
      state.addNode(type, { value: node.value as string })
    },
  },
  toMarkdown: {
    match: (node) => node.type.name === 'relaComment',
    runner: (state, node) => {
      // Written back as an `html` mdast node holding the original bytes. The
      // write-back guard compares parsed trees and `serializerContract.ts`
      // excludes `html` from whitespace-insensitive leaves, so anything less
      // than byte-exact here is reported as semantic drift rather than
      // silently rewriting the author's file.
      state.addNode('html', undefined, node.attrs.value as string)
    },
  },
}))
