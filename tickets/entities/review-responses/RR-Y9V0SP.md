---
id: RR-Y9V0SP
type: review-response
title: 'Minor polish: vacuous confirmations, misleading Target arrow, projection aliasing, unused Apply'
finding: Four small items. (a) A confirmation covering zero rows reported 'confirmed 0 row(s)' and looked identical to a successful one, when it is vacuous and worth saying so. (b) Target() returned 'task.status → face'; the arrow suggests a transformation when the step's central claim is that nothing moves. (c) enumValuesIn returned the projection's internal slice without copying, handing callers a live handle into a shared ShapeProjection. (d) Run ignores x.Apply, making it the one step where the flag is meaningless, with no comment saying so deliberately.
severity: minor
resolution: (a) Zero rows now reports 'no rows at the bare coordinate — nothing to confirm', pinned by TestConfirmFace_EmptyStoreSaysNothingToConfirm. (b) Target is now 'task.status (confirm bare face)', with a comment explaining why the arrow is avoided. (c) enumValuesIn returns slices.Clone, with a comment citing the shared-projection reason. (d) Documented on Run that writing nothing makes Apply moot, so a reader auditing Apply handling sees it is deliberate.
status: addressed
---

All four taken. (c) is the one I would have skipped as theoretical, but the
project's `SharedBase` rule makes projection aliasing a live concern rather than
a hypothetical one, and a `slices.Clone` at this call volume costs nothing.

The reviewer also noted that duplicate `confirm_face` steps for the same entity
parse cleanly and produce two identical notes. Left as-is: it is harmless, and
the natural place to catch it would be `validateStepOrder`, which I have just
finished documenting as deliberately narrow.
