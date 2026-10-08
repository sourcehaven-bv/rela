---
id: RR-4Q56DS
type: review-response
title: 'Test gaps: faces, ordering, collision'
finding: Only an incoming faceless edge was covered; memstore only.
severity: minor
resolution: 'Added faced, ordering and collision tests. Postgres is not exercised: the change adds no backend-specific code and reads through the Tx handle like the existing cascade check.'
status: addressed
---
