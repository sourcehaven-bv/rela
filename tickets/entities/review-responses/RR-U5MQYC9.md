---
id: RR-U5MQYC9
type: review-response
title: Literal output values and create required properties not checked
finding: Literals were not validated against the property, and a create could leave a required property unset; both would fail at runtime.
severity: significant
resolution: 'Literals pass metamodel.Validator.ValidatePropertyValue; create refuses an unset required property without a default; a whole enum placeholder must fit the target''s values. Tests: literal wrong type, literal not an option, create leaves required unset.'
status: addressed
---
