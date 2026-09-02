---
id: RR-H1R2R8
type: review-response
title: Stale SchemaEntityDef doc reference in metamodel, and unkeyed schemaJSONWriter literals
finding: Two smaller items. (1) internal/metamodel/schema_output.go:161 said 'returns interface{} to satisfy the SchemaEntityDef interface' — that type no longer exists (it is now the unexported cli.schemaEntityDef). It compiles fine and comment-lint cannot catch it (no brackets), but it is an unqualified pointer to a removed symbol in the very package that satisfies the interface, and this PR is what invalidated it. (2) schemaJSONWriter{out.Out} appeared unkeyed at 6 call sites plus 7 in tests; go vet does not flag a same-package one-field struct, but adding a second field would break all sites at once — silently, if the new field happened to be assignable.
severity: minor
resolution: '(1) Reworded to ''returns any to satisfy the schema-JSON entity view in internal/cli''. (2) All 13 literals converted to the keyed form schemaJSONWriter{out: ...}. Full gates re-run: build, cli/output/metamodel tests, plimsoll, comment-lint, golangci-lint (0 issues), coverage floors PASS.'
status: addressed
---

Minor + nit findings from the TKT-NS3XPE code review (cranky-code-reviewer, PR
#1469).

Reviewer's deferred suggestion, NOT actioned: a `newSchemaJSONWriter(w
io.Writer)` constructor would give one place for a nil-writer guard, instead of
six call sites reaching into the package-global `out`. Left alone because the
coupling is inherited from the old code rather than introduced here, the value
type is immutable, and `out.Out` is set once in runKong — the reviewer
explicitly filed it as "file it, don't fix it".
