---
id: RR-XOONQT8
type: review-response
title: No test through the sqlite transaction lock
finding: Update and Delete were not tested through appbuild wiring and txRecordLock on sqlite.
severity: significant
resolution: Added a sqlite appbuild test that appends; updates; refuses a stale revision and deletes through the wired Service.
status: addressed
---
