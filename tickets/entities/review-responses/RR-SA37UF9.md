---
id: RR-SA37UF9
type: review-response
title: count(DISTINCT id) is count(*)
finding: The usage query counts distinct ids of the user type.
severity: nit
reason: entities is keyed by id and face, so one entity can have several rows. DISTINCT keeps the count per entity.
status: wont-fix
---
