---
id: RR-IK0AU
type: review-response
title: appendInlines default arm drops unknown inline type wrappers
finding: Unknown inline types recurse for inner text but emit no marker. Asymmetric with nodeToLua which captures unknown blocks as raw nodes preserving source.
severity: minor
reason: All goldmark inline kinds enabled in our parser are handled. Custom extensions are out of scope for this refactor. Documented in package comment as a follow-up.
status: deferred
---
