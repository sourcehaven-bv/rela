---
id: RR-LLLBQY
type: review-response
title: warnUndeclaredFaces boot warning becomes false the moment PR-A lands
finding: 'appbuild.warnUndeclaredFaces (appbuild.go:899-917) computes CountEntities(AllStates) - CountEntities({}) and warns that the store ''holds content-state rows that no metamodel declaration accounts for''. Its doc comment states the Step-1 premise explicitly. PR-A falsifies that premise: the instant a project declares faces: {draft:{}} and writes a draft, every boot logs a stranded-data warning about perfectly declared rows. The file is not in the plan''s PR-A file list.'
severity: significant
resolution: 'Fixed in PR-A (72bf2f21) with option (a). warnUndeclaredFaces now takes the metamodel and returns early when declaresFaces(meta) is true, so the coarse two-COUNT probe runs only for projects where no type declares faces — preserving Step-1 behavior for them and going silent for adopters, where `analyze states` (per-type subtraction, RR-E1C216) is authoritative. The doc comment was rewritten: it no longer asserts the retired Step-1 premise and now explains the scope and the warning-fatigue rationale. Pinned by TestDeclaresFaces (table-driven, including nil metamodel and an empty-but-present faces map).'
status: addressed
---

**Finding (design review, TKT-WAV8XP PR-A planning).**

`internal/appbuild/appbuild.go:899-917` logs a startup warning — *"store holds
content-state rows that no metamodel declaration accounts for"* — computed as
`CountEntities(AllStates) - CountEntities({})`. Its doc comment records the
Step-1 premise: *"in Step 1 no metamodel declaration can account for a pointer,
so any state row is data a schema change (or hand edit) stranded."*

PR-A retires that premise. Declare `faces: { draft: {} }`, write one draft,
and every boot warns about stranded data that is in fact fully declared. The
plan does not mention this file.

Not cosmetic: warning fatigue on a boot diagnostic is how the genuinely stranded
case stops being noticed — and the remedy it points at (`rela analyze states`)
is the same detector RR-E1C216 is about. Shipping PR-A as written turns a
working detector into noise for precisely the projects that adopt the feature.

**Resolution:** add `internal/appbuild/appbuild.go` to PR-A's file list and pick
one, in the plan rather than at the keyboard:

- (a) gate the warning on "no type declares faces" — cheap, mechanical,
preserves Step-1 behavior for faceless projects and goes silent for adopters;
or
- (b) replace the two-COUNT probe with a call into `analysis.CheckStates`
under the same advisory, never-fails-the-boot error policy.

(a) matches the "detection only" framing and is the recommendation. Either way
the doc comment must be updated — it currently asserts an invariant PR-A ends.
