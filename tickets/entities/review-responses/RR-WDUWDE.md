---
id: RR-WDUWDE
type: review-response
title: CalDAV DELETE no-op is an existence oracle
finding: The DELETE no-op returned 204 for a hidden or non-member to-do and 404 for a nonexistent one. A caller could learn that a hidden entity exists.
severity: significant
resolution: Fixed in 2869cf91e. The DELETE is a no-op only when the entity is readable and has another readable membership. Otherwise it returns 404. Covered by TestDynamicCollections_MissingMembershipDeleteIsNotAnOracle.
status: addressed
---

Review finding R2-2.
