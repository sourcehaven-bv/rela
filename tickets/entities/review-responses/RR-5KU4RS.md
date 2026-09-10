---
id: RR-5KU4RS
type: review-response
title: 'The bug was not fixed: an empty-steps file still satisfied the gate'
finding: The branch added a step that CAN express the confirmation but nothing that REQUIRES it. A file spanning the bare_face_introduced edge with no confirm_face step still parsed, resolved, applied, advanced the marker and reported 'data schema in sync' — verbatim the defect BUG-TMGWIN describes. Worse, the generator emitted the confirm_face skeleton commented out, so the literal do-nothing file was still what `rela migrate gen` produced by default; applying an unedited draft reproduced the bug exactly.
severity: critical
resolution: 'Confirmed by applying an unedited draft against a scratch project: it succeeded and reported the schema in sync. ParseFile now calls validateDeltasResolved, which recomputes the deltas from the file''s own embedded projections and refuses one leaving a bare_face_introduced unanswered. resolvingSteps is the explicit delta-kind to step-kind table; metamodel.MigrationDeltaKinds plus a source-scanning guard test keep it complete. Because an unconfirmed file can no longer parse, migrate gen now emits a LIVE confirm_face step rather than a commented one. Verified end to end: the do-nothing file is refused with the delta''s own detail text.'
status: addressed
---

The finding was correct and my own report that the bug was fixed was wrong. I
had verified that the step worked, not that the defect was closed.

The distinction the review draws is the right one: "the primitive is available"
is not "the gate is satisfied semantically", and the bug's own why3 names the
latter as the defect. Enforcement was the missing half.

One knock-on worth recording. The commented-skeleton decision was defensible in
isolation (an unedited draft must not confirm a state nobody looked at) but
combined with no enforcement it meant the default path was unchanged. Adding the
enforcement resolves the tension in the other direction: the draft must now be a
live step, and the operator's job is to notice when its pre-filled answer is
wrong rather than to supply an answer at all.
