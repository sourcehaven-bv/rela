/**
 * The markdown serialization contract for the Milkdown editor.
 *
 * rela stores entity bodies as markdown on disk, and `internal/markdown`
 * canonicalizes them into a cross-backend content hash. A WYSIWYG editor
 * parses that markdown into a document tree and writes it back out, so every
 * save risks rewriting bytes the user never touched.
 *
 * Two separate properties matter, and they are not the same thing:
 *
 *   1. Meaning is preserved. The parsed tree of the output must match the
 *      parsed tree of the input. This is the correctness bar, and it is
 *      non-negotiable.
 *   2. Bytes are stable. Re-serializing produces the same text, so opening an
 *      entity and saving it without edits does not churn the git history.
 *
 * The options below buy (2) by matching the conventions already present in the
 * corpus. `assertNoSemanticDrift` in the corpus test enforces (1).
 */
import type { Options as RemarkStringifyOptions } from 'remark-stringify'

/**
 * Serializer options chosen to match the existing corpus conventions, so a
 * round-trip through the editor is a no-op on untouched content.
 *
 * Each value is a deliberate match against what `internal/markdown` and the
 * existing `.md` files already produce. Changing one silently reformats every
 * entity body that passes through the editor.
 */
export const RELA_STRINGIFY_OPTIONS: RemarkStringifyOptions = {
  // `-` for bullets: what the corpus uses, and what goldmark-markdown emits.
  bullet: '-',
  // `_` for emphasis and `*` for strong, so `**bold**` and `_italic_` survive.
  emphasis: '_',
  strong: '*',
  // One space of indent under a list item, matching the corpus.
  listItemIndent: 'one',
  // Fenced code blocks, never indented ones. An indented block loses its
  // language tag, which the renderer needs for syntax highlighting.
  fences: true,
  // `-` for thematic breaks, written as `---` with no spaces between the
  // markers. remark's own default is `***`, which would rewrite every
  // horizontal rule in the corpus on first save.
  rule: '-',
  ruleRepetition: 3,
  ruleSpaces: false,
  // ATX headings without a closing run of hashes, and never setext. A setext
  // heading cannot express depth 3 and deeper, so allowing it would make the
  // heading style depend on the level.
  setext: false,
  closeAtx: false,
}

/**
 * A structural view of an mdast node.
 *
 * mdast's own `Node` is a discriminated union with no index signature, so
 * reading an arbitrary property off it does not typecheck. Comparison is
 * inherently generic over node types, so the union is narrowed to this
 * read-only shape at the entry points below.
 */
interface MdastLike {
  type: string
  value?: unknown
  children?: readonly MdastLike[]
}

/** Reads a named property off a node without widening the mdast union. */
function prop(node: MdastLike, key: string): unknown {
  return (node as unknown as Record<string, unknown>)[key]
}

/**
 * Node properties that carry meaning and must match between two trees.
 *
 * Deliberately excludes `position` (byte offsets, which always differ after a
 * round-trip) and any field that only records how the source was written
 * rather than what it says.
 */
const MEANINGFUL_KEYS: Record<string, readonly string[]> = {
  heading: ['depth'],
  link: ['url', 'title'],
  image: ['url', 'title', 'alt'],
  code: ['lang', 'meta'],
  list: ['ordered', 'start'],
  listItem: ['checked'],
  definition: ['identifier', 'url', 'title'],
  linkReference: ['identifier', 'referenceType'],
  imageReference: ['identifier', 'referenceType'],
  footnoteDefinition: ['identifier'],
  footnoteReference: ['identifier'],
  table: ['align'],
}

/**
 * Leaf types whose whitespace does not carry meaning.
 *
 * Deliberately just these two. A round-trip may re-wrap a paragraph or fold a
 * newline inside an inline code span, and neither changes what the text says.
 *
 * `code`, `html` and `yaml` are NOT here, even though they are also literal
 * text. In a fenced block, indentation IS the content — a Python snippet, a
 * YAML fragment, a Go fixture. Collapsing it would make two blocks that differ
 * only in indentation compare as equal, so the guard would classify a
 * reindented code block as suppressible churn and write the corruption back.
 * The corpus measurement that justified normalizing at all only ever observed
 * the inline-code case; applying it to fenced blocks was over-reach.
 */
const WHITESPACE_INSENSITIVE_LEAVES = new Set(['text', 'inlineCode'])

function normalizeLeafValue(type: string, value: unknown): unknown {
  if (typeof value !== 'string') return value
  if (!WHITESPACE_INSENSITIVE_LEAVES.has(type)) return value
  return value.replace(/\s+/g, ' ').trim()
}

/**
 * Reduces an mdast tree to the subset that carries meaning, so two trees can
 * be compared with a plain deep-equality check.
 *
 * Nil: rejected — pass a parsed root node.
 */
export function semanticShape(node: MdastLike): unknown {
  const shape: Record<string, unknown> = { type: node.type }

  for (const key of MEANINGFUL_KEYS[node.type] ?? []) {
    const value = prop(node, key)
    if (value !== undefined && value !== null) {
      shape[key] = value
    }
  }

  if (node.value !== undefined) {
    shape.value = normalizeLeafValue(node.type, node.value)
  }

  if (node.children) {
    shape.children = node.children.map(semanticShape)
  }

  return shape
}

/**
 * Reports whether two parsed markdown trees say the same thing.
 *
 * This is the write-back guard's decision function: a save whose re-serialized
 * body is not semantically equal to what the editor was given is a bug, and
 * the caller must keep the original bytes rather than write the drifted ones.
 */
export function isSemanticallyEqual(a: MdastLike, b: MdastLike): boolean {
  return JSON.stringify(semanticShape(a)) === JSON.stringify(semanticShape(b))
}
