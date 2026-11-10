---
id: RR-SKOL4WA
type: review-response
title: Older flow versions compile once per run
finding: Explain and the task filter would read and compile an older definition for every run.
severity: significant
resolution: Compiled flows are cached per (flow, version) for one operation; the task budget test includes a run on an older version.
status: addressed
---
