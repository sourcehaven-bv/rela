---
id: RR-FVIHO9
type: review-response
title: classifyPromote re-derives create error mapping instead of reusing dataentry's
finding: 'internal/appbuild/realtime_promote.go:126-160 is a second hand-written switch that misses ErrIllegalEntry, customIDNotAllowedError, rejectFileCreate, rejectExternalRefCreate and relation cardinality errors, and will drift. Fix: move create-error classification into one shared function returning status and kind, used by dataentry and the promoter; pin each class''s fs revert behaviour in TestPromote_ErrorClasses.'
severity: significant
resolution: new entitymanager.ClassifyCreateError and AllCreateErrorClasses used by both the dataentry create handler and the promoter (no arch-lint change needed); TestPromoteDenial_EveryClass fails when a class lacks a promote answer and classify_test pins each class (2ce29a3fe 6302a4b22)
status: addressed
---
