---
id: RR-4PX1
type: review-response
title: ctxRecorder.marker uses `any` — should be a concrete string
finding: 'runtime_test.go:2260, 2382: marker stored as `any` and compared `rec.marker != "parent-marker"`. Works via Go''s interface comparison, but barely. Make marker `string` and use the comma-ok type assertion to extract — compiler stops accidental non-string assignments.'
severity: minor
resolution: 'Changed ctxCall.marker to `string` with a separate `hasMarker bool` field. record() uses the comma-ok form: `v, ok := ctx.Value(ctxMarkerKey{}).(string); calls = append(calls, ctxCall{..., marker: v, hasMarker: ok})`. The compiler now stops any accidental non-string value from being recorded, and ''no marker present'' is explicitly distinct from ''empty-string marker''.'
status: addressed
---
