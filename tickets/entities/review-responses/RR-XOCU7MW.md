---
id: RR-XOCU7MW
type: review-response
title: 'Design: deleted scope target, multiple owners, faces'
finding: 'Entity deletes leave scoped edges stale; the owner may not be unique in imported data; faced ids must match by id.'
severity: significant
resolution: 'Entity deletes are not checked (documented, as for relation rules). Membership is judged: the scope target must be among the target''s owners. Ids are compared, not refs with faces.'
status: addressed
---
