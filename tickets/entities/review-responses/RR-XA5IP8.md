---
id: RR-XA5IP8
type: review-response
title: SidePanel, preview modal and DocumentView have no unit test for the world
finding: Their tests pass without the change, so nothing checks that their edit routes carry ?world=.
severity: minor
reason: All three build the route with editFormRoute, which has its own unit tests, and the e2e covers the main entry point.
status: deferred
---
