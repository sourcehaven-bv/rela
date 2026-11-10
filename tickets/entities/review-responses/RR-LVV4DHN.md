---
id: RR-LVV4DHN
type: review-response
title: 'Code review: control lookups have no timeout'
finding: A hung control-table connection could stall revalidation indefinitely.
severity: minor
resolution: pgControl.lookup bounds each query with a 5s timeout.
status: addressed
---
