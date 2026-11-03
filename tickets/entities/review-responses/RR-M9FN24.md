---
id: RR-M9FN24
type: review-response
title: RelationFieldVerdicts has the same narrowing
finding: 'Relation meta visibility folds roles with the same dimension type: a role without a relations visible: list for the relation type adds nothing, so an admin also conferred a role with relations[...].visible loses meta fields on that edge. Apply the same rule: a role without such a list contributes every meta key when it reads the from-entity type and face.'
severity: significant
resolution: 'RelationFieldVerdicts applies the union: a role without a meta visible list for the relation shows every meta key when it reads the source type and face. Pinned by TestRoleUnion_RelationMeta. Relation create/remove/meta-write union filed as a follow-up.'
status: addressed
---
