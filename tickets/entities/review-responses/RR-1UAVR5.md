---
id: RR-1UAVR5
type: review-response
title: Scalar flag from the metamodel can diverge from propertyNamesCaller
finding: 'The holder predicate sets Scalar: !list from the property definition. propertyNamesCaller decides by the stored value''s shape (string, []string or []any). After a schema change from a scalar to a list property, or for legacy data, read_session answers waiting_on_me: true while the pushed scalar-only predicate excludes the row, which breaks AC 6 parity. Fix: use Scalar: false; non-scalar PropEqual is equality on a scalar and membership on a list, which is exactly propertyNamesCaller''s rule. If the scalar index matters, keep Scalar and add a mixed-shape row to the parity table test.'
severity: minor
resolution: 'Holder predicate uses Scalar: false (equality on scalar, membership on list), matching propertyNamesCaller. Parity table and AC 6 add a list value on a property now declared scalar.'
status: addressed
---
