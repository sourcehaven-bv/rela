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
import { canApplyLink, findLinkAt } from './linkSelection'

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

  // Nothing selected means there is no text to wrap. Pasting a URL at a caret
  // should insert the URL as text, which is what ProseMirror already does.
  if (empty) return null

  // The mark has to be legal here. A code block declares `marks: ''`, so
  // checking this BEFORE returning true is what stops a paste into one being
  // swallowed and dropped.
  if (!canApplyLink(state)) return null

  // Already a link: that is a retarget, and it belongs to the dialog where the
  // user can see what they are changing. Falling through also means pasting
  // over a link still replaces its text, which is what a paste normally does.
  if (findLinkAt(state)) return null

  const result = normalizeLinkUrl(text)
  if (!result.ok) return null

  // A multi-line clipboard is prose that happens to start with a URL, not a
  // URL. `normalizeLinkUrl` would already refuse most of these; this keeps the
  // intent explicit.
  if (/\s/.test(text.trim())) return null

  return result.url
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
 * Applying one mark across a multi-block selection yields one link per block
 * on save. That is inherent to inline marks spanning a block boundary, not
 * something to paper over.
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
          if (!href) return false

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
