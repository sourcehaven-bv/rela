---
id: RR-7FOWDB
type: review-response
title: Search is a fifth world-scope site and a second AllowAll bypass
finding: Search is a fifth independent world-scope site (pgstore/visiblesearch.go:239-241 has its own SQL builder hardcoding e.face='') and reproduces the F5 AllowAll bypass in a second package (search/visible.go:258-262). Once PR-D wires a world(published) surface, its search box returns default-world hits — i.e. drafts — and the ACL row gate cannot catch it because guard rule 1 makes the row gate world-independent.
severity: critical
status: addressed
resolution: "Architect decision 2026-08-20: ACCEPTED as stated. Search is in scope for Step 2 and must REFUSE a non-default world (search.ErrScope) in both search.Visible and pgstore.SearchVisible, rather than silently serving the default world. Rationale: guard rule 1 makes the ACL row gate world-independent, so it structurally cannot catch this; and under the doc's own draft-is-default example a world-bound public surface's search box would return drafts. Refusal makes it impossible for PR-D to wire a world-bound surface with a working-but-wrong search box. Per-world INDEXING remains Step 5; this is the refusal seam only. Added to PR-C scope plus the comment-disambiguation list."
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

The plan counted four pgstore `face = ''` sites (F1) and one ACL `AllowAll`
bypass (F5). There is a fifth site and a second bypass, both in search, and the
plan's OUT-OF-SCOPE line ("per-world search indexing is Step 5; the Step-1 skip
stays as it is") silently smuggles in the stronger claim that search needs no
world work in Step 2.

Verified in tree:

- `internal/store/pgstore/visiblesearch.go:239-241` — `buildVisibleSearchSQL` is
an INDEPENDENT SQL builder, not `buildGraphQuerySQL`, and hardcodes `WHERE
e.face = ''` with a TKT-DOFYR1 comment that reads as already-handled.
- `internal/search/visible.go:269` — the fs/mem path reaches
`v.gq.MatchingIDs(ctx, *ts.Query, ids)`, i.e. a `GraphQuery`. Once PR-B adds
`GraphQuery.World`, search passes a ZERO World and silently gets the default
world while the entity-list path beside it gets the requested one.
- `internal/search/visible.go:258-262` — `allowedIDs` short-circuits on
`ts.AllowAll` and skips `MatchingIDs` entirely. That is the F5 shape reproduced
in a second package.

Why critical rather than Step 5's problem: once PR-D wires a
`visibility(everyone) ∘ world(published)` public surface, that surface's SEARCH
BOX is a working bypass. It returns hits computed against the default world —
and in the design doc's own §4.1 example layout (`draft: {default: true}`) the
default row IS the draft. The ACL row gate does not save this: guard rule 1
makes the row gate world-independent, so a draft the reader may legitimately
read in the default world passes straight through.

**Resolution:** thread the world into the search seam and make BOTH
`search.Visible` and `pgstore.SearchVisible` REFUSE a non-default world
(`search.ErrScope`) rather than silently serving the default one — the same
"reject loudly, never a naive path" stance already adopted for pgstore in Q8.
That makes it structurally impossible for PR-D to wire a world-bound surface
with a working-but-wrong search box. Add `visiblesearch.go:241` and
`search.go:62-65` to PR-C's comment-disambiguation deliverable.
