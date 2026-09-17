---
id: RR-9Q1TZ0
type: review-response
title: isAccessDenied was named for the inferred cause, not the decision
finding: The name reads as a general-purpose auth predicate, but the semantics are 'should a view drop content it is holding' — which is why 404 is in the set, and a 404 is emphatically not a denial in general. A future author adding a login redirect on isAccessDenied(err) would redirect on every deleted entity.
severity: significant
resolution: Renamed to shouldDropHeldContent across all call sites and tests, so the function is named for the decision rather than the cause. The godoc's opening sentence was rewritten to match, and it now carries an explicit 'Do NOT use this to drive a login redirect' line stating that a deleted entity is a true result here. The follow-up ticket for the global 401 handler carries the same constraint, so the next author meets it before writing the code rather than after.
status: addressed
---

Worth doing while the function had exactly two call sites. The rename is the
kind of thing that becomes impractical once a plausible-sounding name has
attracted a handful of subtly wrong uses, which is precisely the failure the
reviewer projected.
