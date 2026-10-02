/**
 * The markdown serialization contract for the editor.
 *
 * A WYSIWYG editor parses markdown into a document tree and writes it back
 * out, so every save risks rewriting bytes the user never touched. When the
 * markdown is stored in git, or hashed, or read by humans, that churn is a
 * real cost: a diff appears for a document that was only opened.
 *
 * Two separate properties matter, and they are not the same thing:
 *
 *   1. Meaning is preserved. The parsed tree of the output must match the
 *      parsed tree of the input. This is the correctness bar.
 *   2. Bytes are stable. Re-serializing produces the same text, so opening a
 *      document and saving it without edits changes nothing.
 *
 * The options below buy (2) by picking one spelling for each construct that
 * remark can write more than one way. `isSemanticallyEqual` is the tool for
 * checking (1): an app that writes markdown back to storage can compare the
 * editor's output against what it loaded and refuse a save that drifted.
 *
 * These are defaults, not a fixed contract. An app whose stored markdown uses
 * other conventions should pass its own options through the editor's
 * `stringifyOptions` prop rather than reformatting its corpus to match.
 */
import type { Options as RemarkStringifyOptions } from 'remark-stringify'

/**
 * Serializer options that pick one spelling per construct.
 *
 * Each value resolves an ambiguity in how remark may write a node. Changing
 * one reformats every document that passes through the editor, which is why
 * they are stated explicitly rather than left to remark's defaults.
 */
export const DEFAULT_STRINGIFY_OPTIONS: RemarkStringifyOptions = {
  // `-` for bullets, the most common convention and what most markdown
  // formatters emit.
  bullet: '-',
  // `_` for emphasis and `*` for strong, so `**bold**` and `_italic_` survive.
  emphasis: '_',
  strong: '*',
  // One space of indent under a list item.
  listItemIndent: 'one',
  // Fenced code blocks, never indented ones. An indented block loses its
  // language tag, which a renderer needs for syntax highlighting.
  fences: true,
  // `-` for thematic breaks, written as `---` with no spaces between the
  // markers. remark's own default is `***`, which would rewrite every
  // horizontal rule on first save.
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
 * text. In a fenced block, indentation IS the content. Collapsing it would
 * make two blocks that differ only in indentation compare as equal, so a
 * caller would classify a reindented code block as suppressible churn and
 * write the corruption back.
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
 * Intended as a write-back guard's decision function: a save whose
 * re-serialized body is not semantically equal to what the editor was given
 * means the round trip lost something, and the caller should keep the
 * original bytes rather than write the drifted ones.
 */
export function isSemanticallyEqual(a: MdastLike, b: MdastLike): boolean {
  return JSON.stringify(semanticShape(a)) === JSON.stringify(semanticShape(b))
}
