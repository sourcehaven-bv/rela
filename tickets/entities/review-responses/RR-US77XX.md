---
id: RR-US77XX
type: review-response
title: 'Minor/leverage notes: guard operand mismatch, JSON asymmetry, fixture narrowing, dead nil-path, Meta() cost'
finding: 'Reviewer minor/leverage notes: (#4) graph guard compares raw title vs e.ID while label is escaped/truncated (latent trap if IDs gain special chars); (#5) list/show JSON stays raw while table shows resolved title (asymmetry); (#6) DisplayTitle only surfaces a title for a declared+required display property, a behavior narrowing vs old blind e.Title(); (#7) Meta() per-relation-loop is cheap - no concern; (#9) the nil-resolver fallback is effectively test-only since all entity-rendering commands require a project.'
severity: minor
resolution: '(#4) Left as-is: IDs are short/quote-free/newline-free so raw-vs-escaped can''t diverge today; documented the invariant intent in the guard comment. (#5) Deliberate: JSON is raw data, table is presentation - consistent with the export decision (RR-X3YE8K); no change. (#6) Documented in the ticket''s deliberate-exclusions; no in-tree metamodel declares a title-carrying type without declaring title required, so safe. (#7) No change - Meta() returns a cached field. (#9) Kept the nil fallback as a defensive default (also serves tests); comment no longer overclaims a live non-project path.'
status: addressed
---
