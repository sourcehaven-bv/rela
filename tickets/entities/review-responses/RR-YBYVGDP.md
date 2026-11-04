---
id: RR-YBYVGDP
type: review-response
title: notify ignores to.type
finding: ResolveTraversal returns the target type per hop but NotifyHop dropped it, so Recipients read every entity reached and a second type with the same property name was notified.
severity: significant
resolution: NotifyHop carries Target; Recipients filters the header read by the last hop's type. TestNotify_ProjectOwnerLearnsOfCompletedTask adds a team on the same relation whose owner must not be notified (mutation-checked).
status: addressed
---
