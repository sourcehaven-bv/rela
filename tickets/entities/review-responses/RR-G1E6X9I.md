---
id: RR-G1E6X9I
type: review-response
title: 'Design: meta-gated relation type fails the whole scope'
finding: 'Refusing the scope for any relation type with meta visible: grants makes relation sync fail for every reader; the flag being policy-wide is not documented.'
severity: minor
resolution: A typed ErrRelationMetaGated names the type so step 5 can refuse or skip that type; RelationMetaGated is documented as policy-wide and independent of the principal.
status: addressed
---
