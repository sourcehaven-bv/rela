---
id: RR-NS8UFHX
type: review-response
title: Guard-first order not pinned when the precondition errors
finding: The drift table checked agreement and Allowed but not which gate refused, and no test showed the precondition is not evaluated for an unauthorized caller.
severity: minor
resolution: 'Addressed: the drift table asserts VerdictGuard for an unheld or nil guard in every when: and bind-error case; TestEnforceUpdate_PreconditionSkippedWhenGuardUnheld counts graph lookups and asserts none for an unauthorized caller.'
status: addressed
---
