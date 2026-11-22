---
id: RR-OHC0B95
type: review-response
title: "The probe could double traffic during an outage"
finding: "Every network error started a probe request."
severity: significant
resolution: "Addressed: one probe at a time and at most one per 30 seconds. Pinned by workspaceAccess.test.ts."
status: addressed
---
