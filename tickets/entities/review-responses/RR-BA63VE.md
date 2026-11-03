---
id: RR-BA63VE
type: review-response
title: schema_json detail tests assert only name
finding: After removing the getters, the entity and relation detail tests checked only name; inverse, owning and id_patterns were unpinned.
severity: minor
resolution: Both tests now compare the whole decoded document; the relation test is table-driven and sets owning and an inverse label.
status: addressed
---
