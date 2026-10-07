---
id: RR-2Z7RT8
type: review-response
title: Re-point to a disallowed target type became a 500
finding: Before this change a create with a type the relation does not allow was written with a warning. The new replace path turned this into a 500 error.
severity: significant
resolution: Fixed in 21e383051. The soft fallback is restored with a warning. Such creates are written in the store transaction with a capacity check, and removes go through DeleteRelation. The non-atomic case is documented. Covered by TestPatchRelations_RepointToDisallowedTypeWarns.
status: addressed
---

Review finding R1-6.
