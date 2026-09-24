---
id: RR-ZL76L1
type: review-response
title: List containment semantics differ from naive
finding: json_each type='text' matches pg's ? operator but not naive's Stringify of every element.
severity: minor
resolution: sqlite matches pg (string elements only), as the ticket asks; the differential keeps numeric list elements out of containment and the difference is recorded in the builder doc.
status: addressed
---
