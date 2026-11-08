---
id: RR-M7HPZ9G
type: review-response
title: Transport in the same package as the algorithm
finding: Put the HTTP transport in a subpackage so the Differ has no net/http dependency.
severity: nit
resolution: The HTTP transport lives in internal/reconclient/httpremote with its own arch-lint component.
status: addressed
---
