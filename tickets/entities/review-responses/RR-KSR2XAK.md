---
id: RR-KSR2XAK
type: review-response
title: 'Design: constructor must reject nil collaborators'
finding: A nil admission channel would read as unbounded.
severity: minor
resolution: newReconcileHandler rejects a nil reader or schema and non-positive caps.
status: addressed
---
