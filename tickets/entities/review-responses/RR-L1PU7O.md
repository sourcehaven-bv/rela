---
id: RR-L1PU7O
type: review-response
title: CalDAV move refused on bounded membership and retried forever
finding: A CalDAV client moves a to-do with a PUT into the new collection and then a DELETE from the old one. On a bounded membership relation the PUT was refused and the client retried forever.
severity: significant
resolution: Fixed in 21e383051 and refined in round 2 (2869cf91e). A PUT into another collection replaces the bounded membership. The follow-up DELETE is a no-op. Covered by TestDynamicCollections_MoveOnSingleMembership.
status: addressed
---

Review finding R1-4.
