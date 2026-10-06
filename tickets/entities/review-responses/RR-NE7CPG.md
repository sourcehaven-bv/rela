---
id: RR-NE7CPG
type: review-response
title: facedApi.waitForVersions duplicated the base helper
finding: It re-implemented api.waitForEntityVersions and retried on every error status for 20s, hiding 403/404/500.
severity: significant
resolution: Removed; the history spec uses api.waitForEntityVersions.
status: addressed
---

It re-implemented api.waitForEntityVersions and retried on every error status
for 20s, hiding 403/404/500.
