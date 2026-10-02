/**
 * Positions a floating panel against the control that opens it.
 *
 * Static `top`/`left` rules place a panel correctly only in the middle of the
 * page. Near an edge the panel runs off screen, and inside an ancestor with
 * `overflow: hidden` it gets clipped — a menu on the last row of a scrolling
 * table loses its bottom half. This wraps Floating UI with the defaults every
 * overlay here wants, so each component states its placement and nothing else.
 *
 * Two things follow from the panel being teleported to `<body>`:
 * focus checks must consult `panel` as well as the trigger's root (see
 * `containsTarget`), and the panel is positioned in viewport coordinates, so
 * it sits above page content without inheriting a clipping ancestor.
 *
 * A panel whose rows are not focusable needs a third thing: see
 * `keepFocusOnTrigger`.
 */
import type { MaybeRefOrGetter, Ref } from 'vue'
import { computed, toValue } from 'vue'
import { autoUpdate, flip, offset, shift, size, useFloating } from '@floating-ui/vue'
import type { Placement } from '@floating-ui/vue'

export interface AnchoredPanelOptions {
  /** Preferred side, and the cross-axis edge to line up with. Reactive. */
  placement?: MaybeRefOrGetter<Placement>
  /** Gap between trigger and panel, in pixels. */
  gap?: number
  /** Keep this much clear of the viewport edge, in pixels. */
  padding?: number
  /**
   * Tie the panel's width to the trigger's. For a field panel such as the
   * multi-select, where a panel narrower than its control reads as a
   * different control rather than as part of it.
   *
   * A floor, not a fixed size. A trigger is often much narrower than the
   * options it opens, such as a filter reading "All" over options reading
   * "Visual regression". A panel pinned to the trigger's width squeezes every
   * option to fit it, which is worse than a panel a little wider than the
   * field. So the panel is at least the trigger's width and grows to its
   * content, up to what the viewport allows.
   */
  matchWidth?: boolean
  /**
   * Cap the panel's height to the space available and let it scroll. Off for
   * a tooltip, which should flip to a side that fits rather than scroll.
   */
  clampHeight?: boolean
  /**
   * For a panel that never takes focus: real focus stays on the trigger the
   * whole time, and the rows are announced through `aria-activedescendant`
   * or are labels wrapping their own control.
   *
   * Such a panel has to suppress the default action of `mousedown` on
   * itself. Pressing a row that cannot hold focus otherwise moves focus to
   * the body, and the trigger's `focusout` then unmounts the panel before
   * the click reaches the row — so the press lands on nothing and the
   * option never registers. Binding this to the panel is what keeps the
   * press from moving focus in the first place.
   *
   * Off by default, and wrong for a panel that does take focus: a menu or a
   * popover moves focus into itself on open, and suppressing the press would
   * stop its own controls from ever being focused.
   */
  keepFocusOnTrigger?: boolean
}

export function useAnchoredPanel(
  trigger: Ref<HTMLElement | null>,
  panel: Ref<HTMLElement | null>,
  options: AnchoredPanelOptions = {},
) {
  const {
    placement = 'bottom-end',
    gap = 4,
    padding = 8,
    matchWidth = false,
    clampHeight = true,
    keepFocusOnTrigger = false,
  } = options

  const { floatingStyles, placement: actual } = useFloating(trigger, panel, {
    placement: computed(() => toValue(placement)),
    // Panels are positioned against the viewport, so a scrolling or clipping
    // ancestor cannot cut them off.
    strategy: 'fixed',
    // Only while the panel is on screen: this attaches scroll and resize
    // listeners, and Floating UI detaches them when the panel unmounts.
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(gap),
      // Flip before shift, so a panel with room on the other side moves there
      // whole rather than being nudged half off the edge it started on.
      flip({ padding }),
      shift({ padding }),
      size({
        padding,
        apply({ availableHeight, availableWidth, rects, elements }) {
          Object.assign(elements.floating.style, {
            maxHeight: clampHeight ? `${availableHeight}px` : '',
            /*
             * `min-width` rather than `width`, so a narrow trigger sets the
             * floor and the options decide the rest. `max-width` stops that
             * growth at the edge of the screen, which is the only thing the
             * content cannot be trusted with.
             */
            minWidth: matchWidth ? `${rects.reference.width}px` : '',
            maxWidth: matchWidth ? `${availableWidth}px` : '',
          })
        },
      }),
    ],
  })

  /**
   * True when `target` is the trigger, inside it, or inside the panel. The
   * `focusout` handlers need this: once the panel is teleported it is no
   * longer a descendant of the trigger's root, so a `root.contains` check
   * alone reports every move into the panel as a move out of the overlay.
   */
  function containsTarget(root: HTMLElement | null, target: Node | null): boolean {
    if (!target) return false
    return Boolean(root?.contains(target)) || Boolean(panel.value?.contains(target))
  }

  /**
   * Bind to the panel element with `v-bind`. Empty unless
   * `keepFocusOnTrigger` is set, so a panel that takes focus is untouched.
   */
  const panelHandlers = computed(() =>
    keepFocusOnTrigger
      ? { onMousedown: (event: MouseEvent) => event.preventDefault() }
      : {},
  )

  return {
    floatingStyles,
    /** The side actually used, after any flip. For a tooltip's arrow. */
    side: computed(() => actual.value.split('-')[0] as 'top' | 'bottom' | 'left' | 'right'),
    containsTarget,
    panelHandlers,
  }
}
