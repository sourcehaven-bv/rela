---
id: RR-JXPWGOF
type: review-response
title: Hidden set and V come from two oracles
finding: FieldVisibleOnEveryRow and SearchFieldScope must agree or a raw value is compared unguarded.
severity: minor
reason: 'Pre-existing: the old propsFilterable relied on FieldVisibleOnEveryRow too. Guarding every property needs a field scope on every reader, which readers without FieldScoper lack. The security review found FieldVisibleOnEveryRow conservative, and TestSearchFieldScope_AgreesWithFieldVerdicts pins V.'
status: deferred
---
