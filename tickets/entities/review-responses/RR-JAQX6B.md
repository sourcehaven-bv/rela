---
id: RR-JAQX6B
type: review-response
title: Packed miss condition and header-typed retiredLocation
finding: 'Nits: the entity GET packed the parse, the redirect and the 404 into one negated condition, and retiredLocation took a whole EntityHeader when it needs only the id.'
severity: nit
resolution: The miss closure now reads as 'redirect and return, else 404', and retiredLocation takes the id string.
status: addressed
---
