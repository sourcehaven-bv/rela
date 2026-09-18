---
id: RR-YLO6CG
type: review-response
title: Mention-menu state machine was forked, not shared
finding: 'The plan claimed relaMentionMenu.ts was ''local because it needs a framework in the SPA'', but
  only the transport and rendering were local: MIN_SEARCH_LEN, the debounce, MAX_RESULTS, the generation
  staleness guard, the backspace-below-minimum reset and the highlight wrap were all duplicated from useMentionMenu.ts,
  in places character for character. That is the same defect class (RR-9PTXV0) the ticket was written
  to remove, shipped at smaller scale. Nothing tested that the two menus agreed about when to search or
  how a slow response resolves.'
severity: significant
resolution: 'Extracted mentionMenuState.ts: a framework-free machine with the transport injected as a
  search callback and an onChange hook for the renderer. useMentionMenu.ts is now a Vue wrapper (reactivity
  + axios) and relaMentionMenu.ts a plain-DOM renderer (+ bridge). Added mentionMenuState.test.ts (13
  tests) covering the staleness rules directly. All 17 existing useMentionMenu tests and all app-editor
  tests pass unchanged, which is what says behaviour was preserved.'
status: addressed
---
