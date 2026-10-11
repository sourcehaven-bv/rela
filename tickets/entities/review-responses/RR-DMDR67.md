---
id: RR-DMDR67
type: review-response
title: pg lifecycle test leaks its sweep on failure
finding: TestVersionSweep_StopEndsCapture registers no t.Cleanup(sw.Stop).
severity: minor
resolution: t.Cleanup(sw.Stop) added after NewVersionSweep.
status: addressed
---
