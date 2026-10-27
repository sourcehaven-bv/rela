---
id: RR-WL9L8Q
type: review-response
title: latest_by and pick pairing ignores whitespace
finding: 'latest_by: '' '' without pick gives two contradictory errors.'
severity: nit
resolution: The pairing checks trim both values.
status: addressed
---
