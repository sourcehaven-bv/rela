---
id: RR-7B4FHE2
type: review-response
title: Stale golden comment on the BUG-2TPO2F case
finding: golden_parity_test.go still said the list-context command feeds the script raw rows and listed BUG-2TPO2F among bug cases that record a known gap. The recordings now show the gated and redacted payload.
severity: nit
resolution: 'Both comments now say the case pins the fix: rows are row-gated and field-redacted as GET /api/v1/tickets shows them. BUG-VUNO11 stays named as the open gap.'
status: addressed
---
