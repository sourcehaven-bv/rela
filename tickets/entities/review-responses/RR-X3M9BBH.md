---
id: RR-X3M9BBH
type: review-response
title: 'Design: clears are deletes nobody authorizes'
finding: 'Clears are found inside the Tx, so they skip the RelationDeleteRequest that ReplaceRelations runs for every named remove. On re-parent the cleared edge belongs to another entity.'
severity: critical
resolution: 'Each distinct (scoped relation, source type) of a clear is authorized as the caller with RelationDeleteRequest before the write. A denied clear refuses the whole write with the ACL error. The clear''s audit record names the cause in its reason.'
status: addressed
---
