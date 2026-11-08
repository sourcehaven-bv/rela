---
id: RR-3QG82IE
type: review-response
title: TOCTOU between edited-locally check and delete
finding: A local edit saved between the check and DeleteEntity is lost with the entity.
severity: minor
reason: entitymanager.DeleteEntity has no expected-version form; adding a compare-and-delete is a manager change for TKT-6GRZHXD. The one-write window is documented in the threat model.
status: deferred
---
