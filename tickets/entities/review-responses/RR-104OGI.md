---
id: RR-104OGI
type: review-response
title: Timestamp migration test covered only two tables
finding: attachments, entity_versions, relation_versions and schema_versions rewrites were not asserted, nor paging.
severity: minor
resolution: Test seeds a row in all six tables and 6000 bulk rows (more than one page) and asserts all are rewritten.
status: addressed
---
