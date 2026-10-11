---
id: RR-3DOK3T
type: review-response
title: SET search_path FROM CURRENT pins the tenant schema name
finding: Trigger functions would keep the migration-time schema after a rename or a restore under another name.
severity: minor
resolution: Functions use EXECUTE format('%I.entities', TG_TABLE_SCHEMA); TestVersionTriggersClearContentHash runs on a schema-pinned pool.
status: addressed
---
