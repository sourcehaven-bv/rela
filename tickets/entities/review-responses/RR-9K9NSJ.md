---
id: RR-9K9NSJ
type: review-response
title: Suggested structural guard asserting every FieldVerdicts caller binds first
finding: Proposed a guard test (in the style of the policy.Roles[ grep guard) asserting that every FieldVerdicts caller is preceded by a Bind/WithRequest, to close the CLASS of the critical fail-open rather than the single instance.
severity: nit
reason: |-
    The critical fix removed the precondition this guard would police. applyClientCeiling now opens its own Request when ctx lacks one, so a caller that does not bind gets the ceiling anyway — there is no longer a 'must bind first' rule to enforce. A guard asserting it would encode a requirement that no longer exists.

    It would also be brittle: the 6+ FieldVerdicts call sites in internal/dataentry are not co-located with their binding (binding happens in middleware, calls happen in handlers), so a lexical proximity check would produce false positives and get suppressed. Removing the dependency is strictly better than policing it — the same reasoning TKT-80EWGM applied when it deleted the raw write-prep handle instead of documenting against it.
status: wont-fix
---
