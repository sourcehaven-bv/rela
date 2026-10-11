---
id: RR-HS49Z2
type: review-response
title: One bind per type per schema call
finding: Each count binds the principal again; so the schema tool binds T+R times
severity: minor
resolution: Deferred to TKT-OJ1UYJ together with the relation pushdown.
reason: Binding is a member-of walk per call. For atlas that is about 60 walks per schema call. That is acceptable for an overview tool and is cheapest to fix at the same seam as the relation count pushdown.
status: deferred
---
