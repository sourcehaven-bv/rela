---
id: RR-AYFK
type: review-response
title: Selected option has no aria-selected scroll behavior on initial render of search results
finding: |-
    `EntityPickerModal.vue:scrollHighlightedIntoView` is called only from `moveHighlight`, which only runs from `ArrowDown`/`ArrowUp` keydown. On the FIRST render of search results (after the user types a query and Bleve returns matches), `highlightedIndex = 0` and the option is marked `aria-selected="true"`. But:

      - If `results.length` is bigger than the viewport, the user sees only the top portion. That's correct — the highlighted item IS the top one.
      - If results come in via a debounced fetch while the user has scrolled the previous-search's list down, the ul retains its scroll position from the prior render but the highlighted index is reset to 0. The user now sees a 'middle of list' scroll position with no visible highlight — the highlighted item is at the top, off-screen.

    A `scrollHighlightedIntoView()` call inside `runSearch` after `results.value = ...` would fix it. Same fix CommandPaletteModal would also benefit from.

    Accessibility concern: `aria-activedescendant` points at the highlighted option ID, but the option is not rendered in the viewport. Screen-reader navigation per WAI-ARIA combobox pattern relies on the highlighted descendant being scrollable into view by the listbox — it usually is, but only because the LISTBOX is set up with a known scroll position. Worth a check with VoiceOver/NVDA before declaring a11y compliance.
severity: nit
resolution: runSearch now calls scrollHighlightedIntoView() after the new results land and highlightedIndex is reset to 0. Comment in the code explains the aria-activedescendant compliance reason.
status: addressed
---
