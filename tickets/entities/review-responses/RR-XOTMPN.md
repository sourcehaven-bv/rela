---
id: RR-XOTMPN
type: review-response
title: '@layer sweeps in tokens.css, silently voiding the byte-identical app-token contract'
finding: 'frontend/src/styles/tokens.css is pinned byte-for-byte to internal/dataentry/apps_tokens.css by TestAppTokensCSSInSyncWithFrontend (apps_test.go:119), and that Go copy is served to sandboxed custom apps as _rela.css. VERIFIED: the two files are byte-identical today. Two failure paths. (1) Build-time wrap (the spike''s generateBundle plugin): source untouched so the sync test stays GREEN, but the SPA''s :root tokens are now inside @layer rela while the apps'' _rela.css tokens are unlayered — the two artifacts asserted identical now behave differently in the cascade. The test''s guarantee is voided while passing, the worst kind of failure. (2) Source-file wrap (the plan''s documented fallback, ''wrap the 8 src/styles/*.css''): the sync test fails immediately, and the obvious fix — re-copy the layered file — ships ''@layer rela { :root {...} }'' to every sandboxed app, where there is no rela CSS to layer against, so any app''s own :root beats the tokens regardless of specificity or order. That is a silent behavior change to a frozen app-facing contract (internal/dataentry/CLAUDE.md:191-197) shipped as a side effect of an unrelated SPA cascade fix.'
severity: critical
status: addressed
resolution: >-
  Dissolved by DECISION 1: tokens.css is excluded from the layer entirely, because it is served into two different cascade environments and must behave identically in both. Implementation adds an assertion that neither copy contains @layer.
---

## Failure scenario

Operator upgrades rela. Their custom app at `apps/dashboard/` uses `_rela.css`
and defines `:root { --accent-color: red }` *before* `<link href="_rela.css">`.
Previously the link won on source order; now the unlayered app rule wins. Every
app relying on token precedence shifts color. Nothing fails — it just looks
wrong, in a release whose only stated change was "operator CSS wins".

## Recommended resolution

Make this an explicit design decision rather than an emergent one: state whether
`tokens.css` is in or out of the layer.

Recommendation: **exclude `tokens.css` from the layer.** Custom properties are
values rather than cascade participants in the way the flat-layer plan assumes,
and it is the one file carrying a cross-artifact contract. Then strengthen
`TestAppTokensCSSInSyncWithFrontend` to assert the *served* forms match, or add
an explicit assertion that neither copy contains `@layer`.

Relates to [[rr-layer-sublayers]] — the strain on the flat layer shows up here
first.
