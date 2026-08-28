---
id: RR-8ZOICR
type: review-response
title: asserted_role_assignments is a privileged-role path neither predicate nor linter models
finding: acl.Policy.AssertedRoles (IdP-claim role grants, resolver.go) can be the only privileged grant in a deployment; MembershipSelfPromotionOpen scans only Assignments, so such a policy gets a clean boot warning and a clean audit. Not a defect in this extraction — asserted roles are not reachable by writing a membership edge, so they are correctly outside THIS predicate, and the pre-existing linter has no check for them either — but the docs framing ('a choice rather than an oversight') slightly oversells coverage.
severity: minor
reason: 'Out of scope for TKT-T31NKT (behaviour-identical extraction; the gap predates it and is not membership-edge escalation). Recorded for the architect: candidate future A-tier aclaudit check (e.g. ''asserted assignment confers privileged role'') — needs its own threat-model decision on whether IdP-claim trust is in the audit''s scope.'
status: deferred
---
