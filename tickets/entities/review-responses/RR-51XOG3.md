---
id: RR-51XOG3
type: review-response
title: 'landing: map with a typo''d or empty key is silently discarded'
finding: 'CopyLanding.UnmarshalYAML (internal/metamodel/types.go) decodes `landing: {wrold: actueel}`, `landing: {}` and `landing: {world: ""}` to the zero value; IsZero() is true, validateCopyLanding returns nil, and the copy silently lands on `written`. The operator''s declaration is discarded without a load error, the exact class BUG-I0N3YR fixed for affordance grants.'
severity: critical
resolution: 'CopyLanding.UnmarshalYAML walks the mapping''s keys: an unknown key, an empty mapping, an empty or non-scalar value, and a non-scalar non-mapping node are all load errors naming the accepted forms. TestCopyLanding_UnmarshalRefusesWhatItCannotMean pins nine shapes. The metamodel guide''s copy load-check list names the new refusals.'
status: addressed
---
