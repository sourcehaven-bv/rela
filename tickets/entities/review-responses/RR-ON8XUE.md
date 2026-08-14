---
id: RR-ON8XUE
type: review-response
title: AC3 is unverifiable by the proposed tests; the Playwright check is the criterion, not a follow-up
finding: 'AC3 is ''Operator CSS wins an equal-specificity tie against a route-chunk selector AFTER client-side
  navigation''. The plan concedes a Go test cannot evaluate CSS and substitutes two proxies (a build-output
  test that every .css is layer-wrapped, and a compileStyle unit test), then says a real end-to-end check
  ''belongs in e2e/ — record as follow-up if not done here''. That clause guts the ticket. The proxies
  verify the MECHANISM (CSS is wrapped) but not the PROPERTY (operator wins). They cannot detect: an operator
  <link> injected in the wrong position or not at all; the !important inversion (RR-8R8U0B); a route chunk
  loaded by a path bypassing the plugin; or the layer failing to be honored after __vitePreload appends
  a chunk at runtime. The entire reason this ticket exists is a MEASURED browser behavior that static
  analysis of the build output completely failed to predict — the original proposal''s static reasoning
  said the operator wins, and the browser said otherwise. Substituting a static test for the browser check
  re-commits the original error inside the acceptance criterion for the fix.'
severity: critical
status: addressed
resolution: Implemented as e2e/tests/customisation.spec.ts — a real Playwright test that writes custom.css,
  loads the app, navigates CLIENT-SIDE so a route chunk is appended after the operator link, and asserts
  getComputedStyle resolves to the operator value. Not deferred. 5 e2e tests pass.
---

## Failure scenario

Build-output test green, `compileStyle` test green, ticket closes. Later a Vite
upgrade changes chunk loading to inject `<style>` tags instead of `<link>`s
(Vite has done this before for small assets), or a plugin-ordering change lets a
chunk escape `generateBundle`. The build test still passes. Nobody notices until
an operator reports their skin dying on list views — the identical bug this
ticket was opened to fix.

## Recommended resolution

The Playwright e2e check is **not optional** and must not be deferred. It is the
only test that verifies AC3 as written, and the infrastructure already exists:
`e2e/` builds a real bundle via `npm run build:e2e` (`justfile:391`).

One test: serve a `custom.css` with an equal-specificity rule, load the app,
navigate client-side to a list view, `getComputedStyle`, assert the operator
value. That is a direct transcription of the manual browser probe already run by
hand during planning.

Either implement it, or downgrade AC3 to honestly describe what is actually
verified. Do not close the ticket with AC3 asserted-but-unverified.
