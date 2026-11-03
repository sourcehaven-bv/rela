---
id: RR-PKLTOE
type: review-response
title: Virtual sort keys hard-coded in a second place
finding: Derive the id/modified exemption from filter's virtual sort key list instead of repeating the names in the gate.
severity: nit
resolution: Exported filter.IsVirtualSortProperty; the gate and the pushdown both use it.
status: addressed
---
