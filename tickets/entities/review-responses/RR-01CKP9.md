---
id: RR-01CKP9
type: review-response
title: 'Injection caching vs per-request existence check: plan proposes two designs and picks neither'
finding: 'The plan says ''Compute the injected HTML once at startup, not per request... Per request we
  need a cheap existence check... document that adding custom.css needs a server restart, OR re-stat per
  request if cheap.'' That unresolved ''or'' sits in the approach section of a plan about to have its
  Design Review box ticked, and the two options have materially different semantics. The plan''s own edge-case
  list names the consequences of both (''File created/deleted while the server runs (restart-required
  vs re-stat)'' and ''Concurrent requests during injection-cache population'') without choosing. The plan
  also conflates two things: the HTML variant to serve is FOUR precomputable strings (none / css-only
  / js-only / both), not one cached string plus a boolean. Concrete defect if precompute-without-restat
  is chosen: custom.css added after startup is SERVED at /_custom/custom.css (the handler reads the FS
  live) but never INJECTED — a confusing half-working state where the file is fetchable but never applied,
  which reads as a rela bug.'
severity: significant
status: addressed
resolution: 'Implemented as recommended: four shell variants precomputed once in buildShellVariants (none/css/js/both),
  selected per request by a cheap stat. Kills the cache-population race and removes the restart caveat.
  The residual TOCTOU with the subsequent fetch is documented in the godoc (see RR-CR-TOCTOU).'
---

## Recommended resolution

Precompute all **four** HTML variants at startup (immutable, no lock, no
cache-population race — this kills the "concurrent requests during cache
population" edge case outright), and re-stat per request to select among them.

Two `stat` syscalls per document load is negligible next to the SPA bundle
fetch, and it makes the feature work without a server restart. AC2's
byte-identical assertion then becomes trivially checkable, and the restart
caveat disappears from the docs entirely.
