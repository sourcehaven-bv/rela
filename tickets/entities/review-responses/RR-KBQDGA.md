---
id: RR-KBQDGA
type: review-response
title: pg test in the skip-tolerant wiring CI step
finding: The test ran in a step that tolerates skips.
severity: nit
resolution: 'Moved to its own Postgres-job step that fails on any SKIP. Helpers stay local: pgmigstate''s are in another test package and backendtest builds a whole appbuild project.'
status: addressed
---
