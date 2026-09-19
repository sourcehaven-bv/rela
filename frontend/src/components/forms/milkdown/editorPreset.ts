/**
 * The markdown half of the editor, shared by both editors verbatim.
 *
 * Which plugins are loaded and how the serializer is configured together decide
 * what bytes an editor writes. Both the data-entry form and the sandboxed app
 * editor must write the SAME bytes for the same document — they edit the same
 * entity bodies — so this cannot be two configurations that agree by comment.
 *
 * It used to be exactly that: each editor computed its own
 * `commonmark.filter(...)`, with a note in both files saying the two must match.
 * That is the shape of the drift TKT-D2JML7 exists to remove, so the filter and
 * the stringify merge live here and are imported.
 *
 * What is NOT here is anything about the editing EXPERIENCE — the block handle,
 * the slash menu, the toolbar. Those legitimately differ between a form field
 * and a sandboxed element, and none of them changes a byte of output.
 */
import type { Ctx } from '@milkdown/kit/ctx'
import { remarkStringifyOptionsCtx } from '@milkdown/kit/core'
import { commonmark, remarkPreserveEmptyLinePlugin } from '@milkdown/kit/preset/commonmark'
import { RELA_STRINGIFY_OPTIONS } from './serializerContract'
import { relaCommentNode, relaCommentRemarkPlugin } from './commentNode'

/**
 * The commonmark preset with `remarkPreserveEmptyLinePlugin` taken out.
 *
 * That plugin keeps blank lines between paragraphs, but it does so by
 * serializing EVERY empty paragraph as a literal `<br />` — including the empty
 * cells of a table, so adding a row or column wrote raw HTML into the stored
 * markdown.
 *
 * Removing it is a straight improvement rather than a trade: blank runs are
 * preserved exactly as written instead of being normalized, and no `<br />`
 * appears. Filtered here rather than through `Editor.remove`, which returns a
 * promise and would break the builder chain.
 */
const EMPTY_LINE_PLUGIN_PARTS = new Set<unknown>(remarkPreserveEmptyLinePlugin)
export const RELA_COMMONMARK = commonmark.filter((plugin) => !EMPTY_LINE_PLUGIN_PARTS.has(plugin))

/**
 * Nodes that change what bytes an editor writes, in the order they load.
 *
 * `relaComment` belongs here rather than in each editor's own `.use()` chain
 * because it decides whether an HTML comment serializes through the preset's
 * `html` node or through the chip — a difference in OUTPUT, which is exactly
 * what this file exists to keep identical between the two editors. Registering
 * it per-editor would recreate the drift shape TKT-D2JML7 removed: two chains
 * that must match, with only a comment in each saying so.
 *
 * The remark plugin must load with the node. It retypes comment-only `html`
 * mdast nodes to `relaComment`, which is what wins the match against the
 * preset (see `commentNode.ts`); the node alone would silently lose.
 *
 * `entityRefNode` is deliberately NOT moved here in this change. It has the
 * same claim to belong, but the two editors resolve its titles differently
 * (the SPA from a per-principal mentions map, the app editor not at all), so
 * relocating it is its own piece of work rather than a drive-by.
 */
export const RELA_OUTPUT_NODES = [relaCommentRemarkPlugin, relaCommentNode].flat()

/**
 * Applies rela's serializer options to an editor's ctx.
 *
 * Merges rather than replaces: the defaults carry Milkdown's own remark
 * handlers, and dropping them would break serialization of every node type the
 * presets contribute.
 */
export function configureRelaSerializer(ctx: Ctx): void {
  ctx.update(remarkStringifyOptionsCtx, (prev) => ({
    ...prev,
    ...RELA_STRINGIFY_OPTIONS,
  }))
}
