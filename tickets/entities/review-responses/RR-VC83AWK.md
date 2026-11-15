---
id: RR-VC83AWK
type: review-response
title: Wrong resumed log after the IdP recheck
finding: When a resume was followed by a recheck that suspended the tenant again, the host still logged that the tenant was resumed.
severity: minor
resolution: 'Fixed in 13cd61e18: the log reports the final outcome after the recheck. Pinned by the extended TestStatusSync_ResumeRechecksTheIdP.'
status: addressed
---
