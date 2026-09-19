/**
 * Positioning the link panel against the LINK, not against the selection.
 *
 * `TooltipProvider` is the obvious tool and is the wrong one here, for two
 * reasons that are both visible in its source:
 *
 *  1. It anchors to `posToDOMRect(view, from, to)` — the SELECTION. For this
 *     panel the selection is usually a collapsed caret, so the anchor is a
 *     zero-width rect wherever the cursor happens to sit, which is not where
 *     the link is.
 *  2. Its updater is `throttle`d. A throttled position is a position computed
 *     from an earlier view, so the panel visibly lags a click and appears at
 *     the previous anchor — reported as "it tracks the previous mouse
 *     coordinates", which is exactly what a stale rect looks like.
 *
 * Neither is configurable from the outside, so this positions directly. It is
 * a dozen lines of floating-ui, against a rect we compute from the link's own
 * document range.
 */
import { computePosition, flip, offset, shift, autoUpdate } from '@floating-ui/dom'
import type { VirtualElement } from '@floating-ui/dom'
import { posToDOMRect } from '@milkdown/kit/prose'
import type { EditorView } from '@milkdown/kit/prose/view'

export interface LinkPanelAnchor {
  /** Document range of the link the panel describes. */
  from: number
  to: number
}

/**
 * Keeps `panel` positioned under the link at `anchor`.
 *
 * Returns a cleanup function. Calling this again replaces the previous
 * placement, so the caller does not have to track whether one is active.
 *
 * Nil: a null anchor hides the panel rather than leaving it at a stale
 * position — a panel pointing at the wrong link is worse than no panel.
 */
export function positionLinkPanel(
  view: EditorView,
  panel: HTMLElement,
  anchor: LinkPanelAnchor | null
): () => void {
  if (!anchor) {
    panel.dataset.show = 'false'
    return () => {}
  }

  const virtualEl: VirtualElement = {
    // Recomputed on every call rather than captured: `autoUpdate` fires on
    // scroll and resize, and a rect captured once would be wrong by then.
    getBoundingClientRect: () => posToDOMRect(view, anchor.from, anchor.to),
    contextElement: view.dom,
  }

  const place = (): void => {
    void computePosition(virtualEl, panel, {
      // `fixed`, and the panel is `position: fixed` to match.
      //
      // `posToDOMRect` returns VIEWPORT coordinates. The editor shell is
      // `position: relative`, so an absolutely-positioned panel would resolve
      // those numbers against the shell's box instead — offsetting the panel
      // by however far the shell sits down the page, which grows as the form
      // scrolls. That is the second half of the misplacement: the anchor was
      // wrong, and so was the coordinate space it was written into.
      strategy: 'fixed',
      // Below the link. The provider's default is `top`, which for a link on
      // the first line puts the panel over the toolbar beside it. `flip()`
      // still lifts it when there is no room below.
      placement: 'bottom-start',
      middleware: [offset(6), flip(), shift({ padding: 8 })],
    }).then(({ x, y }) => {
      Object.assign(panel.style, { left: `${x}px`, top: `${y}px` })
      panel.dataset.show = 'true'
    })
  }

  return autoUpdate(virtualEl, panel, place)
}
