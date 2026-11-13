---
id: RR-XPBYH92
type: review-response
title: '[security] Relation _updated leaked when a hidden key was cleared'
finding: '[security] The relation GET served _updated when no present meta key was redacted. A caller whose visible: list hides a key could learn when that key was cleared because the edge then carries no hidden key. It also ignored rel.Inaccessible.'
severity: minor
resolution: relationWhole withholds _updated when the caller can be denied any meta key of the type (affordances.RelationMetaRestricted) or a field is inaccessible; a resolver that redacts but cannot answer fails closed. TestUpdated_RelationWithheldAfterHiddenKeyCleared and TestRelationMetaRestricted pin it.
status: addressed
---
