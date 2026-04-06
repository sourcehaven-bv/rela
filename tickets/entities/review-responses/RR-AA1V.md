---
id: RR-AA1V
type: review-response
title: YAML union type for dark field needs custom UnmarshalYAML
finding: The dark field can be string 'auto', bool false, or a nested palette object. Go's yaml.v3 cannot handle this natively. Needs a custom UnmarshalYAML method. The codebase has precedent (HeaderCheck in metamodel/types.go:273-293, InverseDef at types.go:229-251) but the plan doesn't acknowledge the implementation complexity of a three-way union type.
severity: minor
resolution: Plan updated to acknowledge custom UnmarshalYAML needed. Will follow HeaderCheck/InverseDef pattern from metamodel/types.go.
status: addressed
---
