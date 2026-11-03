---
id: RR-XPFI09
type: review-response
title: The busy-menu unit test did not set current
finding: The first opening test in SpaceCreateMenu.test.ts left current unset, so it did not show that opening wins over a set current, and it checked only the trigger.
severity: nit
resolution: The test sets current and opening, and checks aria-busy, aria-disabled and that no rows are offered.
status: addressed
---
