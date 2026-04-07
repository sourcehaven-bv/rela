---
id: RR-35MT
type: review-response
title: Use cmp.Compare instead of bespoke generic
finding: Go 1.21+ has cmp.Compare[T cmp.Ordered] in stdlib. compareOrdered is reinventing it. Less code, idiomatic, no custom generic to maintain.
severity: nit
resolution: Replaced bespoke compareOrdered with cmp.Compare from stdlib (Go 1.21+). Generic constraint changed to cmp.Ordered. Wrapper interprets the int result against the operator. Less code, idiomatic.
status: addressed
---
