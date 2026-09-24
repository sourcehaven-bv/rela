---
id: RR-RXJH05
type: review-response
title: Related branch in visiblesearch is unreachable
finding: The ACL never fills Related, so the new rendering is dead code today.
severity: nit
resolution: Kept deliberately (skipping a gate field would widen results) and commented as such.
status: addressed
---
