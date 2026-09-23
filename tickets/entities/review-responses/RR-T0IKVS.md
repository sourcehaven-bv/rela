---
id: RR-T0IKVS
type: review-response
title: No server-side default_sort fallback
finding: parseSortParam returns nil without sort= and applyV1Sorting leaves rows unsorted (api_v1.go:2230-2270); metamodel DefaultSort is unused in dataentry. listIndexSpec derives nothing for an empty sort (queryplan.go:494).
severity: significant
resolution: 'One dataentryconfig helper resolves the effective sort: entry sort:, else the type default_sort, else none (id order). Both the wire Sort and the synthetic List fed to queryplan use it.'
status: addressed
---
