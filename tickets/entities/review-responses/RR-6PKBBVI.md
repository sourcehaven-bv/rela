---
id: RR-6PKBBVI
type: review-response
title: Single-implementation StatusSyncer interface
finding: tenanthost.StatusSyncer has one implementation, which a reviewer flagged as an unnecessary abstraction.
severity: nit
reason: 'Deliberate test seam: the consumer-side interface lets the host tests drive suspension sync without a database or IdP. It follows the project rule to define interfaces at the call site.'
status: wont-fix
---
