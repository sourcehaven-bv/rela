---
id: RR-RO5MWTU
type: review-response
title: validate_flows skips CheckTriggers when acl.yaml fails
finding: validate_flows.go returns before CheckTriggers when acl.yaml does not validate although triggers do not depend on the ACL.
severity: nit
resolution: reportFlowsValidation checks triggers before reading acl.yaml, so an invalid acl.yaml no longer skips them.
status: addressed
---
