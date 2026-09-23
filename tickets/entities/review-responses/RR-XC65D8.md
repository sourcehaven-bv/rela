---
id: RR-XC65D8
type: review-response
title: Links stale after ACL change
finding: A role-conferring relation write rebuilds the read gate but emits no entity:changed (watcher.go runSSELoop).
severity: minor
resolution: 'Documented: after a grant change the links update on the next write to that type or on reload, the same as an open list.'
status: addressed
---
