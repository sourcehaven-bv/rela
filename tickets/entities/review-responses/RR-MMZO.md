---
id: RR-MMZO
type: review-response
title: E2E flicker polling at 16ms can miss a fast loading flip and silently regress
finding: 'setInterval(16ms) racing async isVisible() can miss a sub-frame loading flip; PATCH on warm-local rounds-trip <16ms in CI. Test passes; regression lands. Better: instrument the component with a `loadingFlipCount` exposed via a debug global, or use a page-side MutationObserver capturing every truthy v-if transition on the spinner.'
severity: significant
resolution: Replaced the setInterval(16ms) polling with an in-page MutationObserver installed BEFORE the click via `page.evaluate`. Observer watches childList+subtree on document.body and sets `__entityDetailLoadingSeen` true if the scoped selector ever matches. After the toggle settles, the test reads the flag back via page.evaluate. Sub-frame loading flips cannot slip past a MutationObserver — it sees every DOM mutation regardless of timing.
status: addressed
---
