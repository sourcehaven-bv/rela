---
id: RR-87P2GGM
type: review-response
title: Relation entries need relation meta gating and edit rights
finding: 'The plan gates relation entries by endpoints only. A field entry on a relation is a meta property that visible: may hide, and taking the remote value writes the local relation.'
severity: significant
resolution: Relation field entries are shown only when RelationMetaRestricted is false for the reader; take remote needs the relation update affordance for that field (RelationFieldVerdicts).
status: addressed
---
