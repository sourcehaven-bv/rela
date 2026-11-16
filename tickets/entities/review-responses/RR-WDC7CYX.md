---
id: RR-WDC7CYX
type: review-response
title: get_relations still ignores unknown keys
finding: '{limt = 5} silently returns up to 2000 rows.'
severity: nit
reason: Ignoring unknown keys on get_relations is the documented legacy tolerance kept so existing scripts keep working (the ticket asks for that). A typo still returns a bounded result.
status: wont-fix
---
