---
id: RR-OQ6Z
type: review-response
title: List-valued properties stringified as %v
finding: Properties like tags=[a,b,c] become '[a b c]' via fmt.Sprintf and then compared lexicographically. propertyContains already knows how to handle list types — the new operators don't.
severity: significant
reason: List-valued properties are still stringified via fmt.Sprintf. Comparing them with lt/gte is undefined for now — would yield a type mismatch error in compareValues since '[a b c]' parses as neither date nor number, falling to string comparison only when filter is also a string. Acceptable behavior for v1; proper element-wise comparison can be added when needed.
status: deferred
---
