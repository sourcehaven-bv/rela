---
id: RR-YP42WK
type: review-response
title: bare_face_changed onto an occupied face orphans rows and confirm_face waves it through
finding: 'draftActiveStep routed bare_face_changed to the same drafting case, and Validate accepted it, but the step''s reasoning is entirely about the flat-to-faced case where no named rows exist. Repointing between two EXISTING faces can strand data: the bare row starts impersonating the newly-bare face while the pre-existing named row for it becomes unreachable, and the old bare face resolves to an empty coordinate. renameFaceStep.Run detects exactly this collision and refuses it; confirm_face never looked for it.'
severity: critical
resolution: 'Scoped the enforcement to bare_face_introduced only. bare_face_changed is listed in resolvingSteps with an empty value and a written reason, so the exemption is visible rather than implied. Requiring confirm_face there would have been wrong twice over: the step does not check for an occupied destination, and it would break the legitimate rename_face-based migrations that handle the repoint today. Filed TKT-L3P8I6 with the three candidate fixes, the cheapest being to stop sharing the drafting case. This surfaced concretely during implementation: two existing rename_face tests failed the moment the enforcement was added.'
status: addressed
---

Correct, and the review's framing is what made the fix obvious. The two deltas
were sharing a drafting case only because both mention `bare_face`, which is not
a good enough reason.

Worth noting the enforcement surfaced this by itself: adding
`validateDeltasResolved` immediately broke
`TestRenameFace_OntoAnOccupiedBareRowIsRefused` and
`TestRenameFace_OntoTheBareFaceIsAlwaysACollision`, which are precisely the
tests covering the case `confirm_face` should not claim. The existing suite
already knew these were different problems.

Not fixed here because the honest fix is a design decision (which of the three
options in TKT-L3P8I6), and the case needs a type with faces AND a repoint AND
pre-existing named rows. Scoping out with a visible exemption is the right size
of response for this branch.
