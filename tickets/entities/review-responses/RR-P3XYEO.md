---
id: RR-P3XYEO
type: review-response
title: delete_entity/rename_entity report counts over hidden relations
finding: delete_entity uses ungated CountRelations and rename_entity reports RelationsUpdated over hidden edges, revealing how many hidden neighbours exist.
severity: minor
reason: Pre-existing and outside this bug's scope (Lua tools and search). Needs its own ticket.
status: deferred
---
