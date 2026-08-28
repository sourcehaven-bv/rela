---
id: RR-JZBP5E
type: review-response
title: Patch.Apply's nil-map + MetaUnset invariant is load-bearing but untested
finding: Patch.Apply (writeapi.go:186-193) allocates a Properties map only when len(p.Properties) > 0, so a MetaUnset-only patch runs delete() against a possibly-nil map. That is a no-op in Go rather than a panic, so the code is CORRECT — but the invariant ('unset-only patches on a nil-property entity are silently fine') is load-bearing, since both MCP and CLI can emit unset-only patches, and it was covered by no test. It held by a Go language property nobody had written down; a future refactor to a non-map property container would break it with no failing test. The guard also reads as if Properties == nil were the hazard being handled, obscuring what the real invariant is.
severity: significant
resolution: Added two tests at different levels. TestPatch_ApplyOnNilMap pins it on the value type directly, so a change of property container fails there rather than only through the manager. TestPatchEntity_NilPropertiesEntity gained an 'unset-only against a nil property map does not panic' subtest exercising the full manager path. The existing set-on-nil-map case was kept as a sibling subtest.
status: addressed
---
