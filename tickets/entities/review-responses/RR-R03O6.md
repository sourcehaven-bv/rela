---
id: RR-R03O6
type: review-response
title: ValidationErrorUnknownProperty does not exist; closed-schema check would be a new behavior, not a softening
finding: 'Plan''s IsSoft switch references ValidationErrorUnknownProperty. Verified: only 5 ValidationErrorType constants exist (Required, InvalidValue, InvalidType, UnknownType, IDPrefix). ValidateProperties iterates schema.PropertyDefs() (declared keys only); never inspects input map keys. Unknown properties are silently accepted today. Plan smuggles a NEW closed-schema check under ''softening'' framing. AC3/AC6 have nothing to soften. Recommendation: drop closed-schema work from this ticket OR rename ticket scope to ''soften AND add closed-schema check'' and add the validator step explicitly. From design-review F1.'
severity: critical
resolution: Closed-schema check dropped from this ticket per recommendation. Plan's Out-of-scope section explicitly documents that ValidationErrorUnknownProperty doesn't exist and that adding the check is its own design (forward-compat tolerance, etc.). All references to unknown_property_key warning code removed. AC3, AC6 reframed around InvalidValue (per RR-3C82L) instead. Filed as follow-up consideration.
status: addressed
---
