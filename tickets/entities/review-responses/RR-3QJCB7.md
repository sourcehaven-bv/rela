---
id: RR-3QJCB7
type: review-response
title: 'Acknowledged nits: insertion sort, NBSP collapse, defense-in-depth invariant'
finding: 'Reviewer nits: (#2) GetPrimaryProperty uses a hand-rolled insertion sort where sort.Strings would do; (#4) collapseWhitespace via strings.Fields collapses ALL Unicode whitespace incl. NBSP in literal text; (#6) the ''render assumes validated input'' godoc slightly oversells — LoadWithoutMigrationCheck skips validate, so render''s graceful-degradation branch is genuinely reachable (and safe).'
severity: nit
reason: (#2) The insertion sort is pre-existing code I didn't touch; changing it is unrelated churn — out of scope for this ticket. (#4) Collapsing all Unicode whitespace is correct and desired per the spec (single-space display names); a deliberate NBSP in a display template is not a supported use case. (#6) The behaviour is correct (safe, no panic); the graceful-degradation branch is now directly tested (TestRenderDisplayTemplate/unclosed_brace_emitted_verbatim), and the godoc already notes it 'degrades gracefully' — no code change needed.
status: wont-fix
---
