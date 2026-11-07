---
id: RR-WZ2Y7XS
type: review-response
title: Injected engine role is visible to operator tools
finding: injectFlowEngine writes Roles/Assignments, so acl audit and docs show _flow-engine; it runs before later validation errors.
severity: minor
reason: The engine role is a real grant and reports should show it. A failed load returns an error and the policy is discarded.
status: wont-fix
---
