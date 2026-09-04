---
id: RR-EHER1V
type: review-response
title: Count paths bypass the world, leaking the unpublished-draft tally
finding: 'gatedGraphReader.CountEntities (appbuild.go:528-530) deliberately goes to the raw store, justified by ''a count is STRUCTURAL... entity existence is the secret the row gate protects; an aggregate tally of a type the metamodel already publishes is not''. That reasoning is correct for ACL and wrong for worlds: design doc §4.1 makes existence in a world the publication bit, so an unscoped tally tells a published-world surface how many unpublished drafts exist. The comment at appbuild.go:501 even says ''If either judgement changes, this is the one type to fix'' — this judgement is changing.'
severity: significant
status: addressed
resolution: "Architect decision 2026-08-20: ACCEPTED - the finding correctly identifies that a judgement recorded as revisable is now changing. Counts must be world-scoped: design doc S4.1 makes existence in a world the publication bit, so an unscoped tally tells a published-world surface how many unpublished drafts exist. The ACL-era reasoning (an aggregate over a type the metamodel publishes is not a secret) does not transfer, because the world's population IS the secret. Fix gatedGraphReader.CountEntities to route through the world scope; update the comment at appbuild.go:501 to record that the judgement changed and why."
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

`internal/appbuild/appbuild.go:528-535` routes `CountEntities` /
`CountRelations` to the RAW store, with a documented justification at `:495`:
*"a count is STRUCTURAL: it says how many rows of a declared type exist, never
which. Entity existence is the secret the row gate protects; an aggregate tally
of a type the metamodel already publishes is not."*

Sound for ACL. Wrong for worlds, and the difference is the whole feature. Design
doc §4.1: *"Existence in a world IS the publication bit — 'unpublished'
literally means 'nonexistent in world:published'."* An unscoped tally therefore
leaks the count of unpublished drafts to a published-world surface. "There are
47 pages" on a site publishing 12 is a disclosure the ACL argument explicitly
does not cover: in the ACL model those 35 rows exist and are merely gated; in
the world model they do not exist.

Same shape at `internal/dataentry/views_handler.go:330` (sidebar counts — that
one goes through `GraphQuery` so it inherits `World` for free once PR-B adds it)
and at `appbuild.go:900-905` (RR-LLLBQY's probe).

Note the existing comment ends *"If either judgement changes, this is the one
type to fix."* This is that change.

**Resolution:** add `CountEntities` to PR-D's wiring scope with the decision
recorded explicitly. The right answer is the cheap one: the world rides on
`EntityQuery`, and `gatedGraphReader.CountEntities` passes it through to the raw
store — the raw store still applies the WORLD, it just skips the ACL gate, which
is what that comment actually means. Two lines, but it must be said, because
silence here reads as "counts are fine as they are."
