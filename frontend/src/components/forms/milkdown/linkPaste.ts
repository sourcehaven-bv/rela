/**
 * Turning "paste a URL over selected text" into a link.
 *
 * The expected behaviour everywhere else: select some words, paste a URL, and
 * the words become a link to it rather than being replaced by the URL. Nothing
 * in the commonmark preset does this — it ships no paste rule at all — so it
 * is a small `$prose` plugin here.
 *
 * The rule is deliberately narrow. `handlePaste` returning `true` SWALLOWS the
 * paste, so every condition that is not met must fall through to ProseMirror's
 * own handling rather than being approximated. Getting this wrong does not
 * produce a wrong link; it produces a paste that silently does nothing.
 */
import { $prose } from '@milkdown/kit/utils'
import { Plugin, PluginKey } from '@milkdown/kit/prose/state'
import type { EditorView } from '@milkdown/kit/prose/view'
import { normalizeLinkUrl } from './linkUrl'
import { canApplyLink, findLinkAt, isWithinOneBlock } from './linkSelection'

export const linkPasteKey = new PluginKey('rela-link-paste')

/**
 * Decides whether this paste should become a link, and returns the href.
 *
 * Split out from the plugin so the decision is testable without a DOM event
 * or a mounted editor. Returns `null` for "not ours", which the caller turns
 * into `false` so the normal paste proceeds.
 */
export function linkPasteHref(view: EditorView, text: string): string | null {
  const { state } = view
  const { empty } = state.selection

  // Whitespace in the RAW clipboard means prose that happens to contain a URL,
  // not a URL. Checked first, against the untrimmed text: `normalizeLinkUrl`
  // strips the characters a browser ignores while resolving a scheme, so by
  // the time it has run, internal whitespace is gone and this can no longer
  // tell the two apart.
  if (/\s/.test(text.trim())) return null

  // Nothing selected means there is no text to wrap. Pasting a URL at a caret
  // should insert the URL as text, which is what ProseMirror already does.
  if (empty) return null

  // One block only. A mark spanning a block boundary becomes one mark per
  // block, so a single pasted URL would turn into a link in each paragraph it
  // touched.
  if (!isWithinOneBlock(state)) return null

  // The mark has to be legal across the WHOLE selection — `canApplyLink` is an
  // all-nodes test for exactly this reason. A code block declares `marks: ''`,
  // and swallowing a paste we cannot then apply loses the user's clipboard.
  if (!canApplyLink(state)) return null

  // Any existing link in range is the dialog's business, not this plugin's.
  // A selection wholly inside one is an ordinary label edit; a partial overlap
  // is refused here and then BLOCKED outright — see `pasteWouldDamageLink`.
  if (findLinkAt(state)) return null

  const result = normalizeLinkUrl(text)
  if (!result.ok) return null

  return result.url
}

/**
 * Whether the selection overlaps a link that a plain paste would damage.
 *
 * Refusing to LINKIFY is not enough on its own. Falling through hands the
 * selection to ProseMirror's default paste, which replaces it — and a
 * selection running from plain prose into a link takes the link apart:
 * `see [docs](url) here` came back as
 * `shttps\://new\.test/[s](url) here`, with the word gone, the link reduced to
 * one character, and the URL left as escaped literal text.
 *
 * So the plugin has to CLAIM this paste and do nothing, rather than decline it.
 * Retargeting a link is the dialog's job, where the user can see what is
 * changing.
 */
export function pasteWouldDamageLink(view: EditorView): boolean {
  const { state } = view
  if (state.selection.empty) return false
  const link = findLinkAt(state)
  if (!link) return false

  // Wholly inside the link is an ordinary text replacement — the link keeps
  // its target and the user is editing its label, which is what a paste over
  // selected text normally means. Only a PARTIAL overlap is destructive.
  const { from, to } = state.selection
  return from < link.from || to > link.to
}

/**
 * Applies the link mark across the selection.
 *
 * `addMark` rather than the `ToggleLink` command: over a selection that
 * already carries a link, `toggleMark` REMOVES it (its `removeWhenPresent`
 * tests `.some()`). That case is filtered out above, so the two would agree
 * here — but naming the operation that cannot misfire keeps the guarantee
 * local instead of resting on a check three functions away.
 *
 * By the time this runs the selection is known to sit inside one block, to
 * accept the mark throughout, and to carry no link already. Returning `true`
 * without those being true is how a paste gets swallowed and the clipboard
 * lost, so they are conditions rather than assumptions.
 */
export const linkPaste = $prose(
  () =>
    new Plugin({
      key: linkPasteKey,
      props: {
        handlePaste: (view, event) => {
          const text = event.clipboardData?.getData('text/plain') ?? ''
          if (!text) return false

          const href = linkPasteHref(view, text)
          if (!href) {
            // Claim the paste and do nothing, rather than declining it: the
            // default handler would replace a selection that straddles a link
            // boundary and destroy the link in the process.
            return pasteWouldDamageLink(view)
          }

          const { state, dispatch } = view
          const { from, to } = state.selection
          const linkType = state.schema.marks.link
          if (!linkType) return false

          dispatch(state.tr.addMark(from, to, linkType.create({ href })))
          return true
        },
      },
    })
)
