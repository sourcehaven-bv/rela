---
id: RR-Z19ME
type: review-response
title: JSON-pointer escaping for property names — untested ground
finding: 'Property names today are typically simple identifiers but YAML accepts arbitrary string keys. Plan reuses jsonPointerEscape (good) but no tests for property-name edge cases on entity side. relations_v1_wire_test.go tests for relation paths but not property paths. Property named ''foo/bar'' produces path ''/properties/foo~1bar'' — verify escape happens. Verify metamodel accepts such names (or rejects at load time, in which case moot). Recommendation: table tests for path construction with property names containing /, ~, Unicode, empty string. Half are probably forbidden by metamodel loader — if so document and skip, if not test. Five min work. From design-review F11.'
severity: minor
resolution: 'AC29 added: synthetic test with property name containing /. If metamodel loader rejects such names at load-time, AC documents that finding and is skipped. jsonPointerEscape from internal/dataentry already handles RFC 6901.'
status: addressed
---
