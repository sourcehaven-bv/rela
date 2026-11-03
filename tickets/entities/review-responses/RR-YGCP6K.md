---
id: RR-YGCP6K
type: review-response
title: get_entity accepts numbers via CheckString coercion
finding: rela.get_entity(12) silently queried id 12.
severity: minor
resolution: stringArg rejects non-strings; tested.
status: addressed
---
