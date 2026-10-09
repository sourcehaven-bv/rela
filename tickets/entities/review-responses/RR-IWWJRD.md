---
id: RR-IWWJRD
type: review-response
title: Storage key not scoped to the project
finding: Two projects on the same origin share rela:kanban-columns:<id>.
severity: minor
reason: The list-groups key has the same property and the reviewer recommends fixing both or neither. Scoping needs a project identity on the client that the SPA does not expose today, so it belongs in a separate ticket covering every localStorage key, not in this one.
status: deferred
---
