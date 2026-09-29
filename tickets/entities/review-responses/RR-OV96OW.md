---
id: RR-OV96OW
type: review-response
title: Any identifier named bareEntityID is counted
finding: Only the function declaration is excluded, so a local variable of that name would count.
severity: minor
reason: Harmless over-count in the safe direction; none exists and the doc says the match is by identifier name.
status: wont-fix
---
