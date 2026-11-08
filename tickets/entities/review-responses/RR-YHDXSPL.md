---
id: RR-YHDXSPL
type: review-response
title: Inherited and face-split subject reads never reach runs
finding: gateHop refuses a read query that uses inherit_roles_through or Any branches. Readers who reach a subject only that way would never see its runs, so the plan's claim that runs follow the subject exactly does not hold.
severity: critical
resolution: 'Plan changed: a subject type whose read gateHop cannot fold drops its branch, which fails closed (the answer is an allowlist). AC4 now says a reader never sees more runs than subjects, and sees exactly the runs of subjects read through a global grant or a plain role relation. Nested inheritance in endpoints is a backend change with its own conformance work; filed as a follow-up ticket.'
status: addressed
---
