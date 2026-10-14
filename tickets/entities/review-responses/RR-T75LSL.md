---
id: RR-T75LSL
type: review-response
title: JSON verbs array and can-relation enum
finding: Adding rename changes rela acl map JSON; the enum test must not include can-relation, whose verbs are create, update and delete.
severity: nit
resolution: Rename is not added in this ticket, so the JSON is unchanged. The enum test lists can, who-can and map only and says why can-relation is excluded.
status: addressed
---
