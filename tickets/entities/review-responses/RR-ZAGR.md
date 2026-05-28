---
id: RR-ZAGR
type: review-response
title: AnyType in entity RecordType would fail every Eval, locking out edits on drifted data
finding: 'Plan proposed AnyType for unknown metamodel property types. Verified at predicate/eval.go:79-100: runtimeTypeAccepts only short-circuits for RecordType/ListType; AnyType (primitiveType{any}) falls through to name-match equality that rejects every concrete value. Under fail-closed-on-Eval-error, a hand-edited property of off-type (storage is permissive per CLAUDE.md) would deny every grant referencing that field — locking operators out of editing the entity (P0 at 3am pattern).'
severity: critical
resolution: 'Dropped AnyType entirely. Defined value-coercion contract at binding layer (full table in plan): off-type stored values bind as Nil rather than failing Eval. Unsupported metamodel types are NOT declared in env — predicate referencing them fails at compile with a clear operator-facing error. Coercion failures bind Nil, not error: hand-edit data drift becomes a slog.Warn, not a permissions outage.'
status: addressed
---
