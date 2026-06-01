---
id: RR-15UQS
type: review-response
title: Small gaps in test coverage
finding: 'Missing: (a) explicit widget:''unknown'' on boolean property falls back to checkbox (the warn->default path); (b) FieldRenderer-level test that checkbox label-click focuses the input (composed id binding); (c) FieldShell + undefined field.property -> ''field-undefined'' id (harmless but unverified); (d) defaultRegistry double-register warning (would catch accidental double import).'
severity: minor
resolution: 'Added 4 tests: (a) registry.test.ts -- explicit widget:''does-not-exist'' on a boolean property falls back to checkbox (not text); (b) registry.test.ts -- double-register on defaultRegistry warns; (c) FieldRenderer.test.ts -- checkbox label-click composition: both label.for and input.id resolve to the same field-XXX id; (d) FieldShell.test.ts -- undefined fieldId produces a label with no for= attribute. 900 tests pass.'
status: addressed
---
