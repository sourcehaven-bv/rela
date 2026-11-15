---
id: RR-FIM4AWO
type: review-response
title: Proposed values skip the metamodel value rules
finding: Parse checked type, enum and integer range only; decimal scale, custom-type patterns and other schema rules were not applied, so a proposal could fail on apply.
severity: significant
resolution: 'Parse runs metamodel.Validator.ValidatePropertyValue on each value (each list item separately). Test: decimal beyond scale.'
status: addressed
---
