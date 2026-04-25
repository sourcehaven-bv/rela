---
id: RR-ZZU08
type: review-response
title: Unify bad-characters and wrong-id_type error paths
finding: '''wrong id_type'' and ''bad characters'' are two different error messages for conceptually overlapping problems (caller supplied an ID we don''t want).'
severity: nit
reason: 'They''re semantically different: ''bad characters'' is about malformed IDs (any id_type); ''wrong id_type'' is about the caller''s right to supply an ID at all. Merging them would lose diagnostic precision. The two messages target different fixes.'
status: wont-fix
---
