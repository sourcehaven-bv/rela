---
id: RR-AL8RQG
type: review-response
title: project_files DDL was duplicated between schemaSQL and the migration
finding: The CREATE TABLE for project_files existed verbatim in both schemaSQL (fresh databases) and the v1-to-v2 migration step (existing ones), with a prose comment saying every change to one needs a matching change to the other. Duplicated DDL drifts, and when it does a fresh database and a migrated one end up with different shapes — precisely the failure the version-stamping apparatus exists to prevent, arriving by the one route it cannot detect.
severity: significant
resolution: Hoisted to a single `const projectFilesDDL`, interpolated into schemaSQL and referenced by the migration step via sqlSteps. The prose instruction is gone because the constraint is now structural.
status: addressed
---
