---
id: RR-S8X8GA
type: review-response
title: sort=modified exemption leaks through the pushdown when a type declares modified
finding: A schema may declare a string property named modified. The Go sort orders that key by UpdatedAt, but the pushdown's stringShaped check is true and the store orders by the property value; the gate skipped the key, so a hidden value leaks through row order, and the two paths disagree. Make the pushdown decline virtual sort keys.
severity: significant
resolution: planListPushdown declines a virtual sort key (id or modified). TestListPushdown_Eligibility covers a ticket type that declares a string property named modified.
status: addressed
---
