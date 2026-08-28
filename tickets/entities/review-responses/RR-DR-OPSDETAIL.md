---
id: RR-DR-OPSDETAIL
type: review-response
title: Directory/index behaviour, oversize diagnostics, and mux-normalisation need assertions rather than
  inference
finding: 'Three gaps. (1) The plan lists ''directory request -> 404'' as an edge case but no AC covers
  it, and probing shows TWO different rejection paths: ''fonts'' (no slash) is caught by the IsDir check,
  ''fonts/'' is caught earlier by fs.ValidPath (trailing slash invalid). Both 404 but for different reasons
  and only one is documented. (2) Go''s ServeMux REDIRECTS /_custom/sub//.env and /_custom/a/./.env (307)
  before the handler runs - worth pinning so a future mux change is caught. (3) There is deliberately
  no index-file resolution (/_custom/fonts/ must 404, not serve fonts/index.html) - ''serve arbitrary
  files from a directory'' is exactly the phrasing that invites someone to add it later. (4) The 4MB cap
  was sized for ''a stylesheet or a script''; an operator dropping a hero image hits it and gets a uniform
  404 with no diagnostic - ''my logo works, my background does not, both 404''.'
severity: minor
status: addressed
resolution: 'Folded into TKT-IWMETE and PLAN-6VVJJZ before implementation. Add ACs/tests for the directory
  cases and the mux-normalisation spellings; state the no-index-resolution rule in docs and pin it. For
  the cap: since the uniform-404 rationale explicitly says it is NOT a confidentiality measure, log a
  slog.Warn on the oversize branch naming the file and the cap - one line, saves a support round-trip.'
---

Raised by `/design-review` of TKT-IWMETE before implementation.
