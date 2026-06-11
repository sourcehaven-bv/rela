---
id: RR-KT27X
type: review-response
title: cards widget does not fit property-value contract
finding: RelationCards takes entityType/entityId/field/verdict and emits cards-changed with RelationCardState. It renders relations, not property values. Forcing WidgetProps<T> over it produces a leaky abstraction. There is no 'value' here.
severity: critical
resolution: 'Plan revised: cards excluded from this registry. Stays on its existing RelationCards path. A separate RelationWidgetRegistry is a follow-up if/when needed. See TKT-MZSIJ Scope bullet on cards.'
status: addressed
---
