---
id: RR-COVP8
type: review-response
title: NopScriptExecutor unnamed args hurt readability
finding: 'internal/workspace/workspace.go:64, 68 — NopScriptExecutor methods use unnamed positional args: `ExecuteCode(string, lua.WriteDeps, string, *entity.Entity, *entity.Entity) error`. Two *entity.Entity args in a row with no names make it ambiguous which is new vs old. Fix: name them explicitly even if unused (e.g. `_ string, _ lua.WriteDeps, _ string, _, _ *entity.Entity`) or align with the interface''s named args.'
severity: nit
resolution: 'NopScriptExecutor method signatures now use named underscore args: (_ string, _ lua.WriteDeps, _, _ *entity.Entity). Matches the interface''s named parameters when grepping.'
status: addressed
---
