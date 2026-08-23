---
id: RR-NJSCP5
type: review-response
title: PR-C comment taxonomy needs a third category and a real file list
finding: 'PR-C''s deliverable says comment every remaining pointer='''' as either identity anchor or default world. There is a third category the binary taxonomy mislabels: family-scoped WRITE-path literals that must stay pointer-unaware — HighestID (pgstore/entity.go:173-174, states share their family''s number, worlding it would let two entities get the same sequential id), the derived unique indexes (derivedschema.go:240-246,306,322), and the family-type FOR SHARE check (entity.go:296). Plus versioning/sync infrastructure across manifest.go, sweep.go, purge.go, relation.go, relation_version.go. That is ~20 literals across nine files, not four.'
severity: minor
status: addressed
resolution: "Architect decision 2026-08-20: ACCEPTED. PR-C's comment taxonomy gets a THIRD category - family-scoped write-path literals that are deliberately pointer-unaware (HighestID family numbering, derived unique indexes, the family-type FOR SHARE check) - plus versioning/sync infrastructure. Deliverable is the real ~20-literal, nine-file list, not four. Each literal gets one of: identity anchor / default world / family-scoped (pointer-unaware by design)."
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

PR-C's comment-disambiguation deliverable uses a two-value taxonomy (`//
identity anchor` / `// default world`). A third category exists, and mislabeling
it invites a later "fix" that breaks id allocation:

**Family-scoped write path — deliberately pointer-unaware, must NOT be
worlded:**

- `pgstore/entity.go:173-174` (`HighestID`) — "states share their family's
number". Worlding it would let two entities receive the same sequential id.
- `pgstore/derivedschema.go:240-246, 306, 322` — `unique: true` is a natural-key
rule over the default state, deliberately pointer-unaware.
- `pgstore/entity.go:296` — the `SELECT type ... FOR SHARE` family-type check.

**Versioning/sync infrastructure (documented Step-1 skips):**
`pgstore/manifest.go:30-33`, `sweep.go:279-306, 424-441`, `purge.go:306, 323`,
`relation.go:25, 200, 230`, `relation_version.go:182-187, 462`.

Roughly 20 literals across nine files versus the four PR-C budgets. Most stay
as-is — but a reviewer applying a two-value taxonomy to `HighestID` picks
"default world", and someone worlds it later.

**Resolution:** make the taxonomy three-valued in the plan, adding `//
family-scoped write path — deliberately pointer-unaware, do NOT world`.
Enumerate the file list (`grep -rn "pointer = ''\|from_pointer = ''"
internal/store/pgstore/`) so PR-C's author is not discovering scope mid-PR.
Consider a package-scan guard for the third category.
