---
id: RR-0PLJHN
type: review-response
title: _openapi.json exemption matches sub-paths
finding: isNonBrowserExemptV1Path matched /api/v1/_openapi.json/anything.
severity: nit
resolution: Exact match on _openapi.json; a test case pins the sub-path as not exempt.
status: addressed
---
