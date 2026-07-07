---
id: RR-O2JF23
type: review-response
title: Labels field needs yaml omitempty for round-trip safety
finding: 'Metamodel YAML write-back (schema/cleanup.go:236, metamodel/rename.go:84) is AST/node-based, so a new struct field is round-trip-safe. But the new `Labels map[string]string` must carry `yaml:"labels,omitempty"` so an empty map never emits `labels: {}` and creates spurious diffs on every rename/cleanup run. Verified no struct-level MarshalYAML on Metamodel/CustomType/PropertyDef exists.'
severity: minor
resolution: Labels field carries yaml:"labels,omitempty" on both CustomType and PropertyDef, and json:"labels,omitempty" on the wire structs. Verified by TestV1SchemaEnumLabelsOmittedWhenAbsent (no labels key emitted for a label-less enum) and live-server check that a non-enum property emits no labels key.
status: addressed
---
