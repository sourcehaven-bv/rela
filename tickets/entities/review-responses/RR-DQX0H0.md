---
id: RR-DQX0H0
type: review-response
title: 'Step 4 PR-A: stale who-can guarantee, untested create branch, and two doc omissions'
finding: |-
    Three further findings from the PR-A code review, all verified:

    1. WHO-CAN UNDER-REPORTS, AND ITS GODOC CLAIMED IT COULD NOT. `grantingAttributions` (access.go:260) still calls the type-granular grantsVerb, which skips state grants. Post-change the runtime authorizes page@draft for a role holding `update: ["page@draft"]`, while `rela acl who-can update page` reports NOBODY. The godoc directly above asserted 'The write path IS this attribution set, so equivalence is by construction' — true when RR-Q1LI2Y chose to skip state grants, because back then they authorized nothing. This PR killed that premise. The direction is safe (a false negative in a security report, never a false all-clear) but an operator told 'nobody can update this' while a client updates it hourly stops trusting the tool, which is worse than a loud wrong answer.

    2. THE CENTERPIECE TEST COVERED ONE OF TWO BRANCHES. TestExistingGrantsUnchangedByFaceField passed ID:"PAGE-1" on every case including the create one, so it always took the `s.ID != ""` branch of authorizeEntityWrite. The `else` branch — globals-only, the path a real create with no id yet takes — was never exercised, by the test whose comment claims to pin the property for 'every existing call site'.

    3. Two smaller ones: `decideFromAttrs(attrs, op, s.FromType, "", ...)` passed an untyped "" that reads as an empty deny-format string at a glance (the adjacent argument IS a format string); and subject.go's sealed-sum diagram showed RelationSubject with no indication that its face-blindness is deliberate and temporary — that rationale lived only in a comment 55 lines away.
severity: significant
resolution: |-
    1. Corrected the guarantee rather than papering over it: AccessRoutes' godoc now states plainly that equivalence is no longer by construction, that the divergence is deliberate ('who can update page?' is a question about a TYPE and the report has no face in hand), and that the answer is a documented LOWER BOUND. Named the fix if operators need exactness — threading a pointer through AccessRoutes. Also corrected AssertedGrants' and grantingAttributions' comments, and principalGrantsCreate's 'mirrors the resolution grantsVerb performs at write time', which is no longer true.

    2. Added TestCreateWithoutIDStillAuthorized — four cases with `EntitySubject{Type: "page"}`, no ID and no Face, exercising the globals-only fork.

    3. `entity.Face("")` explicit at the authorization call site; the sealed-sum diagram now annotates both variants ('Face zero = default face' / 'default tail only — see authorizeRelationWrite').
status: addressed
---
