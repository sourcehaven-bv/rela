---
id: RR-93UN
type: review-response
title: 'Record-return policy gap: runtimeTypeAccepts doesn''t enforce field-level conformance'
finding: 'eval.go runtimeTypeAccepts: a host function declared returning RecordType{''status'': StringType} can return any Record — the engine accepts whatever it returns. evalAttr (lines 93-111) looks up the field by compile-time name without re-checking the runtime type against the declared field type. If a host returns a Record whose ''status'' is a Number, that Number flows into a relational compare and produces a misleading error. Currently unreachable from ACL use cases (no host fn returns a Record yet) but the contract is wrong. Pick one: (a) forbid RecordType as a host-function return type at DeclareFunc time — simplest, matches the use case; (b) have evalAttr re-check looked-up values against declared field types. Option (a) preferred.'
severity: significant
resolution: DeclareFunc now rejects RecordType and ListType as return types with a named error. Forbid-at-declare is the simpler option per the comment in env.go; matches all current use cases (has_role -> bool, has_relation -> bool, count_relations -> number). New TestDeclareFunc_RejectsRecordReturn and TestDeclareFunc_RejectsListReturn pin this.
status: addressed
---
