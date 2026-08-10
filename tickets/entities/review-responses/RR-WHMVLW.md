---
id: RR-WHMVLW
type: review-response
title: Date binder misses time.Time path — silent ACL deny on unquoted dates
finding: 'Phase-2 slice 1 crux. YAML auto-decodes an unquoted date scalar (due: 2026-02-01) to time.Time, a quoted one to string; filter.matchDate handles both (match.go:271-284). affordances'' current coerceScalar (bindings.go:167-183) routes date through default: which returns NewString only for raw.(string) and NewNil() for everything else incl. time.Time (harmless today: date is StringType). After swapping to date->DateType + NewDate binder, if the binder only adds a string->NewDate(parse) case (as the plan''s ''parsed against field layout'' implies), it MISSES the time.Time case, binds Nil, and every date when: predicate evaluated against a real unquoted timestamp flips (fail-closed passes()=deny, resolver.go:643). Silent authorization change on the ACL read gate. FIX: new date binder must handle time.Time (bind directly to NewDate), string (via metamodel.ParseDateValue against propDef, not a hand-rolled layout), else Nil. Golden test with BOTH quoted and unquoted date frontmatter.'
severity: critical
resolution: 'affordances/bindings.go coerceDate now handles both YAML shapes: time.Time (unquoted scalar) binds directly to NewDate; string (quoted) parses via metamodel.ParseDateValue against the field''s PropertyDef (the same parser filter.matchDate uses), else Nil. Pinned by TestResolver_DateWhen_TimeAndString covering time.Time-before/after, string-before/after, and missing (fail-soft deny). Verified: without the time.Time case, unquoted dates bound Nil and denied — the test catches that regression.'
status: addressed
---
