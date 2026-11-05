---
id: RR-NCBMCTK
type: review-response
title: DetectInFile scans twice
finding: DetectInFile ran HasConflictMarkers and then FindMarkers.
severity: nit
resolution: DetectInFile calls FindMarkers once.
status: addressed
---
