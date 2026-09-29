/**
 * The markdown half of the editor: which plugins load, and how the serializer
 * is configured.
 *
 * Kept apart from the component because these two things together decide what
 * BYTES the editor writes, while nothing here touches the editing experience.
 * An app that mounts the editor twice, or alongside its own second editor,
 * needs both instances to serialize a document identically; importing one
 * configuration is what makes that true by construction rather than by a
 * comment in two files asking for it.
 */
import type { Ctx } from '@milkdown/kit/ctx'
import { remarkStringifyOptionsCtx } from '@milkdown/kit/core'
import { commonmark, remarkPreserveEmptyLinePlugin } from '@milkdown/kit/preset/commonmark'
import type { Options as RemarkStringifyOptions } from 'remark-stringify'
import { DEFAULT_STRINGIFY_OPTIONS } from './serializerContract'

/**
 * The commonmark preset with `remarkPreserveEmptyLinePlugin` taken out.
 *
 * That plugin keeps blank lines between paragraphs, but it does so by
 * serializing EVERY empty paragraph as a literal `<br />` — including the
 * empty cells of a table, so adding a row or column writes raw HTML into the
 * stored markdown.
 *
 * Removing it is a straight improvement rather than a trade: blank runs are
 * preserved exactly as written instead of being normalized, and no `<br />`
 * appears. Filtered here rather than through `Editor.remove`, which returns a
 * promise and would break the builder chain.
 */
const EMPTY_LINE_PLUGIN_PARTS = new Set<unknown>(remarkPreserveEmptyLinePlugin)
export const MARKDOWN_COMMONMARK = commonmark.filter(
  (plugin) => !EMPTY_LINE_PLUGIN_PARTS.has(plugin)
)

/**
 * Applies the editor's serializer options to an editor's ctx.
 *
 * Merges rather than replaces: the defaults carry Milkdown's own remark
 * handlers, and dropping them would break serialization of every node type
 * the presets contribute.
 */
export function configureSerializer(ctx: Ctx, options?: RemarkStringifyOptions): void {
  ctx.update(remarkStringifyOptionsCtx, (prev) => ({
    ...prev,
    ...DEFAULT_STRINGIFY_OPTIONS,
    ...options,
  }))
}
