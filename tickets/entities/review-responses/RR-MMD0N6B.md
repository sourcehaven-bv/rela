---
id: RR-MMD0N6B
type: review-response
title: Startable table misses cases
finding: No case for a flow on another type or a flow without a manual start or a project without acl.yaml or a draft address.
severity: minor
reason: The test project has one flow; the type and manual filters are the same loop Start uses through manualPermissions and are covered by the Start tests. Adding flows to the shared test schema would change many other tests.
status: wont-fix
---
