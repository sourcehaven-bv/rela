---
id: RR-3ZPMR8H
type: review-response
title: Missing argument exits 1 (read as found)
finding: ${1:?} exits 1 which the workflow reads as advisories found; import without path made startswith fail
severity: nit
resolution: Explicit exit 2; pathless import counts as unknown; tests added
status: addressed
---
